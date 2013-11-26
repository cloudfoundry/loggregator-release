package iprange

import (
	"testing"
	"github.com/stretchr/testify/assert"
	"net/url"
)

// Tests for ValidateIpAddresses

func TestRecognizesAValidIpAddressRange(t *testing.T) {
	ranges := []IPRange{IPRange{Start: "127.0.2.2", End: "127.0.2.4"}}
	err := ValidateIpAddresses(ranges)
	assert.NoError(t, err)
}

func TestValidationTheStartAddress(t *testing.T) {
	ranges := []IPRange{IPRange{Start: "127.0.2.2.1", End: "127.0.2.4"}}
	err := ValidateIpAddresses(ranges)
	assert.Error(t, err)
	assert.Equal(t, err.Error(), "Invalid IP Address for Blacklist IP Range: 127.0.2.2.1")
}

func TestValidationTheEndAddress(t *testing.T) {
	ranges := []IPRange{IPRange{Start: "127.0.2.2", End: "127.0.2.4.3"}}
	err := ValidateIpAddresses(ranges)
	assert.Error(t, err)
}

func TestValidatesAllGivenIpAddresses(t *testing.T) {
	ranges := []IPRange{
		IPRange{Start: "127.0.2.2", End: "127.0.2.4"},
		IPRange{Start: "127.0.2.2", End: "127.0.2.4.5"},
	}
	err := ValidateIpAddresses(ranges)
	assert.Error(t, err)
}

func TestValidatesThatStartIPIsBeforeEndIP(t *testing.T) {
	ranges := []IPRange{IPRange{Start: "10.10.10.10", End: "10.8.10.12"}}
	err := ValidateIpAddresses(ranges)
	assert.Error(t, err)
	assert.Equal(t, err.Error(), "Invalid Blacklist IP Range: Start 10.10.10.10 has to be before End 10.8.10.12")
}

func TestAcceptsStartAndEndAsTheSame(t *testing.T) {
	ranges := []IPRange{IPRange{Start: "127.0.2.2", End: "127.0.2.2"}}
	err := ValidateIpAddresses(ranges)
	assert.NoError(t, err)
}

// Tests for IpOutsideOfRanges
var ipTests = []struct {
	url  string
	output bool
}{
	{"http://127.0.0.1", true},
	{"http://127.0.1.1", true},
	{"http://127.0.3.5", true},
	{"http://127.0.2.2", false},
	{"http://127.0.2.3", false},
	{"http://127.0.2.4", false},
	{"https://127.0.1.1", true},
	{"https://127.0.2.3", false},
	{"syslog://127.0.1.1", true},
	{"syslog://127.0.2.3", false},
	{"syslog://127.0.1.1:3000", true},
	{"syslog://127.0.2.3:3000", false},
	{"syslog://127.0.1.1:3000/test", true},
	{"syslog://127.0.2.3:3000/test", false},
	{"syslog://127.0.1.1:3000?app=great", true},
	{"syslog://127.0.2.3:3000?app=great", false},
	{"syslog://127.0.2.3:3000?app=great", false},
}

func TestParsesTheIPAddressProperly(t *testing.T) {
	ranges := []IPRange{IPRange{Start: "127.0.1.2", End: "127.0.3.4"}}

	for _, ipTest := range ipTests {
		parsedURL, _ := url.Parse(ipTest.url)
		outOfRange, err := IpOutsideOfRanges(*parsedURL, ranges)
		assert.NoError(t, err)
		assert.Equal(t, outOfRange, ipTest.output, "Wrong output for url: %s", ipTest.url)
	}
}

func TestReturnErrorWhenUrlDoesNotContainProtocol(t *testing.T) {
	ranges := []IPRange{IPRange{Start: "127.0.2.2", End: "127.0.2.4"}}

	parsedURL, _ := url.Parse("127.0.0.1:300/new")
	_, err := IpOutsideOfRanges(*parsedURL, ranges)
	assert.Error(t, err)
	assert.Equal(t, err.Error(), "Missing protocol for url: {  <nil>  127.0.0.1:300/new  }")
}

func TestReturnsAlwaysTrueWhenIpRangesIsNilOrEmpty(t *testing.T) {
	ranges := []IPRange{}

	parsedURL, _ := url.Parse("https://127.0.0.1")
	outSideOfRange, err := IpOutsideOfRanges(*parsedURL, ranges)
	assert.NoError(t, err)
	assert.True(t, outSideOfRange)

	ranges = nil
	outSideOfRange, err = IpOutsideOfRanges(*parsedURL, ranges)
	assert.NoError(t, err)
	assert.True(t, outSideOfRange)
}

func TestResolvesIpAddresses(t *testing.T) {
	ranges := []IPRange{IPRange{Start: "127.0.0.0", End: "127.0.0.4"}}

	parsedURL, _ := url.Parse("syslog://vcap.me:3000?app=great")
	outSideOfRange, err := IpOutsideOfRanges(*parsedURL, ranges)
	assert.NoError(t, err)
	assert.False(t, outSideOfRange)

	parsedURL, _ = url.Parse("syslog://localhost:3000?app=great")
	outSideOfRange, err = IpOutsideOfRanges(*parsedURL, ranges)
	assert.NoError(t, err)
	assert.False(t, outSideOfRange)

	parsedURL, _ = url.Parse("syslog://doesNotExist.nowhere:3000?app=great")
	outSideOfRange, err = IpOutsideOfRanges(*parsedURL, ranges)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "Resolving host failed: ")
}
