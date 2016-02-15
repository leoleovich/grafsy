package main

import (
	"log"
	"os"
	"net"
	"io/ioutil"
	"time"
	"regexp"
	"strings"
	"strconv"
	"bufio"
)

type Server struct {
	conf Config
	mon *Monitoring
	lg log.Logger
	ch chan string
	chS chan string
}

// Sum metrics with prefix
func (s Server) sumMetricsWithPrefix() {
	for ;; time.Sleep(time.Duration(s.conf.SumInterval)*time.Second) {
		var working_list[] Metric
		chanSize := len(s.chS)
		for i := 0; i < chanSize; i++ {
			found := false
			metric := strings.Replace(<-s.chS, s.conf.SumPrefix, "", -1)
			split := regexp.MustCompile("\\s").Split(metric, 3)

			value, err := strconv.ParseFloat(split[1], 64)
			if err != nil {s.lg.Println("Can not parse value of a metric") ; continue}
			timestamp, err := strconv.ParseInt(split[2], 10, 64)
			if err != nil {s.lg.Println("Can not parse timestamp of a metric") ; continue}

			for i,_ := range working_list {
				if working_list[i].name == split[0] {
					working_list[i].amount++
					working_list[i].value += value
					working_list[i].timestamp += timestamp
					found = true
					break
				}
			}
			if !found {
				working_list = append(working_list, Metric{split[0], 1, value, timestamp})
			}
		}
		for _,val := range working_list {
			s.ch <- val.name + " " +
				strconv.FormatFloat(val.value, 'f', 2, 32) + " " + strconv.FormatInt(val.timestamp/val.amount, 10)
		}
	}
}

// Function checks and removed bad data and sorts it by SUM prefix
func (s Server)cleanAndUseIncomingData(metrics []string) {
	for _,metric := range metrics {
		if validateMetric(metric, s.conf.AllowedMetrics) {
			if strings.HasPrefix(metric, s.conf.SumPrefix) {
				if len(s.chS) < s.conf.MaxMetrics*s.conf.SumInterval{
					s.chS <- metric
				} else {
					s.mon.dropped++
				}
			} else {
				if len(s.ch) < s.conf.MaxMetrics*s.conf.ClientSendInterval {
					s.ch <- metric
				} else {
					s.mon.dropped++
				}
			}
		} else {
			s.mon.dropped++
			s.lg.Println("Removing bad metric \"" + metric + "\" from the list")
		}
	}
	metrics = nil
}

// Handles incoming requests.
func (s Server)handleRequest(conn net.Conn) {
	connbuf := bufio.NewReader(conn)
	var results_list []string
	amount := 0
	for ;; amount++ {
		metric, err := connbuf.ReadString('\n')
		if err!= nil {
			break
		}
		if amount < s.conf.MaxMetrics {
			results_list = append(results_list, strings.Replace(metric, "\n", "", -1))
		} else {
			s.mon.dropped++
		}
	}
	conn.Close()
	s.mon.got.net += amount
	s.cleanAndUseIncomingData(results_list)

	results_list = nil
}

// Reading metrics from files in folder. This is a second way how to send metrics, except direct connection
func (s Server)handleDirMetrics() {
	for ;; time.Sleep(time.Duration(s.conf.ClientSendInterval)*time.Second) {
		files, err := ioutil.ReadDir(s.conf.MetricDir)
		if err != nil {
			panic(err.Error())
		}
		for _, f := range files {
			results_list := readMetricsFromFile(s.conf.MetricDir+"/"+f.Name())
			s.mon.got.dir += len(results_list)
			s.cleanAndUseIncomingData(results_list)
		}

	}
}

func (s Server)runServer() {
	// Listen for incoming connections.
	l, err := net.Listen("tcp", s.conf.LocalBind)
	if err != nil {
		s.lg.Println("Failed to run server:", err.Error())
		os.Exit(1)
	} else {
		s.lg.Println("Server is running")
	}
	// Close the listener when the application closes.
	defer l.Close()

	// Run goroutine for reading metrics from metricDir
	go s.handleDirMetrics()
	// Run goroutine for sum metrics with prefix
	go s.sumMetricsWithPrefix()

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