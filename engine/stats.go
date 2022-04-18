package engine

import (
	"github.com/sirupsen/logrus"
	statsService "github.com/v2fly/v2ray-core/v5/app/stats/command"
	"sort"
	"strings"
	"time"
)

func (e *Engine) stats() {
	timeTick := time.Tick(time.Second)
	for range timeTick {
		e.getRuntimeStats()
		e.getStats()
	}
}

func (e *Engine) getRuntimeStats() {
	r := &statsService.SysStatsRequest{}
	rsp, err := e.statsClient.GetSysStats(e.ctx, r)
	if err != nil {
		logrus.Error("Failed to get system stats: ", err)
		return
	}
	e.outputCh <- Output{
		Type: OutputTypeStats,
		Data: Stats{
			Timestamp:    time.Now().Unix(),
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
		if stat.Value == 0 {
			continue
		}
		slice := strings.Split(stat.Name, ">>>")
		if len(slice) != 4 {
			continue
		}
		if slice[1] == "api" {
			continue
		}
		e.outputCh <- Output{
			Type: OutputTypeTraffic,
			Data: Traffic{
				Timestamp: time.Now().Unix(),
				Bound:     strings.TrimSuffix(slice[0], "bound"),
				Name:      slice[1],
				//Tag:       slice[2],
				Link:  strings.TrimSuffix(slice[3], "link"),
				Value: stat.Value,
			},
		}
	}
}
