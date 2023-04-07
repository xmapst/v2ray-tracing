package main

import (
	"fmt"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"os"
	"os/signal"
	"path"
	"strings"
	"syscall"
)

func init() {
	registerSignalHandlers()
	// log format init
	logrus.SetReportCaller(true)
	logrus.SetLevel(logrus.DebugLevel)
	logrus.SetFormatter(&ConsoleFormatter{})
}

type ConsoleFormatter struct {
	logrus.TextFormatter
}

func (c *ConsoleFormatter) TrimFunctionSuffix(s string) string {
	if strings.Contains(s, ".func") {
		index := strings.Index(s, ".func")
		s = s[:index]
	}
	return s
}

func (c *ConsoleFormatter) Format(entry *logrus.Entry) ([]byte, error) {
	file := path.Base(entry.Caller.File)
	function := c.TrimFunctionSuffix(path.Base(entry.Caller.Function))
	logStr := fmt.Sprintf("%s %s %s:%d %s %v\n",
		entry.Time.Format("2006/01/02 15:04:05"),
		strings.ToUpper(entry.Level.String()),
		file,
		entry.Caller.Line,
		function,
		entry.Message,
	)
	return []byte(logStr), nil
}

var v2rayAPI string
var vectorAddr string

func main() {
	cmd := &cobra.Command{
		Use: "v2ray-tracing",
		Run: func(cmd *cobra.Command, args []string) {
			if val, found := os.LookupEnv("V2RAY_API"); found {
				if val != "" {
					v2rayAPI = val
				}
			}
			if val, found := os.LookupEnv("VECTOR_ADDR"); found {
				if val != "" {
					vectorAddr = val
				}
			}

			// start followLogger()
			e := &Engine{
				v2rayAPI:   v2rayAPI,
				vectorAddr: vectorAddr,
				inputCh:    make(chan string, 1024),
				outputCh:   make(chan output, 1024),
			}
			e.Run()
		},
	}
	cmd.PersistentFlags().StringVar(&v2rayAPI, "v2ray_api", "127.0.0.1:8080", "")
	cmd.PersistentFlags().StringVar(&vectorAddr, "vector_addr", "127.0.0.1:9000", "")
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
