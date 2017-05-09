// Package represents complete internal logic of grafsy
package grafsy

import (
	"log"
	"net"
	"regexp"
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
