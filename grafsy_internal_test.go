package grafsy

import (
	"bufio"
	"net"
	"path"
	"reflect"
	"strings"
	"testing"
)

var cleanMonitoring = &Monitoring{
	Conf: conf,
	Lc:   lc,
	serverStat: serverStat{
		dir:     0,
		invalid: 0,
		net:     0,
	},
	clientStat: map[string]*clientStat{
		"localhost:2003": &clientStat{
			saved:      0,
			sent:       0,
			dropped:    0,
			aggregated: 0,
		},
		"localhost:2004": &clientStat{
			saved:      0,
			sent:       0,
			dropped:    0,
			aggregated: 0,
		},
	},
}

// These variables defined to prevent reading the config multiple times
// and avoid code duplication
var conf, lc, configError = getConfigs()

// There are 3 metrics per backend
var mon, monError = generateMonitoringObject()
var serverStatMetrics = reflect.ValueOf(mon.serverStat).NumField()
var clientStatMetrics = reflect.TypeOf(mon.clientStat).Elem().Elem().NumField()
var MonitorMetrics = serverStatMetrics + len(conf.CarbonAddrs)*clientStatMetrics
var cli = Client{
	Conf: conf,
	Lc:   lc,
	Mon:  mon,
	monChannels: map[string]chan string{
		"localhost:2003": make(chan string, len(testMetrics)),
		"localhost:2004": make(chan string, len(testMetrics)),
	},
}

// These metrics are used in many tests. Check before updating them
var testMetrics = []string{
	"test.oleg.test 8 1500000000",
	"whoop.whoop 11 1500000000",
}

func generateMonitoringObject() (*Monitoring, error) {

	return &Monitoring{
		Conf: conf,
		Lc:   lc,
		serverStat: serverStat{
			net:     1,
			invalid: 4,
			dir:     2,
		},
		clientStat: map[string]*clientStat{
			"localhost:2003": &clientStat{
				1,
				3,
				2,
				4,
				5,
			},
			"localhost:2004": &clientStat{
				1,
				3,
				2,
				4,
				5,
			},
		},
	}, nil
}

func getConfigs() (*Config, *LocalConfig, error) {
	conf := &Config{}
	err := conf.LoadConfig("grafsy.toml")
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

func TestConfig_MonitoringChannelCapacity(t *testing.T) {
	testConf := &Config{}
	err := testConf.LoadConfig("grafsy.toml")
	if err != nil {
		t.Error("Fail to load config")
	}
	backendSets := make([][]string, 5)
	backendSets = append(backendSets, []string{"localhost:2003"})
	backendSets = append(backendSets, []string{"localhost:2003", "localhost:2003"})
	backendSets = append(backendSets, []string{"localhost:2003", "localhost:2003", "localhost:2003"})
	backendSets = append(backendSets, []string{"localhost:2003", "localhost:2003", "localhost:2003", "localhost:2003"})
	backendSets = append(backendSets, []string{"localhost:2003", "localhost:2003", "localhost:2003", "localhost:2003", "localhost:2003"})
	for _, backends := range backendSets {
		testConf.CarbonAddrs = backends
		testLc, err := testConf.GenerateLocalConfig()
		if err != nil {
			t.Error("Fail to generate local config")
		}
		metricsLen := serverStatMetrics + len(backends)*clientStatMetrics
		monCap := cap(testLc.monitoringChannel)
		if monCap != metricsLen {
			t.Errorf("The formula for monitoring channel capasity is wrong, got %v, should be %v", monCap, metricsLen)
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
		t.Logf("Fix the amount of metrics for server stats to %v and client stats to %v\n", serverStatMetrics, clientStatMetrics)
		t.Errorf("Mismatch amount of the monitor metrics: expected=%v, gotten=%v", MonitorMetrics, len(mon.Lc.monitoringChannel))
	}
}

func TestMonitoring_clean(t *testing.T) {
	m, _ := generateMonitoringObject()
	m.Conf = conf
	m.Lc = lc
	m.clean()

	if !reflect.DeepEqual(*cleanMonitoring, *m) {
		t.Errorf("Monitoring was not cleaned up:\n Sample: %+v\n Gotten: %+v", cleanMonitoring, m)
	}
}

func TestMetricData_getSizeInLinesFromFile(t *testing.T) {
	if getSizeInLinesFromFile("grafsy.toml") == 0 {
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
	err := cli.createRetryDir()
	if err != nil {
		t.Error(err)
	}

	err = cli.saveSliceToRetry(testMetrics, conf.CarbonAddrs[0])
	if err != nil {
		t.Error(err)
	}

	metrics, err := readMetricsFromFile(path.Join(conf.RetryDir, conf.CarbonAddrs[0]))
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
	// Pretend to be a server with random port
	carbonServer := "localhost:0"
	l, err := net.Listen("tcp", carbonServer)
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

	// Create monitoring structure for statistic
	cli.Mon.clientStat[carbonServer] = &clientStat{0, 0, 0, 0, 0}

	for _, metric := range testMetrics {
		cli.tryToSendToGraphite(metric, carbonServer, conn)
	}

	// Read from channel
	for i := 0; i < len(testMetrics); i++ {
		received := <-ch
		if received != testMetrics[i] {
			t.Error("Received something different than sent")
		}
	}
}
