package grafsy

import (
	"bufio"
	"net"
	"os"
	"path"
	"reflect"
	"strings"
	"syscall"
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
		ch <- strings.TrimRight(metric, "\r\n")
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

func TestConfig_PrepareEnvironment(t *testing.T) {
	// Create a temporary directory for testing
	tempDir := t.TempDir()

	// Test case 1: Directory doesn't exist - should create with 777 permissions
	testConf := &Config{
		MetricDir:   path.Join(tempDir, "test_metrics"),
		Log:         "-", // Use stdout to avoid log file issues
		CarbonAddrs: []string{"localhost:2003"},
		UseACL:      false,
	}

	// Ensure directory doesn't exist initially
	if _, err := os.Stat(testConf.MetricDir); !os.IsNotExist(err) {
		t.Fatalf("Test directory should not exist initially")
	}

	// Call prepareEnvironment
	err := testConf.prepareEnvironment()
	if err != nil {
		t.Fatalf("prepareEnvironment failed: %v", err)
	}

	// Check that directory was created
	info, err := os.Stat(testConf.MetricDir)
	if err != nil {
		t.Fatalf("Directory was not created: %v", err)
	}

	// Verify it's a directory
	if !info.IsDir() {
		t.Fatalf("Created path is not a directory")
	}

	// Check permissions - should be 777 with sticky bit (01777)
	expectedPerms := os.FileMode(0777 | os.ModeSticky)
	actualPerms := info.Mode().Perm() | (info.Mode() & os.ModeSticky)
	if actualPerms != expectedPerms {
		t.Errorf("Wrong permissions: expected %o, got %o", expectedPerms, actualPerms)
	}

	// Test case 2: Directory exists - should chmod to 777 permissions
	testConf2 := &Config{
		MetricDir:   path.Join(tempDir, "existing_metrics"),
		Log:         "-",
		CarbonAddrs: []string{"localhost:2003"},
		UseACL:      false,
	}

	// Create directory with wrong permissions first
	err = os.MkdirAll(testConf2.MetricDir, 0755)
	if err != nil {
		t.Fatalf("Failed to create test directory: %v", err)
	}

	// Verify it has wrong permissions initially
	info, err = os.Stat(testConf2.MetricDir)
	if err != nil {
		t.Fatalf("Failed to stat existing directory: %v", err)
	}

	initialPerms := info.Mode().Perm()
	if initialPerms == (0777 | os.ModeSticky) {
		t.Fatalf("Directory already has correct permissions, test invalid")
	}

	// Call prepareEnvironment
	err = testConf2.prepareEnvironment()
	if err != nil {
		t.Fatalf("prepareEnvironment failed on existing directory: %v", err)
	}

	// Check that permissions were fixed
	info, err = os.Stat(testConf2.MetricDir)
	if err != nil {
		t.Fatalf("Failed to stat directory after prepareEnvironment: %v", err)
	}

	actualPerms = info.Mode().Perm() | (info.Mode() & os.ModeSticky)
	if actualPerms != expectedPerms {
		t.Errorf("Permissions not fixed: expected %o, got %o", expectedPerms, actualPerms)
	}

	// Test case 3: Test with UseACL enabled (should still work with noacl build)
	testConf3 := &Config{
		MetricDir:   path.Join(tempDir, "acl_metrics"),
		Log:         "-",
		CarbonAddrs: []string{"localhost:2003"},
		UseACL:      true, // This will call setACL
	}

	err = testConf3.prepareEnvironment()
	if err != nil {
		t.Fatalf("prepareEnvironment failed with UseACL=true: %v", err)
	}

	// Verify directory was created with correct permissions
	info, err = os.Stat(testConf3.MetricDir)
	if err != nil {
		t.Fatalf("Directory was not created with UseACL=true: %v", err)
	}

	actualPerms = info.Mode().Perm() | (info.Mode() & os.ModeSticky)
	if actualPerms != expectedPerms {
		t.Errorf("Wrong permissions with UseACL=true: expected %o, got %o", expectedPerms, actualPerms)
	}

	// Test case 4: Test umask doesn't interfere
	// Save current umask and set a restrictive one
	oldUmask := syscall.Umask(0077) // Very restrictive umask
	defer syscall.Umask(oldUmask)   // Restore original umask

	testConf4 := &Config{
		MetricDir:   path.Join(tempDir, "umask_test"),
		Log:         "-",
		CarbonAddrs: []string{"localhost:2003"},
		UseACL:      false,
	}

	err = testConf4.prepareEnvironment()
	if err != nil {
		t.Fatalf("prepareEnvironment failed with restrictive umask: %v", err)
	}

	// Even with restrictive umask, should still get 777 permissions
	info, err = os.Stat(testConf4.MetricDir)
	if err != nil {
		t.Fatalf("Directory was not created with restrictive umask: %v", err)
	}

	actualPerms = info.Mode().Perm() | (info.Mode() & os.ModeSticky)
	if actualPerms != expectedPerms {
		t.Errorf("Umask interfered with permissions: expected %o, got %o", expectedPerms, actualPerms)
	}
}
