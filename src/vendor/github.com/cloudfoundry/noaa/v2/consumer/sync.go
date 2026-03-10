package consumer

import (
	"bytes"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/url"
	"strings"

	"github.com/cloudfoundry/noaa/v2"
	"github.com/cloudfoundry/sonde-go/events"
	"google.golang.org/protobuf/proto"
)

// ContainerMetrics is deprecated in favor of ContainerEnvelopes, since
// returning the ContainerMetric type directly hides important
// information, like the timestamp.
//
// The returned values will be the same as ContainerEnvelopes, just with
// the Envelope stripped out.
func (c *Consumer) ContainerMetrics(appGuid string, authToken string) ([]*events.ContainerMetric, error) {
	envelopes, err := c.ContainerEnvelopes(appGuid, authToken)
	if err != nil {
		return nil, err
	}
	messages := make([]*events.ContainerMetric, 0, len(envelopes))
	for _, env := range envelopes {
		messages = append(messages, env.GetContainerMetric())
	}
	noaa.SortContainerMetrics(messages)
	return messages, nil
}

// ContainerEnvelopes connects to trafficcontroller via its 'containermetrics'
// http(s) endpoint and returns the most recent dropsonde envelopes for an app.
func (c *Consumer) ContainerEnvelopes(appGuid, authToken string) ([]*events.Envelope, error) {
	envelopes, err := c.readTC(appGuid, authToken, "containermetrics")
	if err != nil {
		return nil, err
	}
	for _, env := range envelopes {
		if env.GetEventType() == events.Envelope_LogMessage {
			return nil, fmt.Errorf("upstream error: %s", env.GetLogMessage().GetMessage())
		}
	}
	return envelopes, nil
}

func (c *Consumer) readTC(appGuid string, authToken string, endpoint string) ([]*events.Envelope, error) {
	trafficControllerUrl, err := url.ParseRequestURI(c.trafficControllerUrl)
	if err != nil {
		return nil, err
	}

	recentPath := c.recentPathBuilder(trafficControllerUrl, appGuid, endpoint)

	resp, err := c.requestTC(recentPath, authToken)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	reader, err := getMultipartReader(resp)
	if err != nil {
		return nil, err
	}

	var buffer bytes.Buffer

	var envelopes []*events.Envelope
	for part, loopErr := reader.NextPart(); loopErr == nil; part, loopErr = reader.NextPart() {
		buffer.Reset()

		_, err = buffer.ReadFrom(part)
		if err != nil {
			break
		}

		envelope := new(events.Envelope)
		if err := proto.Unmarshal(buffer.Bytes(), envelope); err != nil {
			continue
		}

		envelopes = append(envelopes, envelope)
	}

	return envelopes, nil
}

func (c *Consumer) requestTC(path, authToken string) (*http.Response, error) {
	if authToken == "" && c.refreshTokens {
		return c.requestTCNewToken(path)
	}
	var err error
	resp, httpErr := c.tryTCConnection(path, authToken)
	if httpErr != nil {
		err = httpErr.error
		if httpErr.statusCode == http.StatusUnauthorized && c.refreshTokens {
			resp, err = c.requestTCNewToken(path)
		}
	}
	return resp, err
}

func (c *Consumer) requestTCNewToken(path string) (*http.Response, error) {
	token, err := c.getToken()
	if err != nil {
		return nil, err
	}
	conn, httpErr := c.tryTCConnection(path, token)
	if httpErr != nil {
		return nil, httpErr.error
	}
	return conn, nil
}

func (c *Consumer) tryTCConnection(recentPath, token string) (*http.Response, *httpError) {
	req, _ := http.NewRequest("GET", recentPath, nil)
	req.Header.Set("Authorization", token)

	resp, err := c.client.Do(req)
	if err != nil {
		message := `error dialing trafficcontroller server: %s.
Please ask your Cloud Foundry Operator to check the platform configuration (trafficcontroller endpoint is %s)`
		return nil, &httpError{
			statusCode: -1,
			error:      fmt.Errorf(message, err, c.trafficControllerUrl),
		}
	}

	return resp, checkForErrors(resp)
}

func getMultipartReader(resp *http.Response) (*multipart.Reader, error) {
	contentType := resp.Header.Get("Content-Type")

	if len(strings.TrimSpace(contentType)) == 0 {
		return nil, ErrBadResponse
	}

	matches := boundaryRegexp.FindStringSubmatch(contentType)

	if len(matches) != 2 || len(strings.TrimSpace(matches[1])) == 0 {
		return nil, ErrBadResponse
	}
	reader := multipart.NewReader(resp.Body, matches[1])
	return reader, nil
}
