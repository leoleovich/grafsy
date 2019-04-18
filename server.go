package grafsy

import (
	"bufio"
	"fmt"
	"io/ioutil"
	"net"
	"os"
	"strconv"
	"strings"
	"time"
)

// The main server data
type Server struct {
	// User config.
	Conf *Config

	// Local config.
	Lc *localConfig

	// Pointer to Monitoring structure.
	Mon *Monitoring
}

// Aggregate metrics with prefix.
func (s Server) aggrMetricsWithPrefix() {
	for ; ; time.Sleep(time.Duration(s.Conf.AggrInterval) * time.Second) {
		// We assume, that aggregation is done for a current point in time
		aggrTimestamp := time.Now().Unix()

		workingList := make(map[string]*metricData)
		chanSize := len(s.Lc.aggrChannel)
		for i := 0; i < chanSize; i++ {
			split := strings.Fields(<-s.Lc.aggrChannel)
			metricName := split[0]

			value, err := strconv.ParseFloat(split[1], 64)
			if err != nil {
				s.Lc.lg.Println("Can not parse value of metric ", metricName, ": ", split[1])
				continue
			}

			_, metricExist := workingList[metricName]
			if !metricExist {
				workingList[metricName] = &metricData{}
			}

			if strings.HasPrefix(metricName, s.Conf.SumPrefix) {
				workingList[metricName].value += value
			} else if strings.HasPrefix(metricName, s.Conf.AvgPrefix) {
				workingList[metricName].value += value
				workingList[metricName].amount++
			} else if strings.HasPrefix(metricName, s.Conf.MinPrefix) {
				if !metricExist {
					workingList[metricName].value = value
				} else if workingList[metricName].value > value {
					workingList[metricName].value = value
				}
			} else if strings.HasPrefix(metricName, s.Conf.MaxPrefix) {
				if workingList[metricName].value < value {
					workingList[metricName].value = value
				}
			}
		}
		/*
			We may have a problem, that working_list size will be bigger than main buffer/space in it.
			But then go suppose to block appending into buffer and wait until space will be free.
			I am not sure if we need to check free space of main buffer here...
		*/
		for metricName, metricData := range workingList {
			value := metricData.value
			var prefix string

			if strings.HasPrefix(metricName, s.Conf.SumPrefix) {
				prefix = s.Conf.SumPrefix
			} else if strings.HasPrefix(metricName, s.Conf.AvgPrefix) {
				value = metricData.value / float64(metricData.amount)
				prefix = s.Conf.AvgPrefix
			} else if strings.HasPrefix(metricName, s.Conf.MinPrefix) {
				prefix = s.Conf.MinPrefix
			} else if strings.HasPrefix(metricName, s.Conf.MaxPrefix) {
				prefix = s.Conf.MaxPrefix
			}

			select {
			case s.Lc.mainChannel <- fmt.Sprintf("%s %.2f %d", strings.Replace(metricName, prefix, "", -1), value, aggrTimestamp):
			default:
				s.Lc.lg.Printf("Too many metrics in the main queue (%d). I can not append sum metrics", len(s.Lc.mainChannel))
				s.Mon.dropped++
			}
		}
	}
}

func (s *Server) overwriteName(metric *string) {
	for i, re := range s.Lc.overwriteRegexp {
		if re.MatchString(*metric) {
			*metric = re.ReplaceAllString(*metric, s.Conf.Overwrite[i].ReplaceWith)
			return
		}
	}
}

// Validate metrics list in order:
// 1) Find proper channel for metric.
// 2) Check overflow of the channel.
// 3) Put metric in a proper channel.
func (s Server) cleanAndUseIncomingData(metrics []string) {
	for _, metric := range metrics {
		s.overwriteName(&metric)
		if s.Lc.allowedMetrics.MatchString(metric) {
			if s.Lc.aggrRegexp.MatchString(metric) {
				select {
				case s.Lc.aggrChannel <- metric:
				default:
					s.Mon.dropped++
				}
			} else {
				select {
				case s.Lc.mainChannel <- metric:
				default:
					s.Mon.dropped++
				}
			}
		} else {
			if metric != "" {
				s.Mon.invalid++
				s.Lc.lg.Printf("Removing bad metric '%s' from the list", metric)
			}
		}
	}
}

// Reading metrics from network
func (s Server) handleRequest(conn net.Conn) {
	defer conn.Close()
	conBuf := bufio.NewReader(conn)
	for {
		s.Mon.got.net++
		metric, err := conBuf.ReadString('\n')
		// Even if error occurred we still put "metric" into analysis, cause it can be a valid metric, but without \n
		s.cleanAndUseIncomingData([]string{strings.Replace(strings.Replace(metric, "\r", "", -1), "\n", "", -1)})
		if err != nil {
			return
		}
	}

	return
}

// Reading metrics from files in folder.
// This is a second way how to send metrics, except network.
func (s Server) handleDirMetrics() {
	for ; ; time.Sleep(time.Duration(s.Conf.ClientSendInterval) * time.Second) {
		files, err := ioutil.ReadDir(s.Conf.MetricDir)
		if err != nil {
			panic(err.Error())
		}
		for _, f := range files {
			results_list, _ := readMetricsFromFile(s.Conf.MetricDir + "/" + f.Name())
			s.Mon.got.dir += len(results_list)
			s.cleanAndUseIncomingData(results_list)
		}

	}
}

// Run server.
// Should be run in separate goroutine.
func (s *Server) Run() {
	// Listen for incoming connections.
	l, err := net.Listen("tcp", s.Conf.LocalBind)
	if err != nil {
		s.Lc.lg.Println("Failed to run server:", err.Error())
		os.Exit(1)
	} else {
		s.Lc.lg.Println("Server is running")
	}
	defer l.Close()

	// Run goroutine for reading metrics from metricDir
	go s.handleDirMetrics()
	// Run goroutine for aggr metrics with prefix
	go s.aggrMetricsWithPrefix()

	for {
		// Listen for an incoming connection.
		conn, err := l.Accept()
		if err != nil {
			s.Lc.lg.Println("Error accepting: ", err.Error())
			os.Exit(1)
		}
		// Handle connections in a new goroutine.
		go s.handleRequest(conn)
	}
}
