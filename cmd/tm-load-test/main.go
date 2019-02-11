package main

import (
	"flag"
	"fmt"
	"os"

	log "github.com/sirupsen/logrus"
	"github.com/tendermint/networks/pkg/loadtest"
)

func main() {
	var (
		isMaster   = flag.Bool("master", false, "start this process in MASTER mode")
		isSlave    = flag.Bool("slave", false, "start this process in SLAVE mode")
		configFile = flag.String("c", "load-test.toml", "the path to the configuration file for a load test")
		isVerbose  = flag.Bool("v", false, "increase logging verbosity to DEBUG level")
	)
	flag.Usage = func() {
		fmt.Println(`Tendermint load testing utility

tm-load-test is a tool for distributed load testing on Tendermint networks,
assuming that your Tendermint network currently runs the "kvstore" proxy app.

Usage:
  tm-load-test -c load-test.toml -master   # Run a master
  tm-load-test -c load-test.toml -slave    # Run a slave

Flags:`)
		flag.PrintDefaults()
		fmt.Println("")
	}
	flag.Parse()

	if (!*isMaster && !*isSlave) || (*isMaster && *isSlave) {
		fmt.Println("Either -master or -slave is expected on the command line to explicitly specify which mode to use.")
		os.Exit(1)
	}

	if *isVerbose {
		log.SetLevel(log.DebugLevel)
	}
	logger := log.WithField("mod", "main")

	if *isMaster {
		logger.Infoln("Starting in MASTER mode")
	} else {
		logger.Infoln("Starting in SLAVE mode")
	}

	if err := loadtest.Execute(*isMaster, *configFile); err != nil {
		logger.WithFields(log.Fields{
			"err": err,
		}).Errorln("Load test execution failed")
		if ltErr, ok := err.(loadtest.Error); ok {
			os.Exit(ltErr.ExitCode)
		} else {
			os.Exit(1)
		}
	}
}
