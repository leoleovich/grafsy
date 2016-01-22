package main

import (
	"time"
	"log"
	"os"
	"regexp"
//"net"
)

func checkMetric(metric string) bool {
	// Fix regexp
	match, _ := regexp.MatchString("^([-a-zA-Z0-9_.]+){3,}", metric)
	return match
}
// Function writes to cache file metric. These metrics will be retransmitted
func saveMetricToCache(metr string)  {
	graphiteCacheFile := "/tmp/grafsy-cache-retry"
	f, _ := os.OpenFile(graphiteCacheFile, os.O_RDWR | os.O_CREATE | os.O_APPEND, 0660)
	f.Write([]byte(metr))
}

// Sending data to graphite
func client(maxMetric int, graphiteAddr string, c chan string, lg log.Logger) {
	/*_graphiteAdrr, err := net.ResolveTCPAddr("tcp", graphiteAddr)
	if err != nil {
		lg.Println("This is not a valid address:", err.Error())
		os.Exit(1)
	}
	*/
	for {
		time.Sleep(2 * time.Second)
		results_list := make(map[int] string)

		// Check file cache

		for i := 0; i< len(c); i++ {
			results_list[i] = <-c
		}
		// Check metric syntax is correct
		for i, metr := range results_list {
			if ! checkMetric(metr) {
				lg.Println("Removing bad metric from list")
				delete(results_list, i)
			}
		}
		// Check if we do not have too many metrics
		if len(results_list) > maxMetric {
			lg.Println("Too many metrics " + string(len(results_list)) + ". Will send only " + string(maxMetric))
			for i := 10000; i < len(results_list); i++ {
				saveMetricToCache(results_list[i])
				delete(results_list, i)
			}
		}

		// Send data to graphite
		//conn, _ := net.DialTCP("tcp", nil, _graphiteAdrr)
		lg.Println("CLIENT:" + string(len(results_list)))
		for _, metr := range results_list {
			/*_, err := conn.Write([]byte(metr))
			if err != nil {
				lg.Println("Write to server failed:", err.Error())
				saveMetricToCache(metr)
			}
			*/
			lg.Println("CLIENT:" + metr)
		}
	}
}