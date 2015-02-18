package syslogwriter

import (
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"
)

const (
	rfc5424 = "2006-01-02T15:04:05.999999Z07:00"
)

type Writer interface {
	Connect() error
	WriteStdout(b []byte, source, sourceId string, timestamp int64) (int, error)
	WriteStderr(b []byte, source, sourceId string, timestamp int64) (int, error)
	Close() error
	IsConnected() bool
	SetConnected(bool)
}

func NewWriter(outputUrl *url.URL, appId string, skipCertVerify bool) (Writer, error) {
	switch outputUrl.Scheme {
	case "https":
		return NewHttpsWriter(outputUrl, appId, skipCertVerify)
	case "syslog":
		return NewSyslogWriter(outputUrl, appId)
	case "syslog-tls":
		return NewTlsWriter(outputUrl, appId, skipCertVerify)
	default:
		return nil, errors.New(fmt.Sprintf("Invalid scheme type %s, must be https, syslog-tls or syslog", outputUrl.Scheme))
	}
}

func clean(in string) string {
	return strings.Replace(in, "\000", "", -1)
}

func createMessage(p int, appId string, source string, sourceId string, msg string, timestamp int64) string {
	// ensure it ends in a \n
	nl := ""
	if !strings.HasSuffix(msg, "\n") {
		nl = "\n"
	}

	msg = clean(msg)
	timeString := time.Unix(0, timestamp).Format(rfc5424)
	timeString = strings.Replace(timeString, "Z", "+00:00", 1)

	var formattedSource string
	if source == "App" {
		formattedSource = fmt.Sprintf("[%s/%s]", source, sourceId)
	} else {
		formattedSource = fmt.Sprintf("[%s]", source)
	}

	// syslog format https://tools.ietf.org/html/rfc5424#section-6
	return fmt.Sprintf("<%d>1 %s %s %s %s - - %s%s", p, timeString, "loggregator", appId, formattedSource, msg, nl)
}
