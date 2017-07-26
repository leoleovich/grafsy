package grafsy

import (
	"net"
	"os"
	"strings"
	"time"
)

// The main client data
type Client struct {
	// User config.
	Conf *Config

	// Local config.
	Lc *localConfig

	// Pointer to Monitoring structure.
	Mon *Monitoring
}

// Save []string to file.
func (c Client) saveSliceToRetry(metrics []string) error {
	/*
		If size of file is bigger, than max size we will remove lines from this file,
		and will call this function again to check result and write to the file.
		Recursion:)
	*/

	c.Lc.lg.Printf("Saving %d metrics to the retry-file", len(metrics))
	f, err := os.OpenFile(c.Conf.RetryFile, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0600)
	if err != nil {
		c.Lc.lg.Println(err)
		c.Mon.dropped += len(metrics)
		return err
	}
	defer f.Close()

	for _, metric := range metrics {
		_, err = f.WriteString(metric + "\n")
		if err != nil {
			c.Mon.dropped++
			c.Lc.lg.Println(err)
		} else {
			c.Mon.saved++
		}
	}
	return c.removeOldDataFromRetryFile()
}

// Save part of entire content of channel to file.
func (c Client) saveChannelToRetry(ch chan string, size int) {
	/*
		If size of file is bigger, than max size we will remove lines from this file,
		and will call this function again to check result and write to the file.
		Recursion:)
	*/

	c.Lc.lg.Printf("Saving %d metrics to the retry-file from channel", size)
	f, err := os.OpenFile(c.Conf.RetryFile, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0600)
	if err != nil {
		c.Lc.lg.Println(err.Error())
	}

	for i := 0; i < size; i++ {
		_, err = f.WriteString(<-ch + "\n")
		if err == nil {
			c.Mon.saved++
		} else {
			c.Mon.dropped++
			c.Lc.lg.Println(err.Error())
		}
	}
	f.Close()
	c.removeOldDataFromRetryFile()
}

// Cleaning up retry-file.
// Entire file is sorted to have newest metrics at the beginning.
func (c Client) removeOldDataFromRetryFile() error {

	currentLinesInFile := getSizeInLinesFromFile(c.Conf.RetryFile)
	if currentLinesInFile > c.Lc.fileMetricSize {
		c.Lc.lg.Printf("I can not save to %s more, than %d. I will have to drop the rest (%d)",
			c.Conf.RetryFile, c.Lc.fileMetricSize, currentLinesInFile-c.Lc.fileMetricSize)
		// We save first c.Lc.fileMetricSize of metrics (newest)
		wholeFile, _ := readMetricsFromFile(c.Conf.RetryFile)
		return c.saveSliceToRetry(wholeFile[:c.Lc.fileMetricSize])
	}
	return nil
}

// Attempt to send metric to graphite server via connection
func (c *Client) tryToSendToGraphite(metric string, conn net.Conn) error {
	// If at any point "HOSTNAME" was used instead of real hostname - replace it
	metric = strings.Replace(metric, "HOSTNAME", c.Lc.hostname, -1)

	_, err := conn.Write([]byte(metric + "\n"))
	if err != nil {
		c.Lc.lg.Println("Write to server failed:", err.Error())
		return err
	} else {
		c.Mon.sent++
		return nil
	}
}

//Run a client, which sends data to Graphite in the order:
// 1) Metrics from monitor queue
// 2) Metrics from main quere
// 3) Retry file
// Should be run in separate goroutine.
func (c Client) Run() {
	sup := supervisor(c.Conf.Supervisor)
	for ; ; time.Sleep(time.Duration(c.Conf.ClientSendInterval) * time.Second) {
		var connectionFailed bool
		// Notify watchdog about aliveness of Client routine
		sup.notify()

		// Try to dial to Graphite server. If ClientSendInterval is 10 seconds - dial should be no longer than 1 second
		conn, err := net.DialTimeout("tcp", c.Lc.graphiteAddr.String(), time.Duration(c.Conf.ConnectTimeout)*time.Second)
		if err != nil {
			c.Lc.lg.Println("Can not connect to graphite server: ", err.Error())
			c.saveChannelToRetry(c.Lc.monitoringChannel, len(c.Lc.monitoringChannel))
			c.saveChannelToRetry(c.Lc.mainChannel, len(c.Lc.mainChannel))
			c.removeOldDataFromRetryFile()
			continue
		} else {
			// We set dead line for connection to write. It should be the rest of we have for client interval
			err := conn.SetWriteDeadline(time.Now().Add(time.Duration(c.Conf.ClientSendInterval-c.Conf.ConnectTimeout-1) * time.Second))
			if err != nil {
				c.Lc.lg.Println("Can not set deadline for connection: ", err.Error())
				connectionFailed = true
			}

			processedTotal := 0

			// We send retry file first, we have a risk to lose old data
			if !connectionFailed {
				retryFileMetrics, _ := readMetricsFromFile(c.Conf.RetryFile)
				for numOfMetricFromFile, metricFromFile := range retryFileMetrics {
					if numOfMetricFromFile+1 < c.Lc.mainBufferSize {
						err = c.tryToSendToGraphite(metricFromFile, conn)
						if err != nil {
							c.Lc.lg.Printf("Error happened in the middle of writing retry metrics. Resaving %d metrics\n", len(retryFileMetrics)-numOfMetricFromFile)
							// If we failed to write a metric to graphite - something is wrong with connection
							c.saveSliceToRetry(retryFileMetrics[numOfMetricFromFile:])
							connectionFailed = true
							break
						} else {
							c.Mon.got.retry++
						}

					} else {
						c.Lc.lg.Printf("Can read only %d metrics from %s. Rest will be kept for the next run", numOfMetricFromFile+1, c.Conf.RetryFile)
						c.saveSliceToRetry(retryFileMetrics[numOfMetricFromFile:])
						break
					}
					processedTotal++
				}
			}

			// Monitoring. We read it always and we reserved space for it
			bufSize := len(c.Lc.monitoringChannel)
			if !connectionFailed {
				for i := 0; i < bufSize; i++ {
					err = c.tryToSendToGraphite(<-c.Lc.monitoringChannel, conn)
					if err != nil {
						c.Lc.lg.Println("Error happened in the middle of writing monitoring metrics. Saving...")
						c.saveChannelToRetry(c.Lc.monitoringChannel, bufSize-i)
						connectionFailed = true
						break
					}
				}
			} else {
				c.saveChannelToRetry(c.Lc.monitoringChannel, bufSize)
			}

			/*
			 Main Buffer. We read it completely but send only part which fits in mainBufferSize
			 Rests we save
			*/

			bufSize = len(c.Lc.mainChannel)
			if !connectionFailed {
				for processedMainBuff := 0; processedMainBuff < bufSize; processedMainBuff, processedTotal = processedMainBuff+1, processedTotal+1 {
					if processedTotal < c.Lc.mainBufferSize {
						err = c.tryToSendToGraphite(<-c.Lc.mainChannel, conn)
						if err != nil {
							c.Lc.lg.Printf("Error happened in the middle of writing metrics. Saving %d metrics\n", bufSize-processedMainBuff)
							c.saveChannelToRetry(c.Lc.mainChannel, bufSize-processedMainBuff)
							break
						}
					} else {
						/*
						 Save only data for the moment of run. Concurrent goroutines know no mercy
						 and they continue to write...
						*/
						c.saveChannelToRetry(c.Lc.mainChannel, bufSize-processedMainBuff)
						break
					}
				}
			} else {
				c.saveChannelToRetry(c.Lc.monitoringChannel, bufSize)
			}
		}
		conn.Close()
	}
}
