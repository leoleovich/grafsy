package main

import (
	"log"
	"os"
	"sync"
	"github.com/BurntSushi/toml"
	"fmt"
	"net"
	"syscall"
	"path/filepath"
	"flag"
)

type Config struct {
	Supervisor string
	ClientSendInterval int
	MetricsPerSecond int
	GraphiteAddr string // Think about multiple servers
	LocalBind string
	Log string
	MetricDir string
	RetryFile string
	SumPrefix string
	SumInterval int
	SumsPerSecond int
	AvgPrefix string
	AvgInterval int
	AvgsPerSecond int
	GrafsyPrefix string
	GrafsySuffix string
	GrafsyHostname string
	AllowedMetrics string
}

type LocalConfig struct {
	monitoringBuffSize int
	mainBufferSize int
	sumBufSize int
	avgBufSize int
	fileMetricSize int
}


func main() {
	var configFile string
	flag.StringVar(&configFile, "c", "/etc/grafsy/grafsy.toml", "Path to config file.")
	flag.Parse()

	var conf Config
	if _, err := toml.DecodeFile(configFile, &conf); err != nil {
		fmt.Println("Failed to parse config file", err.Error())
	}
	f, err := os.OpenFile(conf.Log, os.O_RDWR | os.O_CREATE | os.O_APPEND, 0660)
	if err != nil {
		log.Println("Can not open file "+ conf.Log, err.Error())
		os.Exit(1)
	}
	lg := log.New(f, "", log.Ldate|log.Lmicroseconds|log.Lshortfile)

	monitorMetrics := monitorMetrics
	if conf.GrafsyPrefix == "null" && conf.GrafsySuffix == "null" {
		lg.Println("Monitoring is disabled")
		monitorMetrics = 0
	}

	supervisor := Supervisor{conf.Supervisor}

	/*
		Units - metric
	 */
	lc := LocalConfig{
		/*
			For now we have only 6 metrics (see Monitoring):
				got (net,dir,retry),
				saved,
				sent,
				dropped)
		 */
		monitorMetrics,
		/*
			This is a main buffer
			Every ClientSendInterval you will send upto MetricsPerSecond per second
			It does not make any sense to have it too big cause metrics will be dropped during saving to file
			This buffer is ready to take MetricsPerSecond*ClientSendInterval. Which gives you the rule, than bigger interval you have or
			amount of metric in interval, than more metrics it can take in memory.
		*/
		conf.MetricsPerSecond*conf.ClientSendInterval,
		/*
			This is a sum buffer. I assume it make total sense to have maximum buf = SumsPerSecond*sumInterval.
			For example up to 60*60 sums per second
		*/
		conf.SumsPerSecond*conf.SumInterval,
		/*
			This is a avg buffer. I assume it make total sense to have maximum buf = SumsPerSecond*sumInterval.
			For example up to 60*60 sums per second
		 */
		conf.AvgsPerSecond*conf.AvgInterval,
		/*
			Retry file will take only 10 full buffers
		 */
		conf.MetricsPerSecond*conf.ClientSendInterval*10}


	if _, err := os.Stat(filepath.Dir(conf.Log)); os.IsNotExist(err) {
		if os.MkdirAll(filepath.Dir(conf.Log), os.ModePerm) != nil {
			log.Println("Can not create logfile's dir " + filepath.Dir(conf.Log))
		}
	}

	graphiteAdrrTCP, err := net.ResolveTCPAddr("tcp", conf.GraphiteAddr)
	if err != nil {
		lg.Println("This is not a valid address:", err.Error())
		os.Exit(1)
	}

	/*
		Check if directories for temporary files exist
		This is especially important when your metricDir is in /tmp
	 */
	if _, err := os.Stat(conf.MetricDir); os.IsNotExist(err) {
		oldUmask := syscall.Umask(0)
		os.MkdirAll(conf.MetricDir, 0777|os.ModeSticky)
		syscall.Umask(oldUmask)
	} else {
		os.Chmod(conf.MetricDir, 0777|os.ModeSticky)
	}

	/* Buffers */
	var ch chan string = make(chan string, lc.mainBufferSize + monitorMetrics)
	var chS chan string = make(chan string, lc.sumBufSize)
	var chA chan string = make(chan string, lc.avgBufSize)
	var chM chan string = make(chan string, lc.monitoringBuffSize)

	mon := &Monitoring{
		conf, Source{},
		0,
		0,
		0,
		0,
		*lg,
		chM}

	cli := Client{
		conf,
		lc,
		supervisor,
		mon,
		*graphiteAdrrTCP,
		*lg,
		ch,
		chM}

	srv := Server{
		conf,
		lc,
		mon,
		*lg,
		ch,
		chS,
		chA}

	var wg sync.WaitGroup
	go srv.runServer()
	go cli.runClient()
	if monitorMetrics != 0 {
		go mon.runMonitoring()
	}

	wg.Add(1)
	wg.Wait()
}