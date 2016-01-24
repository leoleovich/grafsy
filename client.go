package main

import (
	"time"
	"log"
	"os"
	"regexp"
	"net"
	"bufio"
	"io/ioutil"
)
type Client struct {
	clientSendInterval time.Duration
	maxMetric int
	graphiteAddr net.TCPAddr
	metricDir string
	retryFile string
	retryFileMaxSize int64
	lg log.Logger
	ch chan string
}

func (c Client)checkMetric(metric string) bool {
	// Fix regexp
	match, _ := regexp.MatchString("^[-a-zA-Z0-9_]+.[-a-zA-Z0-9_]+.\\S+(\\s)[-0-9.eE+]+(\\s)[0-9]{10}", metric)
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

// Reading metrics from file and remove file afterwords
func (c Client)readMetricsFromFile(file string) []string {
	var results_list []string
	f, err := os.Open(file)
	if err != nil {
		return results_list
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		results_list = append(results_list, scanner.Text())
	}
	os.Remove(file)
	return results_list
}

// Reading metrics from files in folder. This is a second way how to send metrics, except direct connection
func (c Client)readMetricsFromDir() []string {
	var results_list []string
	files, err := ioutil.ReadDir(c.metricDir)
	if err != nil {
		panic(err.Error())
		return results_list
	}
	for _, f := range files {
		results_list = append(results_list, c.readMetricsFromFile(c.metricDir+"/"+f.Name())...)
	}
	return results_list
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
	wholeFile := c.readMetricsFromFile(c.retryFile)
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
		results_list := c.readMetricsFromFile(c.retryFile)

		// Get all data from metrics files
		results_list = append(results_list, c.readMetricsFromDir()...)

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
		if len(results_list) > c.maxMetric {
			c.lg.Println("Too many metrics " + string(len(results_list)) + ". Will send only " + string(c.maxMetric))
			for i := 10000; i < len(results_list); i++ {
				c.saveMetricToCache(results_list[i])
				results_list = append(results_list[:i], results_list[i+1:]...)
			}
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