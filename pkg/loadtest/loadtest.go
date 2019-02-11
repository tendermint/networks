package loadtest

import (
	log "github.com/sirupsen/logrus"
)

// Execute is the primary entrypoint to executing a load test. On failure, it
// returns a relevant error message.
func Execute(isMaster bool, configFile string) error {
	logger := log.WithField("mod", "loadtest")
	logger.WithField("configFile", configFile).Debugln("Loading configuration file")
	cfg, err := LoadConfig(configFile)
	if err != nil {
		return err
	}
	return ExecuteWithConfig(logger, isMaster, cfg)
}

// ExecuteWithConfig allows us to execute a load test with the given lower-level
// configuration.
func ExecuteWithConfig(logger *log.Entry, isMaster bool, config *Config) error {
	logger.WithField("config", config).Debugln("Using configuration")
	if isMaster {
		return runMaster(logger, config)
	}
	return runSlave(logger, config)
}
