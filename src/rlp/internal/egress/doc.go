package egress

import (
	v2 "plumbing/v2"
)

type ReceiverServer interface {
	v2.Egress_ReceiverServer
}
