// +build !linux

package monitor

import "time"

// LinuxFileDescriptor does nothing if it's not on linux.
type LinuxFileDescriptor struct {
}

func NewLinuxFD(time.Duration) *LinuxFileDescriptor {
	return &LinuxFileDescriptor{}
}

func (l *LinuxFileDescriptor) Start() {}
func (l *LinuxFileDescriptor) Stop()  {}
