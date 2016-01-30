package main

import (
	"time"
	"log"
	"os"
	"net"
	"strconv"
	"runtime"
)
type Client struct {
	conf Config
	graphiteAddr net.TCPAddr
	lg log.Logger
	ch chan string
}


// Function takes file size and returning it as int64 in bytes
func (c Client) getFileSize(file string) int64 {
	f, err := os.Open(file)
	if err != nil {
		return 0
	}
	stat, err := f.Stat()
	f.Close()
	if err != nil {
		return 0
	}
	return stat.Size()
}
/*
 Function saves []string to file. We need it cause it make a lot of IO to save and check size of file
 After every single metric
*/
func (c Client)saveSliceToRetry(results_list []string)  {
	/*
	If size of file is bigger, than max size we will remove lines from this file,
	and will call this function again to check result and write to the file.
	Recursion:)
	 */
	f, err := os.OpenFile(c.conf.RetryFile, os.O_RDWR | os.O_CREATE | os.O_APPEND, 0600)
	if err != nil {
		c.lg.Println("CLIENT:", err.Error())
	}
	for _, metric := range results_list {
		f.WriteString(metric+"\n")
	}
	f.Close()

	if c.getFileSize(c.conf.RetryFile) > int64(c.conf.MaxMetrics*c.conf.ClientSendInterval*metricsSize) {
		c.lg.Println("I have to drop metrics from " + c.conf.RetryFile + ", because filesize is: " + strconv.FormatInt(c.getFileSize(c.conf.RetryFile),10) )
		c.removeOldDataFromRetryFile()
	}
}

func (c Client) removeOldDataFromRetryFile() {
	realFileSize := c.getFileSize(c.conf.RetryFile)
	wholeFile := readMetricsFromFile(c.conf.RetryFile)
	var sizeOfLines int64
	for num,line := range wholeFile {
		/*
		Calculating size of strings and waiting till it will be more, than difference between
		real file size and maximum amount.
		 */
		sizeOfLines += int64(len([]byte(line)))
		if sizeOfLines > realFileSize - int64(c.conf.MaxMetrics*c.conf.ClientSendInterval*metricsSize) {
			c.saveSliceToRetry(wholeFile[(num+1):])
			return
		}
	}

}

// Sending data to graphite
func (c Client)runClient() {
	for results_list := []string{};; time.Sleep(time.Duration(c.conf.ClientSendInterval) * time.Second) {
		// Get all data from server part
		chanSize := len(c.ch)
		for i := 0; i < chanSize; i++ {
			results_list = append(results_list, <-c.ch)
		}
		/*
		If we have correct metrics in queue or retry file - we will try to connect to graphite
		If we do not have connection to graphite - we will not read cache file, which will save us a lot of CPU
		 */
		if _, err := os.Stat(c.conf.RetryFile); err == nil || len(results_list) != 0 {
			conn, err := net.DialTCP("tcp", nil, &c.graphiteAddr)
			if err != nil {
				// We can not connect to graphite - append queue in retry file
				c.lg.Println("CLIENT: can not connect to graphite server: ", err.Error())
				c.saveSliceToRetry(results_list)
			} else {
				// Check if we do not have too many metrics
				if len(results_list) > c.conf.MaxMetrics {
					c.lg.Println("Too many metrics in buffer: " +
						strconv.Itoa(len(results_list)) + ". Do not read from retry. Will send only " +
						strconv.Itoa(c.conf.MaxMetrics))
					// Saving to retry file metrics which will not be delivered this time
					c.saveSliceToRetry(results_list[c.conf.MaxMetrics:])
					results_list = results_list[:c.conf.MaxMetrics]
				} else {
					// Get all data from "retry" file if there is something
					results_list = append(results_list, readMetricsFromFile(c.conf.RetryFile)...)
					// Check again if we have too many metrics
					if len(results_list) > c.conf.MaxMetrics {
						c.lg.Println("Too many metrics after reading retry: " +
							strconv.Itoa(len(results_list)) + ". Will send only " + strconv.Itoa(c.conf.MaxMetrics))
						c.saveSliceToRetry(results_list[c.conf.MaxMetrics:])
						results_list = results_list[:c.conf.MaxMetrics]
					}
				}
				// Send metrics to graphite
				for _, metr := range results_list {
					_, err := conn.Write([]byte(metr + "\n"))
					if err != nil {
						c.lg.Println("Write to server failed:", err.Error())
						c.saveSliceToRetry([]string{metr})
					}
				}
				if err == nil {
					conn.Close()
				}
			}
		}
		results_list = nil
		runtime.GC()
	}
}