package loadtest

// RunMaster will create the master node server and run it (blocking).
func RunMaster(configFile string) error {
	config, err := LoadConfig(configFile)
	if err != nil {
		return err
	}
	n := NewMasterNode(config)
	return n.RunAndWait()
}

// RunSlave will create the slave node server and run it (blocking).
func RunSlave(configFile string) error {
	config, err := LoadConfig(configFile)
	if err != nil {
		return err
	}
	n := NewSlaveNode(config)
	return n.RunAndWait()
}
