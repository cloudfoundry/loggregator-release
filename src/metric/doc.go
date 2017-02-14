package metric

import (
	v2 "plumbing/v2"
)

//go:generate hel

type IngressServer interface {
	v2.IngressServer
}
