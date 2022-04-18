package engine

import (
	"context"
	"fmt"
	"github.com/avast/retry-go/v4"
	"github.com/sirupsen/logrus"
	logService "github.com/v2fly/v2ray-core/v5/app/log/command"
	statsService "github.com/v2fly/v2ray-core/v5/app/stats/command"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"io"
	"time"
)

type Engine struct {
	ctx         context.Context
	cancel      context.CancelFunc
	address     string
	inputCh     chan string
	outputCh    chan Output
	conn        *grpc.ClientConn
	statsClient statsService.StatsServiceClient
}

func New(address string) *Engine {
	return &Engine{
		address:  address,
		inputCh:  make(chan string, 1024),
		outputCh: make(chan Output, 1024),
	}
}

func (e *Engine) Run() {
	go e.processLogger()
	go e.output()
	for {
		var stream logService.LoggerService_FollowLogClient
		_ = retry.Do(
			func() (err error) {
				ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
				defer cancel()
				e.conn, err = grpc.DialContext(ctx, e.address, grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithBlock())
				if err != nil {
					return err
				}
				r := &logService.FollowLogRequest{}
				stream, err = logService.NewLoggerServiceClient(e.conn).FollowLog(context.Background(), r)
				if err != nil {
					return err
				}
				e.statsClient = statsService.NewStatsServiceClient(e.conn)
				return
			},
			retry.Attempts(0),
			retry.DelayType(func(n uint, err error, config *retry.Config) time.Duration {
				max := time.Duration(n)
				if max > 8 {
					max = 8
				}
				duration := time.Second * max * max
				fmt.Printf("dial %s failed %d times: %v, wait %s\n", e.address, n, err, duration.String())
				return duration
			}),
			retry.MaxDelay(time.Second*64),
		)
		logrus.Infof("dial %s success", e.address)
		e.ctx, e.cancel = context.WithCancel(context.Background())
		go e.stats()

		for {
			resp, err := stream.Recv()
			if err != nil {
				if err == io.EOF {
					logrus.Info("logger service closed")
				} else {
					logrus.Errorf("logger service error: %v", err)
				}
				break
			}
			msg := resp.GetMessage()
			if msg != "" {
				e.inputCh <- msg
			}
		}
		e.cancel()
		e.conn.Close()
	}
}
