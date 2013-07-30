package cfsink

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestAddAndGet(t *testing.T) {
	groupedChannels := newGroupedChannels()
	channel1 := make(chan []byte)
	groupedChannels.add(channel1, "3")

	assert.Equal(t, len(groupedChannels.get("3")), 1)
	assert.Equal(t, groupedChannels.get("3")[0], channel1)

	channel2 := make(chan []byte)
	groupedChannels.add(channel2, "3")

	assert.Equal(t, len(groupedChannels.get("3")), 2)
	assert.Equal(t, groupedChannels.get("3")[0], channel1)
	assert.Equal(t, groupedChannels.get("3")[1], channel2)
}

func TestNestedAddAndGet(t *testing.T) {
	groupedChannels := newGroupedChannels()
	channel1 := make(chan []byte)
	groupedChannels.add(channel1, "3", "a")

	assert.Equal(t, len(groupedChannels.get("3", "a")), 1)
	assert.Equal(t, groupedChannels.get("3", "a")[0], channel1)

	assert.Equal(t, len(groupedChannels.get("3")), 0)

	channel2 := make(chan []byte)
	groupedChannels.add(channel2, "3", "a")

	assert.Equal(t, len(groupedChannels.get("3", "a")), 2)
	assert.Equal(t, groupedChannels.get("3", "a")[0], channel1)
	assert.Equal(t, groupedChannels.get("3", "a")[1], channel2)
}

func TestAddAndGetForEmptySubgroup(t *testing.T) {
	groupedChannels := newGroupedChannels()
	channel1 := make(chan []byte)
	channel2 := make(chan []byte)
	groupedChannels.add(channel1, "3")
	groupedChannels.add(channel2, "3", "")

	assert.Equal(t, len(groupedChannels.get("3", "")), 2)
	assert.Equal(t, groupedChannels.get("3", "")[0], channel1)
	assert.Equal(t, groupedChannels.get("3", "")[1], channel2)

	assert.Equal(t, len(groupedChannels.get("3")), 2)
	assert.Equal(t, groupedChannels.get("3")[0], channel1)
	assert.Equal(t, groupedChannels.get("3")[1], channel2)
}

func TestDelete(t *testing.T) {
	groupedChannels := newGroupedChannels()
	channel1 := make(chan []byte)
	channel2 := make(chan []byte)

	groupedChannels.add(channel1, "3")
	groupedChannels.add(channel2, "3")

	groupedChannels.delete(channel1, "3")

	assert.Equal(t, len(groupedChannels.get("3")), 1)
	assert.Equal(t, groupedChannels.get("3")[0], channel2)
}

func TestNestedDelete(t *testing.T) {
	groupedChannels := newGroupedChannels()
	channel1 := make(chan []byte)
	channel2 := make(chan []byte)

	groupedChannels.add(channel1, "3", "a")
	groupedChannels.add(channel2, "3")

	groupedChannels.delete(channel1, "3", "a")

	assert.Equal(t, len(groupedChannels.get("3", "a")), 0)
	assert.Equal(t, groupedChannels.get("3")[0], channel2)
}
