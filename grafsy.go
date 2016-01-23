package main

import (
	"log"
	"os"
	"sync"
	"time"
	"github.com/BurntSushi/toml"
	"fmt"
	"net"
)

func main() {
	type Config struct {
		ClientSendInterval int
		MaxMetric int
		GraphiteAddr string // Think about multiple servers
		LocalBind string
		Log string
		MetricDir string
		RetryFile string
		RetryFileSize int
	}

	var conf Config
	if _, err := toml.DecodeFile("/etc/grafsy/grafsy.toml", &conf); err != nil {
		fmt.Println("Failed to parse config file", err.Error())
	}

	var wg sync.WaitGroup

	f, err := os.OpenFile(conf.Log, os.O_RDWR | os.O_CREATE | os.O_APPEND, 0660)
	if err != nil {
		log.Println("Can not open file "+ conf.Log, err.Error())
		os.Exit(1)
	}
	lg := log.New(f, "", log.Ldate|log.Lmicroseconds|log.Lshortfile)

	var ch chan string = make(chan string, conf.MaxMetric)

	graphiteAdrrTCP, err := net.ResolveTCPAddr("tcp", conf.GraphiteAddr)
	if err != nil {
		lg.Println("This is not a valid address:", err.Error())
		os.Exit(1)
	}

	cli := Client{
		(time.Duration(conf.ClientSendInterval) * time.Second),
		conf.MaxMetric,
		*graphiteAdrrTCP,
		conf.MetricDir,
		conf.RetryFile,
		conf.RetryFileSize,
		*lg,
		ch}
	srv := Server{conf.LocalBind, *lg, ch}

	go srv.runServer()
	go cli.runClient()

	wg.Add(2)
	wg.Wait()
}