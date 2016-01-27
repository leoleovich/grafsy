package main

import (
	"log"
	"os"
	"net"
	"bytes"
	"io/ioutil"
	"time"
	"regexp"
	"strings"
	"strconv"
)

type Server struct {
	graphiteAddr string
	metricDir string
	sumPrefix string
	SumInterval int
	lg log.Logger
	ch chan string
	chS chan string
}

// Sum metrics with prefix
func (s Server) sumMetricsWithPrefix() []string {
	for ;; time.Sleep(time.Duration(s.SumInterval)*time.Second) {
		var working_list[] Metric
		chanSize := len(s.chS)
		for i := 0; i < chanSize; i++ {
			found := false
			metric := strings.Replace(<-s.chS, "SUM_", "", -1)
			split := regexp.MustCompile("\\s").Split(metric, 3)

			value, err := strconv.ParseFloat(split[1], 64)
			if err != nil {continue}
			timestamp, err := strconv.ParseInt(split[2], 10, 64)
			if err != nil {continue}

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
func (s Server)cleanAndSortIncomingData(metric string) {
	if validateMetric(metric) {
		if strings.HasPrefix(metric, s.sumPrefix) {
			s.chS <- metric
		} else {
			s.ch <- metric
		}
	}else {
		s.lg.Println("Removing bad metric \"" + metric + "\" from the list")
	}
}

// Handles incoming requests.
func (s Server)handleRequest(conn net.Conn) {
	// Make a buffer to hold incoming data.
	buf := make([]byte, 1024)
	_, err := conn.Read(buf)
	if err != nil {
		s.lg.Println("Error reading:", err.Error())
	}
	conn.Close()
	n := bytes.Index(buf, []byte{0})
	s.cleanAndSortIncomingData(string(buf[:n-1]))
}

// Reading metrics from files in folder. This is a second way how to send metrics, except direct connection
func (s Server)handleDirMetrics() []string {
	for ;; time.Sleep(1*time.Second) {
		var results_list []string
		files, err := ioutil.ReadDir(s.metricDir)
		if err != nil {
			panic(err.Error())
			return results_list
		}
		for _, f := range files {
			for _,metr := range readMetricsFromFile(s.metricDir+"/"+f.Name()) {
				s.cleanAndSortIncomingData(metr)
			}
		}

	}
}


func (s Server)runServer() {
	// Listen for incoming connections.
	l, err := net.Listen("tcp", s.graphiteAddr)
	if err != nil {
		s.lg.Println("Failed to Run server:", err.Error())
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