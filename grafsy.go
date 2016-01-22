package main

import (
	"log"
	"os"
	//"net"
	"time"
)

func main() {
	f, _ := os.OpenFile("/var/log/grafsy/grafsy.log", os.O_RDWR | os.O_CREATE | os.O_APPEND, 0660)
	lg := log.New(f, "", log.Ldate|log.Lmicroseconds|log.Lshortfile)

	maxMetric := 10000
	graphiteAddr := "127.0.0.1:2003"
	localBind := "127.0.0.1:3002"

	var c chan string = make(chan string, maxMetric)

	go client(maxMetric, graphiteAddr, c, *lg)
	go server(localBind, c, *lg)

	time.Sleep(5000 * time.Second)
}