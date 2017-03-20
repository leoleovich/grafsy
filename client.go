package main

import (
	"log"
	"net"
	"os"
	"time"
)

type Client struct {
	conf         Config
	lc           LocalConfig
	mon          *Monitoring
	graphiteAddr net.TCPAddr
	lg           log.Logger
	ch           chan string
	chM          chan string
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
func (c Client) saveSliceToRetry(metrics []string) {
	/*
		If size of file is bigger, than max size we will remove lines from this file,
		and will call this function again to check result and write to the file.
		Recursion:)
	*/
	c.lg.Printf("Saving %d metrics to the retry-file", len(metrics))
	f, err := os.OpenFile(c.conf.RetryFile, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0600)
	if err != nil {
		c.lg.Println(err.Error())
	}

	for _, metric := range metrics {
		f.WriteString(metric + "\n")
		c.mon.saved++
	}
	f.Close()
	c.removeOldDataFromRetryFile()
}

func (c Client) saveChannelToRetry(ch chan string, size int) {
	/*
		If size of file is bigger, than max size we will remove lines from this file,
		and will call this function again to check result and write to the file.
		Recursion:)
	*/

	c.lg.Printf("Saving %d metrics to the retry-file from channel", size)
	f, err := os.OpenFile(c.conf.RetryFile, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0600)
	if err != nil {
		c.lg.Println(err.Error())
	}

	for i := 0; i < size; i++ {
		f.WriteString(<-ch + "\n")
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
			c.conf.RetryFile, c.lc.fileMetricSize, currentLinesInFile-c.lc.fileMetricSize)
		// We save first c.lc.fileMetricSize of metrics (newest)
		wholeFile := readMetricsFromFile(c.conf.RetryFile)[:c.lc.fileMetricSize]
		c.saveSliceToRetry(wholeFile)
	}
}

func (c Client) tryToSendToGraphite(metric string, conn net.Conn) error {
	_, err := conn.Write([]byte(metric + "\n"))
	if err != nil {
		c.lg.Println("Write to server failed:", err.Error())
		return err
	} else {
		c.mon.sent++
		return nil
	}
}

/*
	Sending data to graphite:
	1) Metrics from monitor queue
	2) Metrics from main quere
	3) Retry file
*/
func (c Client) runClient() {
	sup := Supervisor{c.conf.Supervisor}
	for ; ; time.Sleep(time.Duration(c.conf.ClientSendInterval) * time.Second) {
		var connectionFailed bool
		// Notify watchdog about aliveness of Client routine
		sup.notify()

		// Try to dial to Graphite server. If ClientSendInterval is 10 seconds - dial should be no longer than 1 second
		conn, err := net.DialTimeout("tcp", c.graphiteAddr.String(), time.Duration(c.conf.ConnectTimeout)*time.Second)
		if err != nil {
			c.lg.Println("Can not connect to graphite server: ", err.Error())
			c.saveChannelToRetry(c.chM, len(c.chM))
			c.saveChannelToRetry(c.ch, len(c.ch))
			c.removeOldDataFromRetryFile()
			continue
		} else {
			// We set dead line for connection to write. It should be the rest of we have for client interval
			err := conn.SetWriteDeadline(time.Now().Add(time.Duration(c.conf.ClientSendInterval - c.conf.ConnectTimeout - 1)*time.Second))
			if err != nil {
				c.lg.Println("Can not set deadline for connection: ", err.Error())
				connectionFailed = true
			}

			processedTotal := 0

			// We send retry file first, we have a risk to lose old data
			if !connectionFailed {
				retryFileMetrics := readMetricsFromFile(c.conf.RetryFile)
				for numOfMetricFromFile, metricFromFile := range retryFileMetrics {
					if numOfMetricFromFile + 1 < c.lc.mainBufferSize {
						err = c.tryToSendToGraphite(metricFromFile, conn)
						if err != nil {
							c.lg.Printf("Error happened in the middle of writing retry metrics. Resaving %d metrics\n", len(retryFileMetrics) - numOfMetricFromFile)
							// If we failed to write a metric to graphite - something is wrong with connection
							c.saveSliceToRetry(retryFileMetrics[numOfMetricFromFile:])
							connectionFailed = true
							break
						} else {
							c.mon.got.retry++
						}

					} else {
						c.lg.Printf("Can read only %d metrics from %s. Rest will be kept for the next run", numOfMetricFromFile + 1, c.conf.RetryFile)
						c.saveSliceToRetry(retryFileMetrics[numOfMetricFromFile:])
						break
					}
					processedTotal++
				}
			}

			// Monitoring. We read it always and we reserved space for it
			bufSize := len(c.chM)
			if !connectionFailed {
				for i := 0; i < bufSize; i++ {
					err = c.tryToSendToGraphite(<-c.chM, conn)
					if err != nil {
						c.lg.Println("Error happened in the middle of writing monitoring metrics. Saving...")
						c.saveChannelToRetry(c.chM, bufSize-i)
						connectionFailed = true
						break
					}
				}
			} else {
				c.saveChannelToRetry(c.chM, bufSize)
			}

			/*
			 Main Buffer. We read it completely but send only part which fits in mainBufferSize
			 Rests we save
			*/

			bufSize = len(c.ch)
			if !connectionFailed {
				for processedMainBuff := 0; processedMainBuff < bufSize; processedMainBuff, processedTotal = processedMainBuff + 1, processedTotal + 1 {
					if processedTotal < c.lc.mainBufferSize {
						err = c.tryToSendToGraphite(<-c.ch, conn)
						if err != nil {
							c.lg.Printf("Error happened in the middle of writing metrics. Saving %d metrics\n", bufSize - processedMainBuff)
							c.saveChannelToRetry(c.ch, bufSize - processedMainBuff)
							break
						}
					} else {
						/*
						 Save only data for the moment of run. Concurrent goroutines know no mercy
						 and they continue to write...
						*/
						c.saveChannelToRetry(c.ch, bufSize - processedMainBuff)
						break
					}
				}
			} else {
				c.saveChannelToRetry(c.chM, bufSize)
			}
		}
		conn.Close()
	}
}
