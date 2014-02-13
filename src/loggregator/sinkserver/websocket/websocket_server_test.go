package websocket_test

import (
	"encoding/binary"
	"github.com/gorilla/websocket"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"net/http"
)

var _ = Describe("WebsocketServer", func() {

	assertConnectionFails := func(port string, path string, expectedErrorCode uint16) {
		ws, _, err := websocket.DefaultDialer.Dial("ws://localhost:"+port+path, http.Header{})
		Expect(err).ToNot(HaveOccurred())

		Expect(err).ToNot(HaveOccurred())

		_, data, err := ws.ReadMessage()
		errorCode := binary.BigEndian.Uint16(data)
		Expect(err).To(HaveOccurred())
		Expect(errorCode).To(Equal(expectedErrorCode))
		Expect(err.Error()).To(Equal("EOF"))
	}

	Describe("Start", func() {
		Context("given an unknown path", func() {
			PIt("returns an HTTP 400", func() {
				assertConnectionFails("123", "/INVALID_PATH", 400)
			})
		})
	})
})
