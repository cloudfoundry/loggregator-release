package trafficcontroller_testhelpers

import (
	"encoding/binary"
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
