package loadtest

import (
	"os"
	"os/signal"
	"syscall"

	"github.com/tendermint/networks/pkg/loadtest/messages"
)

// RunMaster will build and execute a master node for load testing and will
// block until the testing is complete or fails.
func RunMaster(configFile string) error {
	cfg, err := LoadConfig(configFile)
	if err != nil {
		return err
	}
	return RunMasterWithConfig(cfg)
}

// RunMasterWithConfig runs a master node with the given configuration and
// blocks until the testing is complete or it fails.
func RunMasterWithConfig(cfg *Config) error {
	probe := NewStandardProbe()
	mpid, ctx, err := NewMaster(cfg, probe)
	if err != nil {
		return err
	}
	sigc := make(chan os.Signal, 1)
	signal.Notify(sigc, syscall.SIGINT, syscall.SIGTERM, os.Interrupt)
	go func() {
		<-sigc
		ctx.Send(mpid, &messages.Kill{})
	}()
	// wait for the master node to terminate
	return probe.Wait()
}

// RunSlave will build and execute a slave node for load testing and will block
// until the testing is complete or fails.
func RunSlave(configFile string) error {
	cfg, err := LoadConfig(configFile)
	if err != nil {
		return err
	}
	return RunSlaveWithConfig(cfg)
}

// RunSlaveWithConfig runs a slave node with the given configuration and blocks
// until the testing is complete or it fails.
func RunSlaveWithConfig(cfg *Config) error {
	probe := NewStandardProbe()
	spid, ctx, err := NewSlave(cfg, probe)
	if err != nil {
		return err
	}
	sigc := make(chan os.Signal, 1)
	signal.Notify(sigc, syscall.SIGINT, syscall.SIGTERM, os.Interrupt)
	go func() {
		<-sigc
		ctx.Send(spid, &messages.Kill{})
	}()
	return probe.Wait()
}
