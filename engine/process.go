package engine

import (
	"github.com/sirupsen/logrus"
	"regexp"
	"strconv"
	"strings"
	"time"
)

var (
	accessRegexp  = regexp.MustCompile(`^(\w+):(\d+\.\d+\.\d+\.\d+):(\d+) accepted (\w+):(.*):(\d+) \[(.*)]$`)
	consoleRegexp = regexp.MustCompile(`^\[(\w+)] \[(\d+)] ([\w/]+): (.*)$`)
)

func (e *Engine) processLogger() {
	for {
		line, ok := <-e.inputCh
		if !ok {
			break
		}
		if strings.HasPrefix(line, "[") {
			match := consoleRegexp.FindStringSubmatch(line)
			if match != nil {
				e.outputCh <- Output{
					Type: OutputTypeConsole,
					Data: Console{
						Timestamp: time.Now().Unix(),
						Level:     strings.ToLower(match[1]),
						SessionID: match[2],
						Type:      match[3],
						Message:   match[4],
					},
				}
			}
		} else {
			match := accessRegexp.FindStringSubmatch(line)
			if match != nil {
				srcPort, err := strconv.ParseInt(match[3], 10, 64)
				if err != nil {
					logrus.Warnf("Failed to parse src port: %s", err)
				}
				dstPort, err := strconv.ParseInt(match[6], 10, 64)
				if err != nil {
					logrus.Warnf("Failed to parse dst port: %s", err)
				}
				e.outputCh <- Output{
					Type: OutputTypeAccess,
					Data: Access{
						Timestamp:   time.Now().Unix(),
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
}
