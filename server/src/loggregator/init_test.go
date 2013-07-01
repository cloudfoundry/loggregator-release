package loggregator

import (
	"github.com/cloudfoundry/gosteno"
)

func init() {
	logger = gosteno.NewLogger("TestLogger")
}

