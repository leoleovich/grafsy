// Package represents complete internal logic of grafsy
package grafsy

import (
	"errors"
	"fmt"
	"github.com/BurntSushi/toml"
	"github.com/naegelejd/go-acl"
	"log"
	"net"
	"os"
	"path/filepath"
	"regexp"
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
	// Default is "HOSTNAME"
	MonitoringPath string

	// Regexp of allowed metric.
	// Every metric which is not passing check against regexp will be removed
	AllowedMetrics string
}

// Local config, generated based on Config.
type LocalConfig struct {
	// Size of main buffer
	MainBufferSize int

	// Size of aggregation buffer
	AggrBufSize int

	// Amount of lines we allow to store in retry-file
	FileMetricSize int

	// Main logger
	Lg *log.Logger

	// Graphite address as a Go type.
	GraphiteAddr net.TCPAddr

	// Aggregation prefix regexp.
	AM *regexp.Regexp

	// Aggregation regexp.
	AggrRegexp *regexp.Regexp

	// Main channel.
	Ch chan string

	// Aggregation channel.
	ChA chan string

	// Monitoring channel.
	ChM chan string
}

func (conf *Config) LoadConfig(configFile string) error {

	if _, err := toml.DecodeFile(configFile, conf); err != nil {
		return errors.New("Failed to parse config file" + err.Error())
	}

	if conf.ClientSendInterval < 1 || conf.AggrInterval < 1 || conf.AggrPerSecond < 1 ||
		conf.MetricsPerSecond < 1 || conf.ConnectTimeout < 1 {
		return errors.New("ClientSendInterval, AggrInterval, AggrPerSecond, ClientSendInterval, " +
			"MetricsPerSecond, ConnectTimeout must be greater than 0")
	}

	if conf.MonitoringPath == "" {
		// This will be replaced later by monitoring routine
		conf.MonitoringPath = "HOSTNAME"
	}

	return nil
}

func (conf *Config) prepareEnvironment() error {
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
			return errors.New("Unable to parse acl: " + err.Error())
			os.Exit(1)
		}
		err = ac.SetFileDefault(conf.MetricDir)
		if err != nil {
			return errors.New("Unable to set acl: " + err.Error())
			os.Exit(1)
		}
	}

	if _, err := os.Stat(filepath.Dir(conf.Log)); os.IsNotExist(err) {
		if os.MkdirAll(filepath.Dir(conf.Log), os.ModePerm) != nil {
			return errors.New("Can not create logfile's dir " + filepath.Dir(conf.Log))
		}
	}

	return nil
}

// Generate LocalConfig with all needed for running server variables
// based on config
func (conf *Config) GenerateLocalConfig() (LocalConfig, error) {

	err := conf.prepareEnvironment()
	if err != nil {
		return LocalConfig{}, errors.New("Can not prepare environment: " + err.Error())
	}

	graphiteAdrrTCP, err := net.ResolveTCPAddr("tcp", conf.GraphiteAddr)
	if err != nil {
		return LocalConfig{}, errors.New("This is not a valid address: " + err.Error())
	}

	/*
		Units - metric
	*/
	/*
		This is a aggr buffer. I assume it make total sense to have maximum buf = PerSecond*Interval.
		For example up to 100*60
	*/
	aggrBuffSize := conf.AggrPerSecond * conf.AggrInterval
	/*
		This is a main buffer
		Every ClientSendInterval you will send upto MetricsPerSecond per second
		It does not make any sense to have it too big cause metrics will be dropped during saving to file
		This buffer is ready to take MetricsPerSecond*ClientSendInterval. Which gives you the rule, than bigger interval you have or
		amount of metric in interval, than more metrics it can take in memory.
	*/
	mainBuffSize := conf.MetricsPerSecond * conf.ClientSendInterval

	f, err := os.OpenFile(conf.Log, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0660)
	if err != nil {
		log.Println("Can not open file", conf.Log, err.Error())
		os.Exit(1)
	}
	lg := log.New(f, "", log.Ldate|log.Lmicroseconds|log.Lshortfile)

	return LocalConfig{

		mainBuffSize,
		aggrBuffSize,
		/*
			Retry file will take only 10 full buffers
		*/
		conf.MetricsPerSecond * conf.ClientSendInterval * 10,
		lg,
		*graphiteAdrrTCP,
		regexp.MustCompile(conf.AllowedMetrics),
		regexp.MustCompile(fmt.Sprintf("^(%s|%s|%s|%s)..*", conf.AvgPrefix, conf.SumPrefix, conf.MinPrefix, conf.MaxPrefix)),
		make(chan string, mainBuffSize+MonitorMetrics),
		make(chan string, aggrBuffSize),
		make(chan string, MonitorMetrics),
	}, nil
}
