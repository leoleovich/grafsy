package grafsy

import (
	"log"
	"net"
	"os"
	"path"
	"strings"
	"sync"
	"time"
)

// Client is a class which sends metrics to the carbon receivers
type Client struct {
	// User config.
	Conf *Config

	// Local config.
	Lc *LocalConfig

	// Pointer to Monitoring structure.
	Mon *Monitoring

	// Monitoring channels per carbon
	monChannels map[string]chan string

	// Main channel per carbon
	mainChannels map[string]chan string
}

var chanLock sync.Mutex

// Create a directory for retry files
func (c Client) createRetryDir() error {
	err := os.MkdirAll(c.Conf.RetryDir, 0750)
	return err
}

// Save []string to file.
func (c Client) saveSliceToRetry(metrics []string, carbonAddr string) error {
	// If size of file is bigger, than max size we will remove lines from this file,
	// and will call this function again to check result and write to the file.
	// Recursion:)

	c.Lc.lg.Printf("Resaving %d metrics back to the retry-file", len(metrics))

	retFile := path.Join(c.Conf.RetryDir, carbonAddr)
	f, err := os.OpenFile(retFile, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0600)
	if err != nil {
		c.Lc.lg.Println(err)
		c.Mon.Increase(&c.Mon.clientStat[carbonAddr].dropped, len(metrics))
		return err
	}
	defer f.Close()

	dropped := 0
	for _, metric := range metrics {
		_, err = f.WriteString(metric + "\n")
		if err != nil {
			c.Lc.lg.Println(err)
			dropped++
		}
	}
	if dropped > 0 {
		c.Mon.Increase(&c.Mon.clientStat[carbonAddr].dropped, dropped)
	}
	return c.removeOldDataFromRetryFile(carbonAddr)
}

// Save part of entire content of channel to file.
func (c Client) saveChannelToRetry(ch chan string, size int, carbonAddr string) {
	// If size of file is bigger, than max size we will remove lines from this file,
	// and will call this function again to check result and write to the file.
	// Recursion:)

	// We save all metrics from channels on the program ending
	// In this case on the size=0 the whole channel is saved
	if size == 0 {
		size = len(ch)
	}

	c.Lc.lg.Printf("Saving %d metrics from channel to the retry-file", size)

	retFile := path.Join(c.Conf.RetryDir, carbonAddr)
	f, err := os.OpenFile(retFile, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0600)
	if err != nil {
		c.Lc.lg.Println(err)
	}
	defer f.Close()

	dropped, saved := 0, 0
	for i := 0; i < size; i++ {
		_, err = f.WriteString(<-ch + "\n")
		if err != nil {
			dropped++
			c.Lc.lg.Println(err)
		} else {
			saved++
		}
	}
	if dropped > 0 {
		c.Mon.Increase(&c.Mon.clientStat[carbonAddr].dropped, dropped)
	}
	if saved > 0 {
		c.Mon.Increase(&c.Mon.clientStat[carbonAddr].saved, saved)
	}
	c.removeOldDataFromRetryFile(carbonAddr)
}

// Cleaning up retry-file.
// Entire file is sorted to have newest metrics at the beginning.
func (c Client) removeOldDataFromRetryFile(carbonAddr string) error {
	retFile := path.Join(c.Conf.RetryDir, carbonAddr)
	currentLinesInFile := getSizeInLinesFromFile(retFile)
	if currentLinesInFile > c.Lc.fileMetricSize {
		c.Lc.lg.Printf("I can not save to %s more, than %d. I will have to drop the rest (%d)",
			retFile, c.Lc.fileMetricSize, currentLinesInFile-c.Lc.fileMetricSize)
		// We save first c.Lc.fileMetricSize of metrics (newest)
		wholeFile, _ := readMetricsFromFile(retFile)
		return c.saveSliceToRetry(wholeFile[:c.Lc.fileMetricSize], carbonAddr)
	}
	return nil
}

// Attempt to send metric to graphite server via connection
func (c *Client) tryToSendToGraphite(metric string, carbonAddr string, conn net.Conn) error {
	// If at any point "HOSTNAME" was used instead of real hostname - replace it
	metric = strings.ReplaceAll(metric, "HOSTNAME", c.Lc.hostname)

	_, err := conn.Write([]byte(metric + "\n"))
	if err != nil {
		c.Lc.lg.Println("Write to server failed:", err)
		return err
	}
	c.Mon.Increase(&c.Mon.clientStat[carbonAddr].sent, 1)
	return nil
}

