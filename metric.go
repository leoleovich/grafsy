package main

import (
	"regexp"
)

type Metric struct {
	name string
	amount int64
	value float64
	timestamp int64
}

// Check metric to match base metric regexp
func validateMetric(metric string) bool {
	match, _ := regexp.MatchString("^([-a-zA-Z0-9_]+\\.){2}[-a-zA-Z0-9_.]+(\\s)[-0-9.eE+]+(\\s)[0-9]{10}", metric)
	return match
}