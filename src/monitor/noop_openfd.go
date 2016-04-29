// +build !linux

package monitor

import (
	"time"

	"github.com/cloudfoundry/gosteno"
)

// LinuxFileDescriptor does nothing if it's not on linux.
type LinuxFileDescriptor struct {
}

func NewLinuxFD(time.Duration, *gosteno.Logger) *LinuxFileDescriptor {
	return &LinuxFileDescriptor{}
}

func (l *LinuxFileDescriptor) Start() {}
func (l *LinuxFileDescriptor) Stop()  {}
