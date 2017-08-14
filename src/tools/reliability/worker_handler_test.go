package reliability_test

import (
	"net/http/httptest"
	"strings"
	"tools/reliability"

	"github.com/gorilla/websocket"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("WorkerServer", func() {
	It("forwards tests a client", func() {
		handler := reliability.NewWorkerHandler()
		server := httptest.NewServer(handler)

		client, err := newFakeClient(strings.Replace(server.URL, "http", "ws", 1))
		Expect(err).ToNot(HaveOccurred())

		handler.Run(&reliability.Test{})
		Eventually(client.tests).Should(HaveLen(1))
	})

	It("forwards tests a multiple clients", func() {
		handler := reliability.NewWorkerHandler()
		server := httptest.NewServer(handler)

		clientA, err := newFakeClient(strings.Replace(server.URL, "http", "ws", 1))
		Expect(err).ToNot(HaveOccurred())
		clientB, err := newFakeClient(strings.Replace(server.URL, "http", "ws", 1))
		Expect(err).ToNot(HaveOccurred())

		handler.Run(&reliability.Test{})
		Eventually(clientA.tests).Should(HaveLen(1))
		Eventually(clientB.tests).Should(HaveLen(1))
	})

	It("shards the number of cycles for each worker to write", func() {
		handler := reliability.NewWorkerHandler()
		server := httptest.NewServer(handler)

		var clients []*fakeClient
		for i := 0; i < 3; i++ {
			c, err := newFakeClient(strings.Replace(server.URL, "http", "ws", 1))
			Expect(err).ToNot(HaveOccurred())
			clients = append(clients, c)
		}

		handler.Run(&reliability.Test{
			Cycles: 1000,
		})

		var writeCyclesTotal uint64
		for _, c := range clients {
			var t reliability.Test
			Eventually(c.tests).Should(Receive(&t))
			writeCyclesTotal += t.WriteCycles
		}

		Expect(writeCyclesTotal).To(Equal(uint64(1000)))
	})

	It("doesn't try to write to closed clients", func() {
		handler := reliability.NewWorkerHandler()
		server := httptest.NewServer(handler)

		clientA, err := newFakeClient(strings.Replace(server.URL, "http", "ws", 1))
		Expect(err).ToNot(HaveOccurred())

		clientB, err := newFakeClient(strings.Replace(server.URL, "http", "ws", 1))
		Expect(err).ToNot(HaveOccurred())

		clientA.Close()

		go func() {
			for i := 0; i < 10; i++ {
				handler.Run(&reliability.Test{})
			}
		}()

		Eventually(clientB.tests).Should(HaveLen(10))
		Eventually(clientA.tests).ShouldNot(Receive())
		Consistently(clientA.tests).ShouldNot(Equal(10))
	})
})

type fakeClient struct {
	tests chan reliability.Test
	conn  *websocket.Conn
}

func newFakeClient(addr string) (*fakeClient, error) {
	client := &fakeClient{
		tests: make(chan reliability.Test, 100),
	}

	conn, _, err := websocket.DefaultDialer.Dial(addr, nil)
	if err != nil {
		return nil, err
	}

	client.conn = conn

	go func() {
		for {
			var test reliability.Test
			err := conn.ReadJSON(&test)
			if err != nil {
				break
			}

			client.tests <- test
		}
	}()

	return client, nil
}

func (f *fakeClient) Close() {
	_ = f.conn.Close()
}
