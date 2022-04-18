package main

import (
	"fmt"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/xmapst/v2ray-tracing/engine"
	"os"
	"os/signal"
	"path"
	"runtime"
	"strings"
	"syscall"
	"time"
)

func init() {
	registerSignalHandlers()
	// log format init
	logrus.SetReportCaller(true)
	logrus.SetLevel(logrus.DebugLevel)
	logrus.SetFormatter(&logrus.TextFormatter{
		DisableColors:          true,
		TimestampFormat:        time.RFC3339,
		FullTimestamp:          true,
		DisableLevelTruncation: true,
		CallerPrettyfier: func(frame *runtime.Frame) (function string, file string) {
			file = fmt.Sprintf("%s:%d", path.Base(frame.File), frame.Line)
			_f := strings.Split(frame.Function, ".")
			function = _f[len(_f)-1]
			return
		},
	})
}

var server string

func main() {
	cmd := &cobra.Command{
		Use: "v2ray-tracing",
		Run: func(cmd *cobra.Command, args []string) {
			if val, found := os.LookupEnv("SERVER"); found {
				if val != "" {
					server = val
				}
			}

			// start followLogger()
			e := engine.New(server)
			e.Run()
		},
	}
	cmd.PersistentFlags().StringVar(&server, "server", "127.0.0.1:8080", "")
	err := cmd.Execute()
	if err != nil {
		logrus.Errorln(err)
	}
}

func registerSignalHandlers() {
	var sigs = make(chan os.Signal, 1)
	signal.Notify(sigs, os.Interrupt, os.Kill, syscall.SIGTERM, syscall.SIGQUIT)
	go func() {
		<-sigs
		logrus.Println("close!!!")
		os.Exit(0)
	}()
}
