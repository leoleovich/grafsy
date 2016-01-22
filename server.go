package main

import (
	"log"
	"os"
	"net"
	"bytes"
	//"io"
)

func server(graphiteAdrr string, c chan string, lg log.Logger) {
	// Listen for incoming connections.
	l, err := net.Listen("tcp", graphiteAdrr)
	if err != nil {
		lg.Println("Failed to Run server:", err.Error())
		os.Exit(1)
	} else {
		lg.Println("Server is running:")
	}
	// Close the listener when the application closes.
	defer l.Close()

	for {
		// Listen for an incoming connection.
		conn, err := l.Accept()
		if err != nil {
			lg.Println("Error accepting: ", err.Error())
			os.Exit(1)
		} else {
			lg.Println("New connection!")
		}
		// Handle connections in a new goroutine.
		go handleRequest(conn, c, lg)
	}
}

// Handles incoming requests.
func handleRequest(conn net.Conn, c chan string, lg log.Logger) {
	// Make a buffer to hold incoming data.
	buf := make([]byte, 1024)
	_, err := conn.Read(buf)
	if err != nil {
		lg.Println("Error reading:", err.Error())
	}
	conn.Close()
	n := bytes.Index(buf, []byte{0})
	c <- string(buf[:n-1])
}