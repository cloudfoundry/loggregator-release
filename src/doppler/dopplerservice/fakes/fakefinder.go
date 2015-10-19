// This file was generated by counterfeiter
package fakes

import (
	"doppler/dopplerservice"
	"sync"
)

type FakeFinder struct {
	StartStub             func()
	startMutex            sync.RWMutex
	startArgsForCall      []struct{}
	StopStub              func()
	stopMutex             sync.RWMutex
	stopArgsForCall       []struct{}
	AllServersStub        func() map[string]string
	allServersMutex       sync.RWMutex
	allServersArgsForCall []struct{}
	allServersReturns     struct {
		result1 map[string]string
	}
	PreferredServersStub        func() map[string]string
	preferredServersMutex       sync.RWMutex
	preferredServersArgsForCall []struct{}
	preferredServersReturns     struct {
		result1 map[string]string
	}
}

func (fake *FakeFinder) Start() {
	fake.startMutex.Lock()
	fake.startArgsForCall = append(fake.startArgsForCall, struct{}{})
	fake.startMutex.Unlock()
	if fake.StartStub != nil {
		fake.StartStub()
	}
}

func (fake *FakeFinder) StartCallCount() int {
	fake.startMutex.RLock()
	defer fake.startMutex.RUnlock()
	return len(fake.startArgsForCall)
}

func (fake *FakeFinder) Stop() {
	fake.stopMutex.Lock()
	fake.stopArgsForCall = append(fake.stopArgsForCall, struct{}{})
	fake.stopMutex.Unlock()
	if fake.StopStub != nil {
		fake.StopStub()
	}
}

func (fake *FakeFinder) StopCallCount() int {
	fake.stopMutex.RLock()
	defer fake.stopMutex.RUnlock()
	return len(fake.stopArgsForCall)
}

func (fake *FakeFinder) AllServers() map[string]string {
	fake.allServersMutex.Lock()
	fake.allServersArgsForCall = append(fake.allServersArgsForCall, struct{}{})
	fake.allServersMutex.Unlock()
	if fake.AllServersStub != nil {
		return fake.AllServersStub()
	} else {
		return fake.allServersReturns.result1
	}
}

func (fake *FakeFinder) AllServersCallCount() int {
	fake.allServersMutex.RLock()
	defer fake.allServersMutex.RUnlock()
	return len(fake.allServersArgsForCall)
}

func (fake *FakeFinder) AllServersReturns(result1 map[string]string) {
	fake.AllServersStub = nil
	fake.allServersReturns = struct {
		result1 map[string]string
	}{result1}
}

func (fake *FakeFinder) PreferredServers() map[string]string {
	fake.preferredServersMutex.Lock()
	fake.preferredServersArgsForCall = append(fake.preferredServersArgsForCall, struct{}{})
	fake.preferredServersMutex.Unlock()
	if fake.PreferredServersStub != nil {
		return fake.PreferredServersStub()
	} else {
		return fake.preferredServersReturns.result1
	}
}

func (fake *FakeFinder) PreferredServersCallCount() int {
	fake.preferredServersMutex.RLock()
	defer fake.preferredServersMutex.RUnlock()
	return len(fake.preferredServersArgsForCall)
}

func (fake *FakeFinder) PreferredServersReturns(result1 map[string]string) {
	fake.PreferredServersStub = nil
	fake.preferredServersReturns = struct {
		result1 map[string]string
	}{result1}
}

var _ dopplerservice.Finder = new(FakeFinder)
