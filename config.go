package grafsy

import (
	"fmt"
	"log"
	"net"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/pelletier/go-toml/v2"
)

// ConfigPath is the default path to the configuration file
var ConfigPath = "/etc/grafsy/grafsy.toml"

// Config is the main config specified by user.
type Config struct {
	// Supervisor manager which is used to run Grafsy. e.g. systemd.
	// Default is none.
	Supervisor string

	// The interval, after which client will send data to graphite. In seconds.
	ClientSendInterval int

	// Maximum amount of metrics which can be processed per second.
	// In case of problems with connection/amount of metrics,
	// this configuration will save up to MetricsPerSecond * ClientSendInterval * 10 metrics in retryDir.
	MetricsPerSecond int

	// Real Carbon servers to which client will send all data
	CarbonAddrs []string

	// Timeout for connecting to graphiteAddr.
	// Timeout for writing metrics themselves will be clientSendInterval-connectTimeout-1.
	// Default 7. In seconds.
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

	// Data, which was not sent will be buffered in this directory.
	RetryDir string

	// Time in seconds to keep metrics in retry file, at least
	RetryKeepSecs int

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
	// If grafsy receives more metrics than aggrPerSecond*aggrInterval - rest will be dropped.
	AggrPerSecond int

	// Alias to use instead of os.Hostname() result
	Hostname string

	// Full path for metrics, send by grafsy itself.
	// "HOSTNAME" will be replaced with os.Hostname() result from GO.
	// Default is "HOSTNAME"
	MonitoringPath string

	// Regexp of allowed metric.
	// Every metric which is not passing check against regexp will be removed.
	AllowedMetrics string

	// List of metrics to overwrite
	Overwrite []struct {
		// Regexp of metric to replace from config
		ReplaceWhatRegexp string

		// New metric part
		ReplaceWith string
	}
}

// LocalConfig is generated based on Config.
type LocalConfig struct {
	// Hostname of server
	hostname string

	// Size of main buffer.
	mainBufferSize int

	// Size of aggregation buffer.
	aggrBufSize int

	// Amount of lines we allow to store in retry-file.
	fileMetricSize int

	// Main logger.
	lg *log.Logger

	// Aggregation prefix regexp.
	allowedMetrics *regexp.Regexp

	// Aggregation regexp.
	aggrRegexp *regexp.Regexp

	// Custom regexps to overwrite metrics via Grafsy.
	overwriteRegexp []*regexp.Regexp

	// Main channel.
	mainChannel chan string

	// Aggregation channel.
	aggrChannel chan string

	// Monitoring channel.
	monitoringChannel chan string
}

// LoadConfig loads a configFile to a Config structure.
func (conf *Config) LoadConfig(configFile string) error {

	// Read the file content
	data, err := os.ReadFile(configFile)
	if err != nil {
		return fmt.Errorf("failed to read config file: %s", err)
	}

	// Unmarshal the data into 'conf'
	if err := toml.Unmarshal(data, conf); err != nil {
		return fmt.Errorf("failed to parse config file: %s", err)
	}

	if conf.ClientSendInterval < 1 || conf.AggrInterval < 1 || conf.AggrPerSecond < 1 ||
		conf.MetricsPerSecond < 1 || conf.ConnectTimeout < 1 {
		return fmt.Errorf("ClientSendInterval, AggrInterval, AggrPerSecond, ClientSendInterval, " +
			"MetricsPerSecond, ConnectTimeout must be greater than 0")
	}

	if conf.RetryKeepSecs <= 0 {
		// Backward compatibility with old behavior
		conf.RetryKeepSecs = conf.ClientSendInterval * 10
	}

	if conf.MonitoringPath == "" {
		// This will be replaced later by monitoring routine
		conf.MonitoringPath = "HOSTNAME"
	}

	return nil
}

