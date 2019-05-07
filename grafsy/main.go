package main

import (
	"flag"
	"fmt"
	"os"
	"sync"

	"github.com/innogames/grafsy"
)

func main() {
	var configFile string
	flag.StringVar(&configFile, "c", "/etc/grafsy/grafsy.toml", "Path to config file.")
	flag.Parse()

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

	var wg sync.WaitGroup
	go mon.Run()
	go srv.Run()
	go cli.Run()

	wg.Add(3)
	wg.Wait()
}
