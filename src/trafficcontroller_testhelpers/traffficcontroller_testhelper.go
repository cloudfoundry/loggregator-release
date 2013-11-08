package trafficcontroller_testhelpers

import (
	"code.google.com/p/go.net/websocket"
	"encoding/binary"
	"github.com/cloudfoundry/gosteno"
	"github.com/stretchr/testify/assert"
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
	config, err := websocket.NewConfig("ws://localhost:"+port+path, "http://localhost")
	assert.NoError(t, err)
	if authToken != "" {
		config.Header.Add("Authorization", authToken)
	}

	ws, err := websocket.DialConfig(config)

	assert.NoError(t, err)
	data := make([]byte, 2)
	_, err = ws.Read(data)
	errorCode := binary.BigEndian.Uint16(data)
	assert.Equal(t, expectedErrorCode, errorCode)
	assert.Equal(t, "EOF", err.Error())
}
