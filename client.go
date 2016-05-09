package main

import (
	"time"
	"log"
	"os"
	"net"
	"runtime"
)
type Client struct {
	conf Config
	lc LocalConfig
	mon *Monitoring
	graphiteAddr net.TCPAddr
	lg log.Logger
	ch chan string
	chM chan string
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
	defer f.Close()
	if err != nil {
		c.lg.Println("CLIENT:", err.Error())
	}
	/*
	 We are saving only amount of lines which less than c.lc.fileMetricSize
	 Of course we drop old metrics if it is
	 */

	for _, metric := range results_list {
		f.WriteString(metric+"\n")
		c.mon.saved++
	}
	c.removeOldDataFromRetryFile()
}
/*
	Function is cleaning up retry-file
	wholeFile is sorted to have newest metrics on the beginning
	So we need to keep newest metrics
 */
func (c Client) removeOldDataFromRetryFile() {
	currentLinesInFile := getSizeInLinesFromFile(c.conf.RetryFile)
	if currentLinesInFile > c.lc.fileMetricSize {
		c.lg.Printf("I can not save to %s more, than %d. I will have to drop the rest (%d)",
			c.conf.RetryFile, c.lc.fileMetricSize, currentLinesInFile-c.lc.fileMetricSize )
		// We save first c.lc.fileMetricSize of metrics (newest)
		wholeFile := readMetricsFromFile(c.conf.RetryFile)[:c.lc.fileMetricSize]
		// We need to swap our slice cause new data should be at the end
		for i, j := 0, len(wholeFile)-1; i < j; i, j = i+1, j-1 {
			wholeFile[i], wholeFile[j] = wholeFile[j], wholeFile[i]
		}
		c.saveSliceToRetry(wholeFile)
	}
}
/*
	Sending data to graphite:
	1) Metrics from monitor queue
	2) Metrics from main quere
	3) Retry file
 */
func (c Client)runClient() {
	for ;; time.Sleep(time.Duration(c.conf.ClientSendInterval) * time.Second) {
		results_list := []string{}
		readSize := len(c.chM)
		/*
			Max size of queue which we will process this run.
		*/
		maxSendQueue := c.lc.mainBufferSize + readSize

		// Get monitoring data. This must be at the beginning to avoid dropping
		for i := 0; i < readSize; i++ {
			results_list = append(results_list, <-c.chM)
		}

		/*
			We should not read more, than c.conf.MaxMetrics + c.lc.monitoringBuffSize in one try.
			Otherwise we extend our limits:
			We suppose to have limitation per minute lc.mainBufferSize + monitorMetrics,
			But if we read all buffer evey time - we multiply our value 60/c.conf.ClientSendInterval times
		*/
		readSize = len(c.ch)
		if maxSendQueue < readSize {
			readSize = maxSendQueue
		}
		for i := 0; i < readSize; i++ {
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
				continue
			}
			// Check if we do not have too many metrics in buffer already
			if len(results_list) < maxSendQueue {
				// Get all data from "retry" file if there is something
				retryFileMetrics := readMetricsFromFile(c.conf.RetryFile)
				for numOfMetricFromFile, metricFromFile := range retryFileMetrics {
					if len(results_list) < maxSendQueue {
						results_list = append(results_list, metricFromFile)
						c.mon.got.retry++
					} else {
						c.lg.Printf("Can read only %d metrics from %s. Rest will be kept for the next run", numOfMetricFromFile, c.conf.RetryFile)
						c.saveSliceToRetry(retryFileMetrics[numOfMetricFromFile:])
						break
					}
				}
			} else {
				c.lg.Println("Send buffer is full. Will not look into retry file" )
			}
			/*
				We need to send old metrics first, cause newer metrics may overwrite old and we definitely want to
				have newer value.
			*/
			for i := len(results_list)-1; i >= 0; i-- {
				_, err := conn.Write([]byte(results_list[i] + "\n"))
				if err != nil {
					c.lg.Println("Write to server failed:", err.Error())
					c.saveSliceToRetry([]string{results_list[i]})
					c.mon.saved++
				} else {
					c.mon.sent++
				}
			}
			conn.Close()
		}
		// Call gc to cleanup structures
		runtime.GC()
	}
}