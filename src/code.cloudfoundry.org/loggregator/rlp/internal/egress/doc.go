package egress

import (
	v2 "plumbing/v2"
)

// ReceiverServer is used to generate a mock for testing the egress server.
type ReceiverServer interface {
	v2.Egress_ReceiverServer
}
