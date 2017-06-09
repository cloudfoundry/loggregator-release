package component

import "code.cloudfoundry.org/loggregator/plumbing"

//go:generate hel
type DopplerIngestorServer interface {
	plumbing.DopplerIngestorServer
}
