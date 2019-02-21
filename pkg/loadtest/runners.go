package loadtest

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
	master := NewMasterNode(cfg)
	if err := master.Start(); err != nil {
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
	return RunSlaveWithConfig(cfg)
}

// RunSlaveWithConfig runs a slave node with the given configuration and blocks
// until the testing is complete or it fails.
func RunSlaveWithConfig(cfg *Config) error {
	slave := NewSlaveNode(cfg, *GetTestHarnessClientFactory(cfg.Clients.Type))
	if err := slave.Start(); err != nil {
		return err
	}
	return slave.Wait()
}
