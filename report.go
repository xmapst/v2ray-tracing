package main

import (
	"context"
	"encoding/json"
	"io"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/avast/retry-go/v4"
	"github.com/sirupsen/logrus"
	logService "github.com/v2fly/v2ray-core/v5/app/log/command"
	statsService "github.com/v2fly/v2ray-core/v5/app/stats/command"
	"github.com/v2fly/v2ray-core/v5/common/net"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type Engine struct {
	ctx         context.Context
	cancel      context.CancelFunc
	v2rayAPI    string
	vectorAddr  string
	inputCh     chan string
	outputCh    chan output
	v2rayConn   *grpc.ClientConn
	statsClient statsService.StatsServiceClient
}

type output struct {
	Type string      `json:"type"`
	Data interface{} `json:"data"`
}

const (
	OutputTypeAccess  = "access"
	OutputTypeConsole = "console"
	OutputTypeStats   = "stats"
	OutputTypeTraffic = "traffic"
)

func (e *Engine) followLogger() {
	for {
		var err = e.ctx.Err()
		if err != nil {
			logrus.Warnf("Context failed %s, interrupt follow", err)
			break
		}
		var logStream logService.LoggerService_FollowLogClient
		_ = retry.Do(
			func() (err error) {
				ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
				defer cancel()
				e.v2rayConn, err = grpc.DialContext(ctx, e.v2rayAPI, grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithBlock())
				if err != nil {
					return err
				}
				r := &logService.FollowLogRequest{}
				logStream, err = logService.NewLoggerServiceClient(e.v2rayConn).FollowLog(context.Background(), r)
				if err != nil {
					return err
				}
				e.statsClient = statsService.NewStatsServiceClient(e.v2rayConn)
				return
			},
			retry.Attempts(0),
			retry.DelayType(func(n uint, err error, config *retry.Config) time.Duration {
				max := time.Duration(n)
				if max > 8 {
					max = 8
				}
				duration := time.Second * max * max
				logrus.Errorf("dial %s failed %d times: %v, wait %s", e.v2rayAPI, n, err, duration.String())
				return duration
			}),
			retry.MaxDelay(time.Second*64),
		)
		logrus.Infof("connected to %s", e.v2rayAPI)
		var stats_ctx, stats_cancel = context.WithCancel(context.Background())
		go e.stats(stats_ctx)
	Out:
		for {
			select {
			case <-e.ctx.Done():
				break Out
			default:
				resp, err := logStream.Recv()
				if err != nil {
					if err == io.EOF {
						logrus.Info("logger service closed")
					} else {
						logrus.Errorf("logger service error: %v", err)
					}
					break Out
				}
				msg := resp.GetMessage()
				if msg != "" {
					e.inputCh <- msg
				}
			}
		}
		stats_cancel()
		_ = e.v2rayConn.Close()
	}
}

var (
	accessRegexp  = regexp.MustCompile(`^(\w+):(\d+\.\d+\.\d+\.\d+):(\d+) accepted (\w+):(.*):(\d+) \[(.*)]$`)
	accessRegexp1 = regexp.MustCompile(`^(\d+\.\d+\.\d+\.\d+):(\d+) accepted (\w+):(.*):(\d+) \[(.*)]$`)
	consoleRegexp = regexp.MustCompile(`^\[(\w+)] \[(\d+)] ([\w/]+): (.*)$`)
)

type Access struct {
	Type        string `json:"type"`
	Src         string `json:"src"`
	SrcPort     int64  `json:"src_port"`
	SrcProtocol string `json:"src_protocol"`
	Dst         string `json:"dst"`
	DstPort     int64  `json:"dst_port"`
	DstProtocol string `json:"dst_protocol"`
	Outbound    string `json:"outbound"`
}

type Console struct {
	Type      string `json:"type"`
	Level     string `json:"level"`
	SessionID string `json:"session_id"`
	Tag       string `json:"tag"`
	Message   string `json:"message"`
}

func (e *Engine) processLogger() {
	for {
		line, ok := <-e.inputCh
		if !ok {
			break
		}
		if strings.HasPrefix(line, "[") {
			match := consoleRegexp.FindStringSubmatch(line)
			if match != nil {
				e.outputCh <- output{
					Type: OutputTypeConsole,
					Data: Console{
						Type:      OutputTypeConsole,
						Level:     strings.ToLower(match[1]),
						SessionID: match[2],
						Tag:       match[3],
						Message:   match[4],
					},
				}
			}
		} else {
			match := accessRegexp.FindStringSubmatch(line)
			if match == nil {
				// TProxy access log
				match = accessRegexp1.FindStringSubmatch(line)
				if match == nil {
					continue
				}
				srcPort, err := strconv.ParseInt(match[2], 10, 64)
				if err != nil {
					logrus.Warnf("Failed to parse src port: %s", err)
				}
				dstPort, err := strconv.ParseInt(match[5], 10, 64)
				if err != nil {
					logrus.Warnf("Failed to parse dst port: %s", err)
				}
				e.outputCh <- output{
					Type: OutputTypeAccess,
					Data: Access{
						Type:        OutputTypeAccess,
						SrcProtocol: "tcp",
						Src:         match[1],
						SrcPort:     srcPort,
						DstProtocol: match[3],
						Dst:         match[4],
						DstPort:     dstPort,
						Outbound:    match[6],
					},
				}
				continue
			}
			srcPort, err := strconv.ParseInt(match[3], 10, 64)
			if err != nil {
				logrus.Warnf("Failed to parse src port: %s", err)
			}
			dstPort, err := strconv.ParseInt(match[6], 10, 64)
			if err != nil {
				logrus.Warnf("Failed to parse dst port: %s", err)
			}
			e.outputCh <- output{
				Type: OutputTypeAccess,
				Data: Access{
					Type:        OutputTypeAccess,
					SrcProtocol: match[1],
					Src:         match[2],
					SrcPort:     srcPort,
					DstProtocol: match[4],
					Dst:         match[5],
					DstPort:     dstPort,
					Outbound:    match[7],
				},
			}
		}
	}
}

func (e *Engine) Run() {
	go e.processLogger()
	dialer := net.Dialer{}
	for {
		var conn net.Conn
		_ = retry.Do(
			func() (err error) {
				ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
				defer cancel()
				conn, err = dialer.DialContext(ctx, "tcp", e.vectorAddr)
				return
			},
			retry.Attempts(0),
			retry.DelayType(func(n uint, err error, config *retry.Config) time.Duration {
				max := time.Duration(n)
				if max > 8 {
					max = 8
				}
				duration := time.Second * max * max
				logrus.Errorf("dial %s failed %d times: %v, wait %s", e.vectorAddr, n, err, duration.String())
				return duration
			}),
			retry.MaxDelay(time.Second*64),
		)
		logrus.Infof("dial %s success, send data to vector", e.vectorAddr)
		e.ctx, e.cancel = context.WithCancel(context.Background())
		go e.followLogger()
	Out:
		for {
			var buf []byte
			var err error
			select {
			case out := <-e.outputCh:
				buf, err = json.Marshal(out.Data)
				if err != nil {
					logrus.Error(err)
					break Out
				}
			}
			if _, err = conn.Write(buf); err != nil {
				logrus.Error(err)
				break Out
			}
			if _, err = conn.Write([]byte{'\n'}); err != nil {
				break Out
			}
		}
		e.cancel()
		_ = conn.Close()
	}
}

func (e *Engine) stats(ctx context.Context) {
	logrus.Info("start retrieve stats")
	timeTick := time.Tick(time.Second)
	for range timeTick {
		var err = ctx.Err()
		if err != nil {
			logrus.Warnf("Stats Context failed %s, interrupt stats", err)
			break
		}
		e.getRuntimeStats()
		e.getStats()
	}
}

type Stats struct {
	Type         string `json:"type"`
	Uptime       uint32 `json:"up_time"`
	Sys          uint64 `json:"sys"`
	NumGoroutine uint32 `json:"num_goroutine"`
	Alloc        uint64 `json:"alloc"`
	LiveObjects  uint64 `json:"live_objects"`
	TotalAlloc   uint64 `json:"total_alloc"`
	Mallocs      uint64 `json:"mallocs"`
	Frees        uint64 `json:"frees"`
	NumGC        uint32 `json:"num_gc"`
	PauseTotalNs uint64 `json:"pause_total_ns"`
}

func (e *Engine) getRuntimeStats() {
	r := &statsService.SysStatsRequest{}
	rsp, err := e.statsClient.GetSysStats(e.ctx, r)
	if err != nil {
		logrus.Error("Failed to get system stats: ", err)
		return
	}
	e.outputCh <- output{
		Type: OutputTypeStats,
		Data: Stats{
			Type:         OutputTypeStats,
			Uptime:       rsp.Uptime,
			Sys:          rsp.Sys,
			NumGoroutine: rsp.NumGoroutine,
			Alloc:        rsp.Alloc,
			LiveObjects:  rsp.LiveObjects,
			TotalAlloc:   rsp.TotalAlloc,
			Mallocs:      rsp.Mallocs,
			Frees:        rsp.Frees,
			NumGC:        rsp.NumGC,
			PauseTotalNs: rsp.PauseTotalNs,
		},
	}
}

type Traffic struct {
	Type  string `json:"type"`
	Bound string `json:"bound"`
	Name  string `json:"name"`
	Tag   string `json:"tag"`
	Link  string `json:"link"`
	Value int64  `json:"value"`
}

func (e *Engine) getStats() {
	r := &statsService.QueryStatsRequest{
		Reset_: true,
	}
	resp, err := e.statsClient.QueryStats(e.ctx, r)
	if err != nil {
		logrus.Error("Failed to query stats: ", err)
		return
	}
	sort.Slice(resp.Stat, func(i, j int) bool {
		return resp.Stat[i].Name < resp.Stat[j].Name
	})
	for _, stat := range resp.Stat {
		slice := strings.Split(stat.Name, ">>>")
		if len(slice) != 4 {
			continue
		}
		if slice[1] == "api" {
			continue
		}
		e.outputCh <- output{
			Type: OutputTypeTraffic,
			Data: Traffic{
				Type:  OutputTypeTraffic,
				Bound: strings.TrimSuffix(slice[0], "bound"),
				Name:  slice[1],
				Tag:   slice[2],
				Link:  strings.TrimSuffix(slice[3], "link"),
				Value: stat.Value,
			},
		}
	}
}
