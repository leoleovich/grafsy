package grafsy

import (
	"os"
	"strconv"
	"strings"
	"time"
)

// Monitoring structure.
// Based on this self-monitoring will be sent to Graphite.
type Monitoring struct {
	// User config.
	Conf *Config

	// Local config.
	Lc *LocalConfig

	// Structure with amount of metrics from client.
	got source

	// Amount of saved metrics.
	saved int

	// Amount of sent metrics.
	sent int

	// Amount of dropped metrics.
	dropped int

	// Amount of invalid metrics.
	invalid int

	// Monitoring channel of metrics.
	Ch chan string
}

// The source of metric daemon got.
type source struct {
	// Amount of metrics from network.
	net int

	// Amount of metrics from directory.
	dir int

	// Amount of metrics from retry file.
	retry int
}

// Amount of self-monitoring metrics.
const MonitorMetrics = 7

// Self monitoring of Grafsy.
func (m *Monitoring) generateOwnMonitoring() {

	now := strconv.FormatInt(time.Now().Unix(), 10)
	hostname, _ := os.Hostname()
	path := strings.Replace(m.Conf.MonitoringPath, "HOSTNAME", strings.Replace(hostname, ".", "_", -1), -1) + ".grafsy."

	// If you add a new one - please increase monitorMetrics
	monitor_slice := []string{
		path + "got.net " + strconv.Itoa(m.got.net) + " " + now,
		path + "got.dir " + strconv.Itoa(m.got.dir) + " " + now,
		path + "got.retry " + strconv.Itoa(m.got.retry) + " " + now,
		path + "saved " + strconv.Itoa(m.saved) + " " + now,
		path + "sent " + strconv.Itoa(m.sent) + " " + now,
		path + "dropped " + strconv.Itoa(m.dropped) + " " + now,
		path + "invalid " + strconv.Itoa(m.invalid) + " " + now,
	}

	for _, metric := range monitor_slice {
		select {
		case m.Ch <- metric:
		default:
			m.Lc.Lg.Printf("Too many metrics in the MON queue! This is very bad")
			m.dropped++
		}
	}

}

// Reset values to 0s.
func (m *Monitoring) clean() {
	m.saved = 0
	m.sent = 0
	m.dropped = 0
	m.invalid = 0
	m.got = source{0, 0, 0}
}

// Run monitoring.
// Should be run in separate goroutine.
func (m *Monitoring) RunMonitoring() {
	for ; ; time.Sleep(60 * time.Second) {
		m.generateOwnMonitoring()
		if m.dropped != 0 {
			m.Lc.Lg.Printf("Too many metrics in the main buffer. Had to drop incommings")
		}
		m.clean()
	}
}
