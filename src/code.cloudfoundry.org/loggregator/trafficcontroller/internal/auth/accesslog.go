package auth

import (
	"bytes"
	"fmt"
	"html/template"
	"log"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const REQUEST_ID_HEADER = "X-Vcap-Request-ID"

var logTemplate *template.Template

type templateContext struct {
	Timestamp       int64
	RequestID       string
	Path            string
	Method          string
	SourceHost      string
	SourcePort      string
	DestinationHost string
	DestinationPort string
}

type AccessLog struct {
	request   *http.Request
	timestamp time.Time
	host      string
	port      uint32
}

func NewAccessLog(req *http.Request, ts time.Time, host string, port uint32) *AccessLog {
	return &AccessLog{
		request:   req,
		timestamp: ts,
		host:      host,
		port:      port,
	}
}

func (al *AccessLog) String() string {
	vcapRequestId := al.request.Header.Get(REQUEST_ID_HEADER)
	path := al.request.URL.Path
	if al.request.URL.RawQuery != "" {
		path = fmt.Sprintf("%s?%s", al.request.URL.Path, al.request.URL.RawQuery)
	}
	remoteHost, remotePort := al.extractRemoteInfo()

	context := templateContext{
		toMillis(al.timestamp),
		vcapRequestId,
		path,
		al.request.Method,
		remoteHost,
		remotePort,
		al.host,
		strconv.Itoa(int(al.port)),
	}
	var buf bytes.Buffer
	err := logTemplate.Execute(&buf, context)
	if err != nil {
		log.Printf("Error executing security access log template: %s\n", err)
		return ""
	}
	return buf.String()
}

func (al *AccessLog) extractRemoteInfo() (string, string) {
	remoteAddr := al.request.Header.Get("X-Forwarded-For")
	index := strings.Index(remoteAddr, ",")
	if index > -1 {
		remoteAddr = remoteAddr[:index]
	}

	if remoteAddr == "" {
		remoteAddr = al.request.RemoteAddr
	}

	if !strings.Contains(remoteAddr, ":") {
		remoteAddr += ":"
	}
	host, port, err := net.SplitHostPort(remoteAddr)
	if err != nil {
		log.Printf("Error splitting host and port for access log: %s\n", err)
		return "", ""
	}
	return host, port
}

func toMillis(timestamp time.Time) int64 {
	return timestamp.UnixNano() / int64(time.Millisecond)
}

func init() {
	extensions := []string{
		"rt={{ .Timestamp }}",
		"cs1Label=userAuthenticationMechanism",
		"cs1=oauth-access-token",
		"cs2Label=vcapRequestId",
		"cs2={{ .RequestID }}",
		"request={{ .Path }}",
		"requestMethod={{ .Method }}",
		"src={{ .SourceHost }}",
		"spt={{ .SourcePort }}",
		"dst={{ .DestinationHost }}",
		"dpt={{ .DestinationPort }}",
	}
	fields := []string{
		"0",
		"cloud_foundry",
		"loggregator_trafficcontroller",
		"1.0",
		"{{ .Method }} {{ .Path }}",
		"{{ .Method }} {{ .Path }}",
		"0",
		strings.Join(extensions, " "),
	}
	templateSource := "CEF:" + strings.Join(fields, "|")
	var err error
	logTemplate, err = template.New("access_log").Parse(templateSource)
	if err != nil {
		log.Panic(err)
	}
}
