package main

import (
	"log"
	"os"
	"net"
	"bytes"
	//"io"
)

type Server struct {
	graphiteAdrr string
	lg log.Logger
	ch chan string
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