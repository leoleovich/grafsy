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

// Check metric to match base metric regexp
func validateMetric(metric string, reg string) bool {
	match, _ := regexp.MatchString(reg, metric)
	return match
}

// Reading metrics from file and remove file afterwords
func readMetricsFromFile(file string) []string {
	var results_list []string
	f, err := os.Open(file)

	if err != nil {
		f.Close()
		return results_list
	}

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		results_list = append(results_list, scanner.Text())
	}
	f.Close()
	// Go will close file automatically?
	os.Remove(file)

	return results_list
}

// Reading metrics from file and remove file afterwords
func getSizeInLinesFromFile(file string) int {
	f, err := os.Open(file)
	defer f.Close()

	res := 0
	if err != nil {
		return res
	}

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		res++
	}
	return res
}