package component

import "plumbing"

//go:generate hel
type DopplerIngestorServer interface {
	plumbing.DopplerIngestorServer
}