// Run go routine per carbon server to:
//  1. Send data from retryFile to a carbon
//  2. Send metrics from monitoring channel to a carbon
//  3. Send metrics from the main channel to carbon
//
// And save everything to the retryFile on any error
func (c Client) runBackend(carbonAddr string) {
	retFile := path.Join(c.Conf.RetryDir, carbonAddr)
	chanLock.Lock()
	monChannel := c.monChannels[carbonAddr]
	mainChannel := c.mainChannels[carbonAddr]
	chanLock.Unlock()
	// TODO: think about graceful shutdown with flush channels

	for ; ; time.Sleep(time.Duration(c.Conf.ClientSendInterval) * time.Second) {
		var connectionFailed bool

		// Try to dial to Graphite server. If ClientSendInterval is 10 seconds - dial should be no longer than 1 second
		conn, err := net.DialTimeout("tcp", carbonAddr, time.Duration(c.Conf.ConnectTimeout)*time.Second)
		if err != nil {
			c.Lc.lg.Println("Can not connect to graphite server:", err)
			c.saveChannelToRetry(monChannel, len(monChannel), carbonAddr)
			c.saveChannelToRetry(mainChannel, len(mainChannel), carbonAddr)
			c.removeOldDataFromRetryFile(carbonAddr)
			continue
		}

		// We set dead line for connection to write. It should be the rest of we have for client interval
		err = conn.SetWriteDeadline(time.Now().Add(time.Duration(c.Conf.ClientSendInterval-c.Conf.ConnectTimeout-1) * time.Second))
		if err != nil {
			c.Lc.lg.Println("Can not set deadline for connection:", err)
			connectionFailed = true
		}

		// We send retry file first, we have a risk to lose old data
		// Metrics from retry file are counted as extra metrics per second to have a chance to send them
		// Otherwise we would only save new incomming metrics and continuously lose part of buffer
		if !connectionFailed {
			retryFileMetrics, _ := readMetricsFromFile(retFile)
			for numOfMetricFromFile, metricFromFile := range retryFileMetrics {
				if numOfMetricFromFile < c.Lc.mainBufferSize {
					err = c.tryToSendToGraphite(metricFromFile, carbonAddr, conn)
					if err != nil {
						c.Lc.lg.Printf("Error happened in the middle of writing retry metrics. Resaving %d metrics\n", len(retryFileMetrics[numOfMetricFromFile:]))
						// If we failed to write a metric to graphite - something is wrong with connection
						c.saveSliceToRetry(retryFileMetrics[numOfMetricFromFile:], carbonAddr)
						connectionFailed = true
						break
					} else {
						c.Mon.Increase(&c.Mon.clientStat[carbonAddr].fromRetry, 1)
					}

				} else {
					c.Lc.lg.Printf("Can read only %d metrics from %s. Rest %d will be kept for the next run", numOfMetricFromFile, retFile, len(retryFileMetrics[numOfMetricFromFile:]))
					c.saveSliceToRetry(retryFileMetrics[numOfMetricFromFile:], carbonAddr)
					break
				}
			}
		}

		// Monitoring. We read it always and we reserved space for it
		bufSize := len(monChannel)
		if !connectionFailed {
			for i := 0; i < bufSize; i++ {
				err = c.tryToSendToGraphite(<-monChannel, carbonAddr, conn)
				if err != nil {
					c.Lc.lg.Println("Error happened in the middle of writing monitoring metrics. Saving...")
					c.saveChannelToRetry(monChannel, bufSize-i, carbonAddr)
					connectionFailed = true
					break
				}
			}
		} else {
			c.saveChannelToRetry(monChannel, bufSize, carbonAddr)
		}

		//  Main Buffer. We read it completely but send only part which fits in mainBufferSize
		//  Rests we save

		bufSize = len(mainChannel)

		if !connectionFailed {
			for processedMainBuff := 0; processedMainBuff < bufSize; processedMainBuff++ {
				metric := <-mainChannel

				err = c.tryToSendToGraphite(metric, carbonAddr, conn)
				if err != nil {
					c.Lc.lg.Printf("Error happened in the middle of writing metrics. Saving %d metrics\n", bufSize-processedMainBuff)
					c.saveChannelToRetry(mainChannel, bufSize-processedMainBuff, carbonAddr)
					break
				}
			}
		} else {
			c.saveChannelToRetry(mainChannel, bufSize, carbonAddr)
		}
		conn.Close()
	}
}

// Run a client, which:
// 1) Make monitoring and main channels per carbon server
// 2) Launchs go routine per carbon server
// 3) Copy metrics from monitoring and main channels to the carbon server specific
// Should be run in separate goroutine.
func (c Client) Run() {
	err := c.createRetryDir()
	if err != nil {
		log.Fatal(err)
	}
	c.mainChannels = make(map[string]chan string)
	c.monChannels = make(map[string]chan string)

	for _, carbonAddr := range c.Conf.CarbonAddrs {
		chanLock.Lock()
		c.mainChannels[carbonAddr] = make(chan string, cap(c.Lc.mainChannel))
		c.monChannels[carbonAddr] = make(chan string, cap(c.Lc.monitoringChannel))
		chanLock.Unlock()
		go c.runBackend(carbonAddr)
	}

	sup := supervisor(c.Conf.Supervisor)
	for ; ; time.Sleep(time.Second) {
		// Notify watchdog about aliveness of Client routine
		sup.notify()

		// write metrics from monitoring and main channels to the server specific channels
		for i := 0; i < len(c.Lc.mainChannel); i++ {
			metric := <-c.Lc.mainChannel
			for _, carbonAddr := range c.Conf.CarbonAddrs {
				select {
				case c.mainChannels[carbonAddr] <- metric:
				default:
					c.Mon.Increase(&c.Mon.clientStat[carbonAddr].dropped, 1)
				}
			}
		}

		for i := 0; i < len(c.Lc.monitoringChannel); i++ {
			metric := <-c.Lc.monitoringChannel
			for _, carbonAddr := range c.Conf.CarbonAddrs {
				select {
				case c.monChannels[carbonAddr] <- metric:
				default:
					c.Mon.Increase(&c.Mon.clientStat[carbonAddr].dropped, 1)
				}
			}
		}
	}
}
