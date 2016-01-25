package main

import (
	"log"
	"os"
	"net"
	"bytes"
	//"io"
	"io/ioutil"
	"time"
)

type Server struct {
	graphiteAdrr string
	metricDir string
	lg log.Logger
	ch chan string
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
	s.ch <- string(buf[:n-1])
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
				s.ch <- metr
			}
		}

	}
}


func (s Server)runServer() {
	// Listen for incoming connections.
	l, err := net.Listen("tcp", s.graphiteAdrr)
	if err != nil {
		s.lg.Println("Failed to Run server:", err.Error())
		os.Exit(1)
	} else {
		s.lg.Println("Server is running:")
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
		} else {
			s.lg.Println("New connection!")
		}
		// Handle connections in a new goroutine.
		go s.handleRequest(conn)
	}
}