package trafficcontroller_testhelpers

import (
	"encoding/binary"
	"errors"
	"github.com/cloudfoundry/gosteno"
	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/assert"
	"net/http"
	"testing"
)

const (
	VALID_AUTHENTICATION_TOKEN   = "bearer correctAuthorizationToken"
	INVALID_AUTHENTICATION_TOKEN = "incorrectAuthorizationToken"
)

func SuccessfulAuthorizer(authToken string, target string, l *gosteno.Logger) bool {
	return authToken == VALID_AUTHENTICATION_TOKEN
}

type AuthorizerResult struct {
	Authorized   bool
	ErrorMessage string
}

type LogAuthorizer struct {
	TokenParam string
	Target     string
	Result     AuthorizerResult
}

func (a *LogAuthorizer) Authorize(authToken string, target string, l *gosteno.Logger) (bool, error) {
	a.TokenParam = authToken
	a.Target = target

	return a.Result.Authorized, errors.New(a.Result.ErrorMessage)
}

type AdminAuthorizer struct {
	TokenParam string
	Result     AuthorizerResult
}

func (a *AdminAuthorizer) Authorize(authToken string, l *gosteno.Logger) (bool, error) {
	a.TokenParam = authToken

	return a.Result.Authorized, errors.New(a.Result.ErrorMessage)
}

func AssertConnectionFails(t *testing.T, port string, path string, authToken string, expectedErrorCode uint16) {
	requestHeader := http.Header{}
	if authToken != "" {
		requestHeader = http.Header{"Authorization": []string{authToken}}
	}

	ws, _, err := websocket.DefaultDialer.Dial("ws://localhost:"+port+path, requestHeader)

	assert.NoError(t, err)
	_, data, err := ws.ReadMessage()
	assert.NoError(t, err)
	errorCode := binary.BigEndian.Uint16(data)
	assert.Equal(t, expectedErrorCode, errorCode)
	assert.Equal(t, "EOF", err.Error())
}

type FakeListener struct {
	Uri     string
	Channel chan []byte
}

func NewFakeListener() *FakeListener {
	return &FakeListener{
		Channel: make(chan []byte, 1024),
	}
}

func (f *FakeListener) Start(url string) (<-chan []byte, error) {
	f.Uri = url
	return f.Channel, nil
}

func (f *FakeListener) Stop() {
	close(f.Channel)
}
