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
	f, err := os.OpenFile(c.retryFile, os.O_RDWR | os.O_CREATE | os.O_APPEND, 0600)
	if err != nil {
		c.lg.Println("CLIENT:", err.Error())
	}
	defer f.Close()
	f.Write([]byte(metr+"\n"))
}
/*
 Function saves []string to file. We need it cause it make a lot of IO to save and check size of file
 After every single metric
*/
func (c Client)saveSliceToCache(results_list []string)  {

	for _, metric := range results_list {
		c.saveMetricToCache(metric)
	}

	if c.getFileSize(c.retryFile) > c.retryFileMaxSize {
		c.lg.Println("I have to drop metrics from " + c.retryFile + ", because filesize is: " + strconv.FormatInt(c.getFileSize(c.retryFile),10) )
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
		var results_list[] string

		// Get all data from listener
		for i := 0; i < len(c.ch); i++ {
			results_list = append(results_list, <-c.ch)
		}

		// Check that metric syntax is correct
		for i := 0; i < len(results_list); {
			if ! c.checkMetric(results_list[i]) {
				c.lg.Println("Removing bad metric \"" + results_list[i] + "\" from the list")

				if i < (len(results_list) - 1) {
					results_list = append(results_list[:i], results_list[i + 1:]...)
				} else {
					results_list = results_list[:i]
					break
				}
			} else {
				/*
				 We are increasing 'i' only if we not removed element from slice
				 This is done cause if we removed element we use free slot to shrink array
				  */
				i++
			}
		}

		/*
		If we have correct metrics in queue or retry file - we will try to connect to graphite
		If we do not have connection to graphite - we will not read cache file, which will save us a lot of CPU
		 */
		if _, err := os.Stat(c.retryFile); err == nil || len(results_list) != 0 {
			conn, err := net.DialTCP("tcp", nil, &c.graphiteAddr)
			if err != nil {
				// We can not connect to graphite - append queue in retry file
				c.lg.Println("CLIENT: can not connect to graphite server: ", err.Error())
				c.saveSliceToCache(results_list)
			} else {
				// Get all data from "retry" file if there is something
				results_list = append(results_list, readMetricsFromFile(c.retryFile)...)

				// Check if we do not have too many metrics
				if len(results_list) > c.maxMetrics {
					c.lg.Println("Too many metrics: " + strconv.Itoa(len(results_list)) + ". Will send only " + strconv.Itoa(c.maxMetrics))
					// Saving to retry file metrics which will not be delivered this time
					c.saveSliceToCache(results_list)
					results_list = results_list[:c.maxMetrics]
				}
				// Send metrics to graphite
				for _, metr := range results_list {
					_, err := conn.Write([]byte(metr + "\n"))
					if err != nil {
						c.lg.Println("Write to server failed:", err.Error())
						c.saveSliceToCache([]string{metr})
					}

				}
				if err == nil {
					conn.Close()
				}
			}



		}
		time.Sleep(c.clientSendInterval)
	}
}