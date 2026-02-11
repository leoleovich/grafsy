package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/leoleovich/grafsy"
)

var version = "dev"

func main() {
	var configFile string
	printVersion := false
	flag.StringVar(&configFile, "c", grafsy.ConfigPath, "Path to config file.")
	flag.BoolVar(&printVersion, "v", printVersion, "Print version and exit")
	flag.Parse()

	if printVersion {
		fmt.Printf("Version: %v\n", version)
		os.Exit(0)
	}
	var conf grafsy.Config
	err := conf.LoadConfig(configFile)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	lc, err := conf.GenerateLocalConfig()
	if err != nil {
		fmt.Println(err)
		os.Exit(2)
	}

	mon := &grafsy.Monitoring{
		Conf: &conf,
		Lc:   lc,
	}

	cli := grafsy.Client{
		Conf: &conf,
		Lc:   lc,
		Mon:  mon,
	}

	srv := grafsy.Server{
		Conf: &conf,
		Lc:   lc,
		Mon:  mon,
	}

	go mon.Run()
	go srv.Run()
	go cli.Run()

	select {}
}
