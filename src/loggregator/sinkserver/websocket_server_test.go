package sinkserver

import (
	"code.google.com/p/go.net/websocket"
	"encoding/binary"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("WebsocketServer", func() {
	assertConnectionFails := func(port string, path string, expectedErrorCode uint16) {
		config, err := websocket.NewConfig("ws://localhost:"+port+path, "http://localhost")
		Expect(err).ToNot(HaveOccurred())

		ws, err := websocket.DialConfig(config)
		Expect(err).ToNot(HaveOccurred())

		data := make([]byte, 2)
		_, err = ws.Read(data)
		errorCode := binary.BigEndian.Uint16(data)
		Expect(err).To(HaveOccurred())
		Expect(errorCode).To(Equal(expectedErrorCode))
		Expect(err.Error()).To(Equal("EOF"))
	}

	Describe("Start", func() {
		Context("given an unknown path", func() {
			It("returns an HTTP 400", func() {
				assertConnectionFails(SERVER_PORT, "/INVALID_PATH", 400)
			})
		})
	})
})
