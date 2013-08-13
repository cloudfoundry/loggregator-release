package groupedchannels

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestAddAndGet(t *testing.T) {
	groupedChannels := NewGroupedChannels()
	channel1 := make(chan []byte)
	groupedChannels.Add(channel1, "3")

	assert.Equal(t, len(groupedChannels.Get("3")), 1)
	assert.Equal(t, groupedChannels.Get("3")[0], channel1)

	channel2 := make(chan []byte)
	groupedChannels.Add(channel2, "3")

	assert.Equal(t, len(groupedChannels.Get("3")), 2)
	assert.Equal(t, groupedChannels.Get("3")[0], channel1)
	assert.Equal(t, groupedChannels.Get("3")[1], channel2)
}

func TestNestedAddAndGet(t *testing.T) {
	groupedChannels := NewGroupedChannels()
	channel1 := make(chan []byte)
	groupedChannels.Add(channel1, "3", "a")

	assert.Equal(t, len(groupedChannels.Get("3", "a")), 1)
	assert.Equal(t, groupedChannels.Get("3", "a")[0], channel1)

	assert.Equal(t, len(groupedChannels.Get("3")), 0)

	channel2 := make(chan []byte)
	groupedChannels.Add(channel2, "3", "a")

	assert.Equal(t, len(groupedChannels.Get("3", "a")), 2)
	assert.Equal(t, groupedChannels.Get("3", "a")[0], channel1)
	assert.Equal(t, groupedChannels.Get("3", "a")[1], channel2)
}

func TestAddAndGetForEmptySubgroup(t *testing.T) {
	groupedChannels := NewGroupedChannels()
	channel1 := make(chan []byte)
	channel2 := make(chan []byte)
	groupedChannels.Add(channel1, "3")
	groupedChannels.Add(channel2, "3", "")

	assert.Equal(t, len(groupedChannels.Get("3", "")), 2)
	assert.Equal(t, groupedChannels.Get("3", "")[0], channel1)
	assert.Equal(t, groupedChannels.Get("3", "")[1], channel2)

	assert.Equal(t, len(groupedChannels.Get("3")), 2)
	assert.Equal(t, groupedChannels.Get("3")[0], channel1)
	assert.Equal(t, groupedChannels.Get("3")[1], channel2)
}

func TestDelete(t *testing.T) {
	groupedChannels := NewGroupedChannels()
	channel1 := make(chan []byte)
	channel2 := make(chan []byte)

	groupedChannels.Add(channel1, "3")
	groupedChannels.Add(channel2, "3")

	groupedChannels.Delete(channel1)

	assert.Equal(t, len(groupedChannels.Get("3")), 1)
	assert.Equal(t, groupedChannels.Get("3")[0], channel2)
}

func TestNestedDelete(t *testing.T) {
	groupedChannels := NewGroupedChannels()
	channel1 := make(chan []byte)
	channel2 := make(chan []byte)

	groupedChannels.Add(channel1, "3", "a")
	groupedChannels.Add(channel2, "3")

	groupedChannels.Delete(channel1)

	assert.Equal(t, len(groupedChannels.Get("3", "a")), 0)
	assert.Equal(t, groupedChannels.Get("3")[0], channel2)
}

func TestTotalNumberOfChannels(t *testing.T) {
	groupedChannels := NewGroupedChannels()
	channel1 := make(chan []byte)
	channel2 := make(chan []byte)

	assert.Equal(t, groupedChannels.NumberOfChannels(), 0)
	groupedChannels.Add(channel1, "3", "a")
	groupedChannels.Add(channel2, "3", "a")
	assert.Equal(t, groupedChannels.NumberOfChannels(), 2)
	groupedChannels.Add(channel2, "3")
	assert.Equal(t, groupedChannels.NumberOfChannels(), 3)

	groupedChannels.Delete(channel1)
	assert.Equal(t, groupedChannels.NumberOfChannels(), 2)
}
