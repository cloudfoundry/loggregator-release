package deaagent

import (
	"github.com/cloudfoundry/gosteno"
)

func logger() *gosteno.Logger {
	return gosteno.NewLogger("TestLogger")
}
