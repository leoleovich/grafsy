package main

import (
	"strconv"
	"os"
	"strings"
	"time"
)

type Monitoring struct {
	conf Config
	got Source
	saved int
	sent int
	dropped int
	ch chan string
}
type Source struct {
	net int
	dir int
	retry int
}

func (m *Monitoring) generateOwnMonitoring(){
	hostname,_ := os.Hostname()
	hostnameForGraphite := strings.Replace(hostname, ".", "_", -1)
	path := m.conf.GrafsyPrefix + "."+ hostnameForGraphite + "." + m.conf.GrafsySuffix + ".grafsy"

	m.ch <- path + ".got.net " + strconv.Itoa(m.got.net)
	m.ch <- path + ".got.dir " + strconv.Itoa(m.got.dir)
	m.ch <- path + ".got.retry " + strconv.Itoa(m.got.retry)
	m.ch <- path + ".saved " + strconv.Itoa(m.saved)
	m.ch <- path + ".sent " + strconv.Itoa(m.sent)
	m.ch <- path + ".dropped " + strconv.Itoa(m.dropped)
}

func (m *Monitoring) clean() *Monitoring{
	m.saved = 0
	m.sent = 0
	m.dropped = 0
	m.got = Source{0,0,0}
	return m
}

func (m *Monitoring) runMonitoring() {
	for ;; time.Sleep(60*time.Second) {
		m.generateOwnMonitoring()
		*m = *m.clean()
	}
}