package stage

import "github.com/cloudfoundry/gosteno"

type Stage interface {
	Run(<-chan error, *gosteno.Logger)
	Shutdown()
}
