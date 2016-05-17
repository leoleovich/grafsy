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
func (c Client)saveSliceToRetry(metrics []string)  {
	/*
	If size of file is bigger, than max size we will remove lines from this file,
	and will call this function again to check result and write to the file.
	Recursion:)
	 */
	f, err := os.OpenFile(c.conf.RetryFile, os.O_RDWR | os.O_CREATE | os.O_APPEND, 0600)
	if err != nil {
		c.lg.Println(err.Error())
	}

	for _, metric := range metrics {
		f.WriteString(metric+"\n")
		c.mon.saved++
	}
	f.Close()
	c.removeOldDataFromRetryFile()
}

func (c Client)saveMetricToRetry(metric string)  {
	/*
	If size of file is bigger, than max size we will remove lines from this file,
	and will call this function again to check result and write to the file.
	Recursion:)
	 */
	f, err := os.OpenFile(c.conf.RetryFile, os.O_RDWR | os.O_CREATE | os.O_APPEND, 0600)
	if err != nil {
		c.lg.Println(err.Error())
	}

	f.WriteString(metric+"\n")
	c.mon.saved++
	f.Close()
}

func (c Client)saveChannelToRetry(ch chan string, size int)  {
	/*
	If size of file is bigger, than max size we will remove lines from this file,
	and will call this function again to check result and write to the file.
	Recursion:)
	 */

	f, err := os.OpenFile(c.conf.RetryFile, os.O_RDWR | os.O_CREATE | os.O_APPEND, 0600)
	if err != nil {
		c.lg.Println(err.Error())
	}

	for i:=0; i<size ; i++ {
		f.WriteString( <-ch + "\n")
		c.mon.saved++
	}
	f.Close()
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
		c.saveSliceToRetry(wholeFile)
	}
}

func (c Client) tryToSendToGraphite(metric string, conn net.Conn) {
	_, err := conn.Write([]byte(metric + "\n"))
	if err != nil {
		c.lg.Println("Write to server failed:", err.Error())
		c.saveMetricToRetry(metric)
		c.mon.saved++
	} else {
		c.mon.sent++
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
		// Call gc to cleanup structures
		runtime.GC()


		conn, err := net.DialTCP("tcp", nil, &c.graphiteAddr)
		if err != nil {
			c.lg.Println("Can not connect to graphite server: ", err.Error())
			// Monitoring
			bufSize := len(c.chM)
			for i :=0 ; i < bufSize ; i++ {
				c.saveMetricToRetry(<-c.chM)
			}

			// Main Buffer
			bufSize = len(c.ch)
			for i :=0 ; i < bufSize ; i++ {
				c.saveMetricToRetry( <-c.ch)
			}
			c.removeOldDataFromRetryFile()
			continue
		} else {
			processedTotal := 0

			// We send retry file first, cause old data needs to be sent before an old data
			retryFileMetrics := readMetricsFromFile(c.conf.RetryFile)
			for numOfMetricFromFile, metricFromFile := range retryFileMetrics {
				if numOfMetricFromFile+1 < c.lc.mainBufferSize {
					c.tryToSendToGraphite(metricFromFile, conn)
					c.mon.got.retry++
				} else {
					c.lg.Printf("Can read only %d metrics from %s. Rest will be kept for the next run", numOfMetricFromFile+1, c.conf.RetryFile)
					c.saveSliceToRetry(retryFileMetrics[numOfMetricFromFile:])
					break
				}
				processedTotal++
			}


			// Monitoring. We read it always and we reserved space for it
			bufSize := len(c.chM)
			for i :=0 ; i < bufSize ; i++ {
				c.tryToSendToGraphite(<-c.chM, conn)
			}

			/*
			 Main Buffer. We read it completely but send only part which fits in mainBufferSize
			 Rests we save
			*/

			bufSize = len(c.ch)
			for processedMainBuff :=0 ; processedMainBuff < bufSize ; processedMainBuff, processedTotal = processedMainBuff+1, processedTotal+1  {
				if processedTotal < c.lc.mainBufferSize {
					c.tryToSendToGraphite(<-c.ch, conn)
				} else {
					/*
					 Save only data for the moment of run. Concurrent goroutines know no mercy
					 and they continue to write...
					  */
					c.saveChannelToRetry(c.ch, bufSize-processedMainBuff)
					break
				}
			}
		}
		conn.Close()
	}
}