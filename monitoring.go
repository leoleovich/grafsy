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
	serverStat serverStat

	// Statistic per carbon receiver
	clientStat map[string]*clientStat
}

// The source of metric daemon got.
type serverStat struct {
	// Amount of metrics from directory.
	dir int

	// Amount of invalid metrics.
	invalid int

	// Amount of metrics from network.
	net int
}

// The statistic of metrics per backend
type clientStat struct {
	// Amount of dropped metrics.
	dropped int

	// Amount of metrics from retry file.
	fromRetry int

	// Amount of saved metrics.
	saved int

	// Amount of sent metrics.
	sent int

	// Amount of aggregated metrics.
	aggregated int
}

var statLock sync.Mutex

// Self monitoring of Grafsy.
func (m *Monitoring) generateOwnMonitoring() {

	now := strconv.FormatInt(time.Now().Unix(), 10)
	path := m.Conf.MonitoringPath + ".grafsy"
	statLock.Lock()

	monitorSlice := []string{
		fmt.Sprintf("%s.got.net %v %v", path, m.serverStat.net, now),
		fmt.Sprintf("%s.got.dir %v %v", path, m.serverStat.dir, now),
		fmt.Sprintf("%s.invalid %v %v", path, m.serverStat.invalid, now),
	}

	for _, carbonAddr := range m.Conf.CarbonAddrs {
		carbonAddrString := strings.ReplaceAll(carbonAddr, ".", "_")
		monitorSlice = append(monitorSlice, fmt.Sprintf("%s.%s.dropped %v %v", path, carbonAddrString, m.clientStat[carbonAddr].dropped, now))
		monitorSlice = append(monitorSlice, fmt.Sprintf("%s.%s.from_retry %v %v", path, carbonAddrString, m.clientStat[carbonAddr].fromRetry, now))
		monitorSlice = append(monitorSlice, fmt.Sprintf("%s.%s.saved %v %v", path, carbonAddrString, m.clientStat[carbonAddr].saved, now))
		monitorSlice = append(monitorSlice, fmt.Sprintf("%s.%s.sent %v %v", path, carbonAddrString, m.clientStat[carbonAddr].sent, now))
		monitorSlice = append(monitorSlice, fmt.Sprintf("%s.%s.aggregated %v %v", path, carbonAddrString, m.clientStat[carbonAddr].aggregated, now))
	}

	statLock.Unlock()

	for _, metric := range monitorSlice {
		select {
		case m.Lc.monitoringChannel <- metric:
		default:
			m.Lc.lg.Printf("Too many metrics in the MON queue! This is very bad")
			for _, carbonAddr := range m.Conf.CarbonAddrs {
				m.Increase(&m.clientStat[carbonAddr].dropped, 1)
			}
		}
	}
}

// Reset values to 0s.
func (m *Monitoring) clean() {
	for _, carbonAddr := range m.Conf.CarbonAddrs {
		m.clientStat[carbonAddr].dropped = 0
		m.clientStat[carbonAddr].fromRetry = 0
		m.clientStat[carbonAddr].saved = 0
		m.clientStat[carbonAddr].sent = 0
		m.clientStat[carbonAddr].aggregated = 0
	}
	m.serverStat = serverStat{0, 0, 0}
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
	m.clientStat = make(map[string]*clientStat)
	for _, carbonAddr := range m.Conf.CarbonAddrs {
		m.clientStat[carbonAddr] = &clientStat{
			0,
			0,
			0,
			0,
			0,
		}
	}
	statLock.Unlock()
	for ; ; time.Sleep(60 * time.Second) {
		m.generateOwnMonitoring()
		statLock.Lock()
		for _, carbonAddr := range m.Conf.CarbonAddrs {
			if m.clientStat[carbonAddr].dropped != 0 {
				m.Lc.lg.Printf("Too many metrics in the main buffer of %s server. Had to drop incommings: %d", carbonAddr, m.clientStat[carbonAddr].dropped)
			}
		}
		m.clean()
		statLock.Unlock()
	}
}
