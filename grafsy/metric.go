package grafsy

import (
	"bufio"
	"os"
)

// The main content of metric in format <name> <value> <timestamp>
// Name is not in the structure because it is a key of related map
type metricData struct {
	value  float64
	amount int64
}

// Reading metrics from file and remove file afterwords
// Return the error only if problems with file (open, close)
// Remove file only if we are able to read it
func readMetricsFromFile(file string) ([]string, error) {
	var results_list []string
	f, err := os.OpenFile(file, os.O_RDONLY, os.ModePerm)
	if err != nil {
		return results_list, err
	}
	// Think about Truncate
	defer os.Remove(file)

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		results_list = append(results_list, scanner.Text())
	}

	// It should first call Close and only then defer with removing of file
	return results_list, f.Close()
}

// Get amount of lines of file
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
