package main

import (
	"flag"
	"fmt"
	"github.com/BurntSushi/toml"
	grafsy "github.com/leoleovich/grafsy/grafsy"
	"github.com/naegelejd/go-acl"
	"log"
	"net"
	"os"
	"path/filepath"
	"regexp"
	"sync"
	"syscall"
)

func main() {
	var configFile string
	flag.StringVar(&configFile, "c", "/etc/grafsy/grafsy.toml", "Path to config file.")
	flag.Parse()

	var conf grafsy.Config
	if _, err := toml.DecodeFile(configFile, &conf); err != nil {
		fmt.Println("Failed to parse config file", err.Error())
	}
	f, err := os.OpenFile(conf.Log, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0660)
	if err != nil {
		log.Println("Can not open file", conf.Log, err.Error())
		os.Exit(1)
	}
	lg := log.New(f, "", log.Ldate|log.Lmicroseconds|log.Lshortfile)

	if conf.ClientSendInterval < 1 || conf.AggrInterval < 1 || conf.AggrPerSecond < 1 ||
		conf.MetricsPerSecond < 1 || conf.ConnectTimeout < 1 {
		lg.Println("ClientSendInterval, AggrInterval, AggrPerSecond, ClientSendInterval, " +
			"MetricsPerSecond, ConnectTimeout must be greater than 0")
		os.Exit(1)
	}

	monitorMetrics := grafsy.MonitorMetrics
	if conf.MonitoringPath == "" {
		lg.Println("Monitoring is disabled")
		monitorMetrics = 0
	}

	graphiteAdrrTCP, err := net.ResolveTCPAddr("tcp", conf.GraphiteAddr)
	if err != nil {
		lg.Println("This is not a valid address:", err.Error())
		os.Exit(1)
	}

	/*
		Units - metric
	*/
	lc := grafsy.LocalConfig{
		/*
			This is a main buffer
			Every ClientSendInterval you will send upto MetricsPerSecond per second
			It does not make any sense to have it too big cause metrics will be dropped during saving to file
			This buffer is ready to take MetricsPerSecond*ClientSendInterval. Which gives you the rule, than bigger interval you have or
			amount of metric in interval, than more metrics it can take in memory.
		*/
		conf.MetricsPerSecond * conf.ClientSendInterval,
		/*
			This is a aggr buffer. I assume it make total sense to have maximum buf = PerSecond*Interval.
			For example up to 100*60
		*/
		conf.AggrPerSecond * conf.AggrInterval,
		/*
			Retry file will take only 10 full buffers
		*/
		conf.MetricsPerSecond * conf.ClientSendInterval * 10,
		lg,
		*graphiteAdrrTCP,
		regexp.MustCompile(conf.AllowedMetrics),
		regexp.MustCompile(fmt.Sprintf("^(%s|%s|%s|%s)..*", conf.AvgPrefix, conf.SumPrefix, conf.MinPrefix, conf.MaxPrefix)),
	}

	if _, err := os.Stat(filepath.Dir(conf.Log)); os.IsNotExist(err) {
		if os.MkdirAll(filepath.Dir(conf.Log), os.ModePerm) != nil {
			log.Println("Can not create logfile's dir ", filepath.Dir(conf.Log))
		}
	}

	/*
		Check if directories for temporary files exist
		This is especially important when your metricDir is in /tmp
	*/
	oldUmask := syscall.Umask(0)

	if _, err := os.Stat(conf.MetricDir); os.IsNotExist(err) {
		os.MkdirAll(conf.MetricDir, 0777|os.ModeSticky)
	} else {
		os.Chmod(conf.MetricDir, 0777|os.ModeSticky)
	}
	syscall.Umask(oldUmask)

	/*
		Unfortunately some people write to MetricDir with random permissions
		To avoid server crashing and overflowing we need to set ACL on MetricDir, that grafsy is allowed
		to read/delete files in there
	*/
	if conf.UseACL {
		ac, err := acl.Parse("user::rw group::rw mask::r other::r")
		if err != nil {
			lg.Println("Unable to parse acl:", err.Error())
			os.Exit(1)
		}
		err = ac.SetFileDefault(conf.MetricDir)
		if err != nil {
			lg.Println("Unable to set acl:", err.Error())
			os.Exit(1)
		}
	}

	/* Buffers */
	var ch chan string = make(chan string, lc.MainBufferSize+monitorMetrics)
	var chA chan string = make(chan string, lc.AggrBufSize)
	var chM chan string = make(chan string, monitorMetrics)

	mon := grafsy.Monitoring{
		Conf: &conf,
		Lc:   &lc,
		Ch:   chM,
	}

	cli := grafsy.Client{
		&conf,
		&lc,
		&mon,
		ch,
		chM,
	}

	srv := grafsy.Server{
		&conf,
		&lc,
		&mon,
		ch,
		chA,
	}

	var wg sync.WaitGroup
	go srv.RunServer()
	go cli.RunClient()
	if monitorMetrics != 0 {
		go mon.RunMonitoring()
	}

	wg.Add(1)
	wg.Wait()
}
