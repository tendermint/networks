package loadtest_test

import (
	"os"

	"github.com/sirupsen/logrus"
)

func init() {
	if os.Getenv("LOG_LEVEL") == "DEBUG" {
		logrus.SetLevel(logrus.DebugLevel)
	}
}
