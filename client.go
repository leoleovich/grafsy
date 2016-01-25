package main

import (
	"time"
	"log"
	"os"
	"regexp"
	"net"
	"strconv"
)
type Client struct {
	clientSendInterval time.Duration
	maxMetrics int
	graphiteAddr net.TCPAddr
	retryFile string
	retryFileMaxSize int64
	lg log.Logger
	ch chan string
}

func (c Client)checkMetric(metric string) bool {
	// Fix regexp
	match, _ := regexp.MatchString("^([-a-zA-Z0-9_]+\\.){2}[-a-zA-Z0-9_.]+(\\s)[-0-9.eE+]+(\\s)[0-9]{10}", metric)
	return match
}
// Function writes to cache file metric. These metrics will be retransmitted
func (c Client)saveMetricToCache(metr string)  {
	/*
	If size of file is bigger, than max size we will remove lines from this file,
	and will call this function again to check result and write to the file.
	Recursion:)
	 */
	f, err := os.OpenFile(c.retryFile, os.O_RDWR | os.O_CREATE | os.O_APPEND, 0660)
	if err != nil {
		c.lg.Println("CLIENT:", err.Error())
	}
	f.Write([]byte(metr+"\n"))
	defer f.Close()

	if c.getFileSize(c.retryFile) > c.retryFileMaxSize {
		c.lg.Println("I have to drop metrics from " + c.retryFile )
		c.removeOldDataFromRetryFile()
	}
}

// Function takes file size and returning it as int64 in bytes
func (c Client) getFileSize(file string) int64 {
	f, err := os.Open(file)
	if err != nil {
		return 0
	}
	defer f.Close()
	// get the file size
	stat, err := f.Stat()
	if err != nil {
		return 0
	}
	return stat.Size()
}

func (c Client) removeOldDataFromRetryFile() {
	realFileSize := c.getFileSize(c.retryFile)
	wholeFile := readMetricsFromFile(c.retryFile)
	var sizeOfLines int64
	for num,line := range wholeFile {
		/*
		Calculating size of strings and waiting till it will be more, than difference between
		real file size and maximum amount.
		 */
		sizeOfLines += int64(len([]byte(line)))
		if sizeOfLines > realFileSize - c.retryFileMaxSize {
			wholeFile = append(wholeFile[:0], wholeFile[num+1:]...)
			break
		}
	}
	for _, metric := range wholeFile {
		c.saveMetricToCache(metric)
	}
}

// Sending data to graphite
func (c Client)runClient() {

	for {
		time.Sleep(c.clientSendInterval)

		// Get all data from "retry" file if ther is something
		results_list := readMetricsFromFile(c.retryFile)

		// Get all data from listener
		for i := 0; i< len(c.ch); i++ {
			results_list = append(results_list, <-c.ch)
		}

		// Check that metric syntax is correct
		for i, metr := range results_list {
			if ! c.checkMetric(metr) {
				c.lg.Println("Removing bad metric from list")
				results_list = append(results_list[:i], results_list[i+1:]...)
			}
		}

		// Check if we do not have too many metrics
		if len(results_list) > c.maxMetrics {
			c.lg.Println("Too many metrics: " + strconv.Itoa(len(results_list)) + ". Will send only " + strconv.Itoa(c.maxMetrics))
			// Saving to retry file metrics which will not be delivered this time
			for i := c.maxMetrics; i < len(results_list); i++ {
				c.saveMetricToCache(results_list[i])
			}
			results_list = results_list[:c.maxMetrics]
		}

		// Send data to graphite
		conn, err := net.DialTCP("tcp", nil, &c.graphiteAddr)

		for _,metr := range results_list {
			if err != nil {
				c.lg.Println("Connect to server failed:", err.Error())
				c.saveMetricToCache(metr)
			} else {
				_, err := conn.Write([]byte(metr))
				if err != nil {
					c.lg.Println("Write to server failed:", err.Error())
					c.saveMetricToCache(metr)
				}
			}
			//c.lg.Println("CLIENT:" + metr)
		}
		if err == nil {
			conn.Close()
		}
	}
}