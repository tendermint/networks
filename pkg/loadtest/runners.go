package loadtest

// RunMaster will build and execute a master node for load testing and will
// block until the testing is complete or fails.
func RunMaster(configFile string) error {
	cfg, err := LoadConfig(configFile)
	if err != nil {
		return err
	}
	master := NewMasterNode(cfg)
	if err = master.Start(); err != nil {
		return err
	}
	return master.Wait()
}

// RunSlave will build and execute a slave node for load testing and will block
// until the testing is complete or fails.
func RunSlave(configFile string) error {
	cfg, err := LoadConfig(configFile)
	if err != nil {
		return err
	}
	slave := NewSlaveNode(cfg, *GetTestHarnessClientFactory(cfg.Clients.Type))
	if err = slave.Start(); err != nil {
		return err
	}
	return slave.Wait()
}
