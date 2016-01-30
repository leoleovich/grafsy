package main

import (
	"regexp"
	"os"
	"bufio"
)

type Metric struct {
	name string
	amount int64
	value float64
	timestamp int64
}
/*
	This is a "theoretical" size of 1 metric
	Maximum size of retry file then we can calculate as
	MaxMetrics*ClientSendInterval*metricsSize*metricsSize
	Which will give you cache for 1 very bad minute
 */
const metricsSize = 50

// Check metric to match base metric regexp
func validateMetric(metric string) bool {
	match, _ := regexp.MatchString("^([-a-zA-Z0-9_]+\\.){2}[-a-zA-Z0-9_.]+(\\s)[-0-9.eE+]+(\\s)[0-9]{10}", metric)
	return match
}

// Reading metrics from file and remove file afterwords
func readMetricsFromFile(file string) []string {
	var results_list []string
	f, err := os.Open(file)
	if err != nil {
		return results_list
	}

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		results_list = append(results_list, scanner.Text())
	}
	os.Remove(file)
	f.Close()
	return results_list
}