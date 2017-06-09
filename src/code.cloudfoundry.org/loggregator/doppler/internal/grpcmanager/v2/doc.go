package v2

import plumbing "code.cloudfoundry.org/loggregator/plumbing/v2"

//go:generate hel
type BatcherSenderServer interface {
	plumbing.DopplerIngress_BatchSenderServer
}
