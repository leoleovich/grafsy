package grafsy

import (
	"bufio"
	"net"
	"strings"
	"testing"
)

var cleanMonitoring = &Monitoring{
	got: source{
		retry: 0,
		dir:   0,
		net:   0,
	},
	dropped: 0,
	invalid: 0,
	saved:   0,
	sent:    0,
}

// These variables defined to prevent reading the config multiple times
// and avoid code duplication
var conf, lc, configError = getConfigs()
var mon, monError = generateMonitoringObject()
var cli = Client{
	conf,
	lc,
	mon,
}

// These metrics are used in many tests. Check before updating them
var testMetrics = []string{
	"test.oleg.test 8 1500000000",
	"whoop.whoop 11 1500000000",
}

func generateMonitoringObject() (*Monitoring, error) {
	s := source{
		net:   1,
		dir:   2,
		retry: 3,
	}

	return &Monitoring{
		Conf:    conf,
		Lc:      lc,
		got:     s,
		dropped: 1,
		invalid: 2,
		saved:   3,
		sent:    4,
	}, nil
}

func getConfigs() (*Config, *localConfig, error) {
	conf := &Config{}
	err := conf.LoadConfig("/etc/grafsy/grafsy.toml")
	if err != nil {
		return nil, nil, err
	}

	lc, err := conf.GenerateLocalConfig()
	return conf, lc, err
}

func acceptAndReport(l net.Listener, ch chan string) error {
	conn, err := l.Accept()
	if err != nil {
		return err
	}

	defer conn.Close()
	conBuf := bufio.NewReader(conn)
	for {
		metric, err := conBuf.ReadString('\n')
		ch <- strings.Replace(strings.Replace(metric, "\r", "", -1), "\n", "", -1)
		if err != nil {
			return err
		}
	}
}

func TestConfig_GenerateLocalConfig(t *testing.T) {
	if configError != nil {
		t.Error(configError)
	}
}

func TestMonitoring_generateOwnMonitoring(t *testing.T) {
	if monError != nil {
		t.Error("Can not generate monitoring object", monError)
	}

	mon.generateOwnMonitoring()
	if len(mon.Lc.monitoringChannel) != MonitorMetrics {
		t.Error("Mismatch amount of the monitor metrics:", len(mon.Lc.monitoringChannel))
	}
}

func TestMonitoring_clean(t *testing.T) {
	m, _ := generateMonitoringObject()
	m.Conf = nil
	m.Lc = nil
	m.clean()

	if *cleanMonitoring != *m {
		t.Error("Monitoring was not cleaned up")
	}
}

func TestMetricData_getSizeInLinesFromFile(t *testing.T) {
	if getSizeInLinesFromFile("/etc/grafsy/grafsy.toml") == 0 {
		t.Error("Can not be 0 lines in config file")
	}
}

func TestConfg_generateRegexpsForOverwrite(t *testing.T) {
	if configError != nil {
		t.Error(configError)
	}

	conf.Overwrite = []struct {
		ReplaceWhatRegexp, ReplaceWith string
	}{{"^test.*test ", "does not matter"}}

	regexps := conf.generateRegexpsForOverwrite()
	if len(regexps) != 1 {
		t.Error("There must be only 1 regexp")
	}

	if !regexps[0].MatchString(testMetrics[0]) {
		t.Error("Test regexp does not match")
	}
}

func TestClient_retry(t *testing.T) {
	var metricsFound int

	err := cli.saveSliceToRetry(testMetrics)
	if err != nil {
		t.Error(err)
	}

	metrics, err := readMetricsFromFile(conf.RetryFile)
	if err != nil {
		t.Error(err)
	}

	for _, m := range metrics {
		for _, tm := range testMetrics {
			if tm == m {
				metricsFound++
			}
		}
	}

	if metricsFound != len(testMetrics) {
		t.Error("Not all metrics were saved/read")
	}
}

func TestClient_tryToSendToGraphite(t *testing.T) {
	// Pretend to be a server
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Error(err)
	}
	ch := make(chan string, len(testMetrics))
	go acceptAndReport(l, ch)

	// Send as client
	conn, err := net.Dial("tcp", l.Addr().String())
	if err != nil {
		t.Error("Unable to connect to", l.Addr().String())
	}

	for _, metric := range testMetrics {
		cli.tryToSendToGraphite(metric, conn)
	}

	// Read from channel
	for i := 0; i < len(testMetrics); i++ {
		received := <-ch
		if received != testMetrics[i] {
			t.Error("Received something different than sent")
		}
	}
}
