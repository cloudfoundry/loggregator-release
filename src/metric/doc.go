package metric

import (
	v2 "plumbing/v2"
)

//go:generate hel

type MetronIngressServer interface {
	v2.MetronIngressServer
}