// Create necessary directories.
func (conf *Config) prepareEnvironment() error {
	/*
		Check if directories for temporary files exist.
		This is especially important when your metricDir is in /tmp.
	*/

	if _, err := os.Stat(conf.MetricDir); os.IsNotExist(err) {
		// Create directory with default permissions first
		err = os.MkdirAll(conf.MetricDir, os.ModePerm)
		if err != nil {
			return fmt.Errorf("failed to create MetricDir %s: %w", conf.MetricDir, err)
		}
		// Then explicitly set the desired permissions to avoid issues with umask
		err = os.Chmod(conf.MetricDir, 0777|os.ModeSticky)
		if err != nil {
			return fmt.Errorf("failed to chmod MetricDir after creation %s: %w", conf.MetricDir, err)
		}
	} else {
		err = os.Chmod(conf.MetricDir, 0777|os.ModeSticky)
		if err != nil {
			return fmt.Errorf("failed to chmod MetricDir %s: %w", conf.MetricDir, err)
		}
	}

	/*
		Unfortunately some people write to MetricDir with random permissions.
		To avoid server crashing and overflowing we need to set ACL on MetricDir, that grafsy is allowed
		to read/delete files in there.
	*/
	if conf.UseACL {
		err := setACL(conf.MetricDir)
		if err != nil {
			return fmt.Errorf("can not set ACLs for dir %s: %w", conf.MetricDir, err)
		}
	}

	if _, err := os.Stat(filepath.Dir(conf.Log)); conf.Log != "-" || os.IsNotExist(err) {
		if err = os.MkdirAll(filepath.Dir(conf.Log), os.ModePerm); err != nil {
			return fmt.Errorf("can not create logfile's dir %s: %w", filepath.Dir(conf.Log), err)
		}
	}

	// Check if servers in CarbonAddrs are resolvable
	for _, carbonAddr := range conf.CarbonAddrs {
		_, err := net.ResolveTCPAddr("tcp", carbonAddr)
		if err != nil {
			return fmt.Errorf("could not resolve an address from CarbonAddrs: %w", err)
		}
	}

	return nil
}

func (conf *Config) generateRegexpsForOverwrite() []*regexp.Regexp {
	overwriteMetric := make([]*regexp.Regexp, len(conf.Overwrite))
	for i := range conf.Overwrite {
		overwriteMetric[i] = regexp.MustCompile(conf.Overwrite[i].ReplaceWhatRegexp)
	}
	return overwriteMetric
}

// GenerateLocalConfig generates LocalConfig with all needed for running server variables
// based on Config.
func (conf *Config) GenerateLocalConfig() (*LocalConfig, error) {

	err := conf.prepareEnvironment()
	if err != nil {
		return nil, fmt.Errorf("can not prepare environment: %w", err)
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

	var f *os.File
	if conf.Log == "-" {
		f = os.Stdout
	} else {
		f, err = os.OpenFile(conf.Log, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0660)
		if err != nil {
			log.Println("Can not open file", conf.Log, err)
			os.Exit(1)
		}
	}
	lg := log.New(f, "", log.Ldate|log.Lmicroseconds|log.Lshortfile)

	hostname := conf.Hostname
	if hostname == "" {
		hostname, err = os.Hostname()
		if err != nil {
			return nil, fmt.Errorf("can not resolve the hostname: %w", err)
		}
		hostname = strings.ReplaceAll(hostname, ".", "_")
	}

	// There are 4 metrics per backend in client and 3 in server stats
	MonitorMetrics := 3 + len(conf.CarbonAddrs)*5

	return &LocalConfig{
		hostname:       hostname,
		mainBufferSize: mainBuffSize,
		aggrBufSize:    aggrBuffSize,
		/*
			Retry file will take only 10 full buffers
		*/
		fileMetricSize:    conf.MetricsPerSecond * conf.RetryKeepSecs,
		lg:                lg,
		allowedMetrics:    regexp.MustCompile(conf.AllowedMetrics),
		aggrRegexp:        regexp.MustCompile(fmt.Sprintf("^(%s|%s|%s|%s)..*", conf.AvgPrefix, conf.SumPrefix, conf.MinPrefix, conf.MaxPrefix)),
		overwriteRegexp:   conf.generateRegexpsForOverwrite(),
		mainChannel:       make(chan string, mainBuffSize+MonitorMetrics),
		aggrChannel:       make(chan string, aggrBuffSize),
		monitoringChannel: make(chan string, MonitorMetrics),
	}, nil
}
