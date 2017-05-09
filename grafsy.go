package main

import (
	"flag"
	"fmt"
	"github.com/BurntSushi/toml"
	"github.com/naegelejd/go-acl"
	"log"
	"net"
	"os"
	"path/filepath"
	"regexp"
	"sync"
	"syscall"
)

// The main config specified by user.
type Config struct {
	// Supervisor manager which is used to run Grafsy. e.g. systemd.
	// Default is none.
	Supervisor string

	// The interval, after which client will send data to graphite. In seconds.
	ClientSendInterval int

	// Maximum amount of metrics which can be processed per second.
	// In case of problems with connection/amount of metrics,
	// this configuration will take save up to maxMetrics*clientSendInterval metrics in.
	MetricsPerSecond int

	// Real Graphite server to which client will send all data
	GraphiteAddr string // TODO: think about multiple servers

	// Timeout for connecting to graphiteAddr.
	// Timeout for writing metrics themselves will be clientSendInterval-connectTimeout-1.
	// Default 7. In seconds
	ConnectTimeout int

	// Local address:port for local daemon.
	LocalBind string

	// Main log file.
	Log string

	// Directory, in which developers/admins... can write any file with metrics.
	MetricDir string

	// Enables ACL for metricDir to let grafsy read files there with any permissions.
	// Default is false.
	UseACL bool

	// Data, which was not sent will be buffered in this file
	RetryFile string

	// Prefix for metric to sum.
	// Do not forget to include it in allowedMetrics if you change it.
	SumPrefix string

	// Prefix for metric to calculate average.
	// Do not forget to include it in allowedMetrics if you change it.
	AvgPrefix string

	// Prefix for metric to find minimal value.
	// Do not forget to include it in allowedMetrics if you change it.
	MinPrefix string

	// Prefix for metric to find maximum value.
	// Do not forget to include it in allowedMetrics if you change it.
	MaxPrefix string

	// Summing up interval for metrics with all prefixes. In seconds.
	AggrInterval int

	// Amount of aggregations which grafsy performs per second.
	// If grafsy receives more metrics than aggrPerSecond*aggrInterval - rest will be dropped
	AggrPerSecond int

	// Full path for metrics, send by grafsy itself.
	// "HOSTNAME" will be replaced with os.Hostname() result from GO.
	MonitoringPath string

	// Regexp of allowed metric.
	// Every metric which is not passing check against regexp will be removed
	AllowedMetrics string
}

// Local config, generated based on Config.
type LocalConfig struct {
	mainBufferSize int
	aggrBufSize    int
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

	monitorMetrics := monitorMetrics
	if conf.MonitoringPath == "" {
		lg.Println("Monitoring is disabled")
		monitorMetrics = 0
	}

	/*
		Units - metric
	*/
	lc := LocalConfig{
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
		conf.MetricsPerSecond * conf.ClientSendInterval * 10}

	if _, err := os.Stat(filepath.Dir(conf.Log)); os.IsNotExist(err) {
		if os.MkdirAll(filepath.Dir(conf.Log), os.ModePerm) != nil {
			log.Println("Can not create logfile's dir ", filepath.Dir(conf.Log))
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
	var ch chan string = make(chan string, lc.mainBufferSize+monitorMetrics)
	var chA chan string = make(chan string, lc.aggrBufSize)
	var chM chan string = make(chan string, monitorMetrics)
	var allowMetricsRegexp = regexp.MustCompile(conf.AllowedMetrics)
	aggrRegexp := regexp.MustCompile(fmt.Sprintf("^(%s|%s|%s|%s)..*", conf.AvgPrefix, conf.SumPrefix, conf.MinPrefix, conf.MaxPrefix))

	mon := Monitoring{
		&conf, Source{},
		0,
		0,
		0,
		0,
		*lg,
		chM}

	cli := Client{
		&conf,
		&lc,
		&mon,
		*graphiteAdrrTCP,
		*lg,
		ch,
		chM}

	srv := Server{
		&conf,
		&lc,
		&mon,
		*lg,
		ch,
		chA,
		allowMetricsRegexp,
		aggrRegexp}

	var wg sync.WaitGroup
	go srv.runServer()
	go cli.runClient()
	if monitorMetrics != 0 {
		go mon.runMonitoring()
	}

	wg.Add(1)
	wg.Wait()
}
