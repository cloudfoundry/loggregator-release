package websocketmessagereader_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

func TestWebsocketmessagereader(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Websocket Message Reader Suite")
}
