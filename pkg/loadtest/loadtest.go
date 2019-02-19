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
	master.Wait()
	return master.GetShutdownError()
}
