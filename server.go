package main

import (
	"log"
	"os"
	"net"
	"io/ioutil"
	"time"
	"regexp"
	"strings"
	"strconv"
	"bufio"
)

type Server struct {
	conf Config
	lc LocalConfig
	mon *Monitoring
	lg log.Logger
	ch chan string
	chS chan string
	chA chan string
}


func (s Server) combineMetricsWithSameName(metric string, metrics []Metric) []Metric {
	split := regexp.MustCompile("\\s").Split(metric, 3)

	value, err := strconv.ParseFloat(split[1], 64)
	if err != nil {s.lg.Println("Can not parse value of metric " + split[0] + ": " + split[1]) ; return metrics}
	timestamp, err := strconv.ParseInt(split[2], 10, 64)
	if err != nil {s.lg.Println("Can not parse timestamp of metric " + split[0] + ": " + split[2]) ; return metrics}

	/*
	Go through existing metrics and search for the same name of metric
	If there is no same metric - append it as a new
	 */
	for i,_ := range metrics {
		if metrics[i].name == split[0] {
			metrics[i].amount++
			metrics[i].value += value
			metrics[i].timestamp += timestamp
			return metrics
		}
	}
	metrics = append(metrics, Metric{split[0], 1, value, timestamp})
	return metrics
}
// Sum metrics with prefix
func (s Server) sumMetricsWithPrefix() {
	for ;; time.Sleep(time.Duration(s.conf.SumInterval)*time.Second) {
		var working_list[] Metric
		chanSize := len(s.chS)
		for i := 0; i < chanSize; i++ {
			working_list = s.combineMetricsWithSameName(strings.Replace(<-s.chS, s.conf.SumPrefix, "", -1), working_list)
		}
		/*
			We may have a problem, that working_list size will be bigger than main buffer/space in it.
			But then go suppose to block appending into buffer and wait until space will be free.
			I am not sure if we need to check free space of main buffer here...
		 */
		for _,val := range working_list {
			s.ch <- val.name + " " +
				strconv.FormatFloat(val.value, 'f', 2, 32) + " " + strconv.FormatInt(val.timestamp/val.amount, 10)
		}
	}
}

// AVG metrics with prefix
func (s Server) avgMetricsWithPrefix() {
	for ;; time.Sleep(time.Duration(s.conf.AvgInterval)*time.Second) {
		var working_list[] Metric
		chanSize := len(s.chA)
		for i := 0; i < chanSize; i++ {
			working_list = s.combineMetricsWithSameName(strings.Replace(<-s.chA, s.conf.AvgPrefix, "", -1), working_list)
		}
		/*
			We may have a problem, that working_list size will be bigger than main buffer/space in it.
			But then go suppose to block appending into buffer and wait until space will be free.
			I am not sure if we need to check free space of main buffer here...
		 */
		for _,val := range working_list {
			s.ch <- val.name + " " +
				strconv.FormatFloat(val.value/float64(val.amount), 'f', 2, 32) + " " + strconv.FormatInt(val.timestamp/val.amount, 10)
		}
	}
}

/*
	Validate metrics list
	Find proper channel for metric
	Check overflow of the channel
	Put metric in a proper channel
 */
func (s Server)cleanAndUseIncomingData(metrics []string) {
	for _,metric := range metrics {
		if validateMetric(metric, s.conf.AllowedMetrics) {
			if strings.HasPrefix(metric, s.conf.SumPrefix) {
				if len(s.chS) < s.lc.sumBufSize {
					s.chS <- metric
				} else {
					s.lg.Println("Too many metrics in the SUM queue (%d). I have to drop incommings", len(s.chS))
					s.mon.dropped++
				}
			} else if strings.HasPrefix(metric, s.conf.AvgPrefix) {
				if len(s.chA) < s.lc.avgBufSize {
					s.chA <- metric
				} else {
					s.lg.Println("Too many metrics in the AVG queue (%d). I have to drop incommings", len(s.chA))
					s.mon.dropped++
				}
			} else {
				if len(s.ch) < s.lc.mainBufferSize {
					s.ch <- metric
				} else {
					s.lg.Printf("Too many metrics in the main queue (%d). I have to drop incommings", len(s.ch))
					s.mon.dropped++
				}
			}
		} else {
			if metric != "" {
				s.mon.dropped++
				s.lg.Printf("Removing bad metric '%s' from the list", metric)
			}
		}
	}
}

// Reading metrics from network
func (s Server)handleRequest(conn net.Conn) {

	connbuf := bufio.NewReader(conn)
	defer conn.Close()
	for ;; {
		s.mon.got.net++
		metric, err := connbuf.ReadString('\n')
		// Even if error occurred we still put "metric" into analysis, cause it can be a valid metric, but without \n
		s.cleanAndUseIncomingData([]string{strings.Replace(strings.Replace(metric, "\r", "", -1), "\n", "", -1)})

		if err!= nil {
			conn.Close()
			break
		}
	}
}

// Reading metrics from files in folder. This is a second way how to send metrics, except network
func (s Server)handleDirMetrics() {
	for ;; time.Sleep(time.Duration(s.conf.ClientSendInterval)*time.Second) {
		files, err := ioutil.ReadDir(s.conf.MetricDir)
		if err != nil {
			panic(err.Error())
		}
		for _, f := range files {
			results_list := readMetricsFromFile(s.conf.MetricDir+"/"+f.Name())
			s.mon.got.dir += len(results_list)
			s.cleanAndUseIncomingData(results_list)
		}

	}
}

func (s Server)runServer() {
	// Listen for incoming connections.
	l, err := net.Listen("tcp", s.conf.LocalBind)
	if err != nil {
		s.lg.Println("Failed to run server:", err.Error())
		os.Exit(1)
	} else {
		s.lg.Println("Server is running")
	}
	// Close the listener when the application closes.
	defer l.Close()

	// Run goroutine for reading metrics from metricDir
	go s.handleDirMetrics()
	// Run goroutine for sum metrics with prefix
	go s.sumMetricsWithPrefix()
	// Run goroutine for avg metrics with prefix
	go s.avgMetricsWithPrefix()

	for {
		// Listen for an incoming connection.
		conn, err := l.Accept()
		if err != nil {
			s.lg.Println("Error accepting: ", err.Error())
			os.Exit(1)
		}
		// Handle connections in a new goroutine.
		go s.handleRequest(conn)
	}
}