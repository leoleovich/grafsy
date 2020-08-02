package main

import (
	"bufio"
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"time"

	"github.com/leoleovich/grafsy"
)

var version = "dev"

func main() {
	var configFile string
	var connectionTimeout int
	printVersion := false
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: %s [args] [file1 [fileN...]]\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "   Or: metrics-generator | %s [args]\n\n", os.Args[0])
		fmt.Fprintln(os.Stderr, "Reads metrics from files or STDIN and writes to grafsy LocalBind address.")
		fmt.Fprintln(os.Stderr, "If STDIN contains something, then files will be ignored")
		fmt.Fprintf(os.Stderr, "\nArgs:\n")
		flag.PrintDefaults()
	}
	flag.StringVar(&configFile, "c", "/etc/grafsy/grafsy.toml", "Path to config file.")
	flag.BoolVar(&printVersion, "v", printVersion, "Print version and exit")
	flag.IntVar(&connectionTimeout, "w", 50, "Timeout ")
	flag.Parse()
	fileNames := flag.Args()

	if printVersion {
		fmt.Printf("Version: %v\n", version)
		os.Exit(0)
	}

	var conf grafsy.Config
	err := conf.LoadConfig(configFile)
	if err != nil {
		log.Fatalln(err)
	}

	conn, err := net.DialTimeout("tcp", conf.LocalBind, time.Duration(connectionTimeout)*time.Second)
	if err != nil {
		log.Fatalf("Fail to establish connection: %v\n", err)
	}
	defer conn.Close()

	stat, err := os.Stdin.Stat()
	if err != nil {
		log.Fatalf("Error in STDIN: %v\n", err)
	}

	// If STDIN is CharDevice, then it contains something.
	// Send it to grafsy daemon
	if (stat.Mode() & os.ModeCharDevice) == 0 {
		stdin := bufio.NewReader(os.Stdin)
		connWriter := bufio.NewWriter(conn)
		stdin.WriteTo(connWriter)
		return
	}

	if len(fileNames) == 0 {
		flag.Usage()
	}

	for _, fileName := range fileNames {
		file, err := os.Open(fileName)
		if err != nil {
			log.Printf("Failed to open file %s: %v\n", fileName, err)
			continue
		}
		defer file.Close()

		r := bufio.NewReader(file)
		connWriter := bufio.NewWriter(conn)
		r.WriteTo(connWriter)
	}
}
