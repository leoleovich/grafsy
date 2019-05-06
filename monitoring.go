package grafsy

import (
	"fmt"
	"strconv"
	"strings"
	"sync"
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

	// Statistic per carbon receiver
	stat map[string]*stat

	// Amount of invalid metrics.
	invalid int
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

// The statistic of metrics per backend
type stat struct {
	// Amount of saved metrics.
	saved int

	// Amount of sent metrics.
	sent int

	// Amount of dropped metrics.
	dropped int
}

var statLock sync.Mutex

// Self monitoring of Grafsy.
func (m *Monitoring) generateOwnMonitoring() {

	now := strconv.FormatInt(time.Now().Unix(), 10)
	path := m.Conf.MonitoringPath + ".grafsy"
	statLock.Lock()

	monitorSlice := []string{
		fmt.Sprintf("%s.got.net %v %v", path, m.got.net, now),
		fmt.Sprintf("%s.got.dir %v %v", path, m.got.dir, now),
		fmt.Sprintf("%s.got.retry %v %v", path, m.got.retry, now),
		fmt.Sprintf("%s.invalid %v %v", path, m.invalid, now),
	}

	for _, carbonAddrTCP := range m.Lc.carbonAddrsTCP {
		backend := carbonAddrTCP.String()
		backendString := strings.Replace(backend, ".", "_", -1)
		monitorSlice = append(monitorSlice, fmt.Sprintf("%s.%s.saved %v %v", path, backendString, m.stat[backend].saved, now))
		monitorSlice = append(monitorSlice, fmt.Sprintf("%s.%s.sent %v %v", path, backendString, m.stat[backend].sent, now))
		monitorSlice = append(monitorSlice, fmt.Sprintf("%s.%s.dropped %v %v", path, backendString, m.stat[backend].dropped, now))
	}

	statLock.Unlock()

	for _, metric := range monitorSlice {
		select {
		case m.Lc.monitoringChannel <- metric:
		default:
			m.Lc.lg.Printf("Too many metrics in the MON queue! This is very bad")
			for _, carbonAddrTCP := range m.Lc.carbonAddrsTCP {
				backend := carbonAddrTCP.String()
				m.Increase(&m.stat[backend].dropped, 1)
			}
		}
	}
}

// Reset values to 0s.
func (m *Monitoring) clean() {
	for _, carbonAddrTCP := range m.Lc.carbonAddrsTCP {
		backend := carbonAddrTCP.String()
		m.stat[backend].saved = 0
		m.stat[backend].sent = 0
		m.stat[backend].dropped = 0
	}
	m.invalid = 0
	m.got = source{0, 0, 0}
}

// Increase metric value in the thread safe way
func (m *Monitoring) Increase(metric *int, value int) {
	statLock.Lock()
	*metric += value
	statLock.Unlock()
}

// Run monitoring.
// Should be run in separate goroutine.
func (m *Monitoring) Run() {
	statLock.Lock()
	m.stat = make(map[string]*stat)
	for _, carbonAddrTCP := range m.Lc.carbonAddrsTCP {
		backend := carbonAddrTCP.String()
		m.stat[backend] = &stat{
			0,
			0,
			0,
		}
	}
	statLock.Unlock()
	for ; ; time.Sleep(60 * time.Second) {
		m.generateOwnMonitoring()
		statLock.Lock()
		for _, carbonAddrTCP := range m.Lc.carbonAddrsTCP {
			backend := carbonAddrTCP.String()
			if m.stat[backend].dropped != 0 {
				m.Lc.lg.Printf("Too many metrics in the main buffer of %s server. Had to drop incommings: %d", backend, m.stat[backend].dropped)
			}
		}
		m.clean()
		statLock.Unlock()
	}
}
