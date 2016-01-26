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
	"sort"
)

type Server struct {
	graphiteAddr string
	metricDir string
	sumPrefix string
	lg log.Logger
	ch chan string
	chS chan string
}

// Sum metrics with prefix
func (s Server) sumMetricsWithPrefix() {
	for ;; time.Sleep(1*time.Minute) {
		var results_list[] string
		for i := 0; i < len(s.chS); i++ {
			results_list = append(results_list, strings.Replace(<-s.chS, s.sumPrefix, "", -1))
		}
		sort.Sort(results_list)
		//
	}
}

// Check metric to match base metric regexp
func (s Server)checkMetric(metric string) bool {
	match, _ := regexp.MatchString("^([-a-zA-Z0-9_]+\\.){2}[-a-zA-Z0-9_.]+(\\s)[-0-9.eE+]+(\\s)[0-9]{10}", metric)
	return match
}

// Function checks and removed bad data and sorts it by SUM prefix
func (s Server)cleanAndSortIncomingData(metric string) {
	if s.checkMetric(metric) {
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
	// Run goroutine for reading metrics from metricDir
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