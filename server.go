package main

import (
	"bufio"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"
)

type Server struct {
	conf       Config
	lc         LocalConfig
	mon        *Monitoring
	lg         log.Logger
	ch         chan string
	chA        chan string
	aM         *regexp.Regexp
	aggrRegexp *regexp.Regexp
}

// Aggregate metrics with prefix
func (s Server) aggrMetricsWithPrefix() {
	for ; ; time.Sleep(time.Duration(s.conf.AggrInterval) * time.Second) {
		// We assume, that aggregation is done for a current point in time
		aggrTimestamp := time.Now().Unix()

		workingList := make(map[string]*MetricData)
		chanSize := len(s.chA)
		for i := 0; i < chanSize; i++ {
			split := strings.Fields(<-s.chA)
			metricName := split[0]

			value, err := strconv.ParseFloat(split[1], 64)
			if err != nil {
				s.lg.Println("Can not parse value of metric ", metricName, ": ", split[1])
				continue
			}

			_, metricExist := workingList[metricName]
			if !metricExist {
				workingList[metricName] = &MetricData{}
			}

			if strings.HasPrefix(metricName, s.conf.SumPrefix) {
				workingList[metricName].value += value
			} else if strings.HasPrefix(metricName, s.conf.AvgPrefix) {
				workingList[metricName].value += value
				workingList[metricName].amount++
			} else if strings.HasPrefix(metricName, s.conf.MinPrefix) {
				if !metricExist {
					workingList[metricName].value = value
				} else if workingList[metricName].value > value {
					workingList[metricName].value = value
				}
			} else if strings.HasPrefix(metricName, s.conf.MaxPrefix) {
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

			if strings.HasPrefix(metricName, s.conf.SumPrefix) {
				prefix = s.conf.SumPrefix
			} else if strings.HasPrefix(metricName, s.conf.AvgPrefix) {
				value = metricData.value / float64(metricData.amount)
				prefix = s.conf.AvgPrefix
			} else if strings.HasPrefix(metricName, s.conf.MinPrefix) {
				prefix = s.conf.MinPrefix
			} else if strings.HasPrefix(metricName, s.conf.MaxPrefix) {
				prefix = s.conf.MaxPrefix
			}

			select {
			case s.ch <- fmt.Sprintf("%s %.2f %d", strings.Replace(metricName, prefix, "", -1), value, aggrTimestamp):
			default:
				s.lg.Printf("Too many metrics in the main queue (%d). I can not append sum metrics", len(s.ch))
				s.mon.dropped++
			}
		}
	}
}

/*
	Validate metrics list
	Find proper channel for metric
	Check overflow of the channel
	Put metric in a proper channel
*/
func (s Server) cleanAndUseIncomingData(metrics []string) {

	for _, metric := range metrics {
		if s.aM.MatchString(metric) {
			if s.aggrRegexp.MatchString(metric) {
				select {
				case s.chA <- metric:
				default:
					s.mon.dropped++
				}
			} else {
				select {
				case s.ch <- metric:
				default:
					s.mon.dropped++
				}
			}
		} else {
			if metric != "" {
				s.mon.invalid++
				s.lg.Printf("Removing bad metric '%s' from the list", metric)
			}
		}
	}
}

// Reading metrics from network
func (s Server) handleRequest(conn net.Conn) {
	connbuf := bufio.NewReader(conn)
	defer conn.Close()
	for {
		s.mon.got.net++
		metric, err := connbuf.ReadString('\n')
		// Even if error occurred we still put "metric" into analysis, cause it can be a valid metric, but without \n
		s.cleanAndUseIncomingData([]string{strings.Replace(strings.Replace(metric, "\r", "", -1), "\n", "", -1)})

		if err != nil {
			conn.Close()
			break
		}
	}
}

// Reading metrics from files in folder. This is a second way how to send metrics, except network
func (s Server) handleDirMetrics() {
	for ; ; time.Sleep(time.Duration(s.conf.ClientSendInterval) * time.Second) {
		files, err := ioutil.ReadDir(s.conf.MetricDir)
		if err != nil {
			panic(err.Error())
		}
		for _, f := range files {
			results_list := readMetricsFromFile(s.conf.MetricDir + "/" + f.Name())
			s.mon.got.dir += len(results_list)
			s.cleanAndUseIncomingData(results_list)
		}

	}
}

func (s Server) runServer() {
	// Listen for incoming connections.
	l, err := net.Listen("tcp", s.conf.LocalBind)
	if err != nil {
		s.lg.Println("Failed to run server:", err.Error())
		os.Exit(1)
	} else {
		s.lg.Println("Server is running")
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
			s.lg.Println("Error accepting: ", err.Error())
			os.Exit(1)
		}
		// Handle connections in a new goroutine.
		go s.handleRequest(conn)
	}
}
