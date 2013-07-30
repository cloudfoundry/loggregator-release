package varz

import (
	. "launchpad.net/gocheck"
)

type CommonSuite struct{}

var _ = Suite(&CommonSuite{})

func (s *CommonSuite) TestUUID(c *C) {
	uuid := generateUUID()

	c.Check(len(uuid), Equals, 32)
}
