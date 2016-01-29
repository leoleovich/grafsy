package main

import (
	"log"
	"os"
	"sync"
	"github.com/BurntSushi/toml"
	"fmt"
	"net"
)

type Config struct {
	ClientSendInterval int
	MaxMetrics int
	GraphiteAddr string // Think about multiple servers
	LocalBind string
	Log string
	MetricDir string
	RetryFile string
	RetryFileMaxSize int64
	SumPrefix string
	SumInterval int
}

const metricsSize = 50
func main() {
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

	/*
		This is a main buffer
		It does not make any sense to have it too big cause metrics will be dropped during saving to file
		I assume, that avg size of metric is 50 Byte. This make us calculate buf = RetryFileMaxSize/50
	 */
	var ch chan string = make(chan string, conf.RetryFileMaxSize/metricsSize)
	/*
		This is a sum buffer. I assume it make total sense to have maximum buf = maxMetric*sumInterval.
		For example up to 10000 sums per second
	*/
	var chS chan string = make(chan string, conf.MaxMetrics*conf.SumInterval)

	graphiteAdrrTCP, err := net.ResolveTCPAddr("tcp", conf.GraphiteAddr)
	if err != nil {
		lg.Println("This is not a valid address:", err.Error())
		os.Exit(1)
	}

	cli := Client{
		conf,
		*graphiteAdrrTCP,
		*lg,
		ch}
	srv := Server{
		conf,
		*lg,
		ch,
		chS}

	go srv.runServer()
	go cli.runClient()

	wg.Add(2)
	wg.Wait()
}