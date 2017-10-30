package api_test

import (
	"net/http/httptest"
	"strings"
	sharedapi "tools/reliability/api"
	"tools/reliability/server/internal/api"

	"github.com/gorilla/websocket"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

// TODO: use github.com/posener/wstest
var _ = Describe("WorkerServer", func() {
	It("forwards tests to a client", func() {
		handler := api.NewWorkerHandler()
		server := httptest.NewServer(handler)

		client, err := newFakeClient(strings.Replace(server.URL, "http", "ws", 1))
		Expect(err).ToNot(HaveOccurred())
		Eventually(handler.ConnCount).Should(Equal(1))

		n, err := handler.Run(&sharedapi.Test{})
		Expect(err).ToNot(HaveOccurred())
		Expect(n).To(Equal(1))
		Eventually(client.tests).Should(HaveLen(1))
	})

	It("forwards tests to multiple clients", func() {
		handler := api.NewWorkerHandler()
		server := httptest.NewServer(handler)

		clientA, err := newFakeClient(strings.Replace(server.URL, "http", "ws", 1))
		Expect(err).ToNot(HaveOccurred())
		clientB, err := newFakeClient(strings.Replace(server.URL, "http", "ws", 1))
		Expect(err).ToNot(HaveOccurred())
		Eventually(handler.ConnCount).Should(Equal(2))

		n, err := handler.Run(&sharedapi.Test{})
		Expect(n).To(Equal(2))
		Eventually(clientA.tests).Should(Receive())
		Eventually(clientB.tests).Should(Receive())
	})

	It("shards the number of cycles for each worker to write", func() {
		handler := api.NewWorkerHandler()
		server := httptest.NewServer(handler)

		var clients []*fakeClient
		for i := 0; i < 3; i++ {
			c, err := newFakeClient(strings.Replace(server.URL, "http", "ws", 1))
			Expect(err).ToNot(HaveOccurred())
			clients = append(clients, c)
		}
		Eventually(handler.ConnCount).Should(Equal(3))

		n, err := handler.Run(&sharedapi.Test{
			Cycles: 1000,
		})
		Expect(err).ToNot(HaveOccurred())
		Expect(n).To(Equal(3))

		var writeCyclesTotal uint64
		for _, c := range clients {
			var t sharedapi.Test
			Eventually(c.tests).Should(Receive(&t))
			writeCyclesTotal += t.WriteCycles
		}

		Expect(writeCyclesTotal).To(Equal(uint64(1000)))
	})

	It("doesn't try to write to closed clients", func() {
		handler := api.NewWorkerHandler()
		server := httptest.NewServer(handler)

		clientA, err := newFakeClient(strings.Replace(server.URL, "http", "ws", 1))
		Expect(err).ToNot(HaveOccurred())

		clientB, err := newFakeClient(strings.Replace(server.URL, "http", "ws", 1))
		Expect(err).ToNot(HaveOccurred())
		Eventually(handler.ConnCount).Should(Equal(2))

		clientA.Close()
		Eventually(handler.ConnCount).Should(Equal(1))

		go func() {
			for i := 0; i < 10; i++ {
				n, err := handler.Run(&sharedapi.Test{})
				Expect(err).ToNot(HaveOccurred())
				Expect(n).To(Equal(1))
			}
		}()

		Eventually(clientB.tests).Should(HaveLen(10))
		Consistently(clientA.tests).ShouldNot(Receive())
	})

	Context("with no connections", func() {
		It("return an error", func() {
			handler := api.NewWorkerHandler()
			n, err := handler.Run(&sharedapi.Test{})
			Expect(err).To(HaveOccurred())
			Expect(n).To(Equal(0))
		})
	})
})

type fakeClient struct {
	tests chan sharedapi.Test
	conn  *websocket.Conn
}

func newFakeClient(addr string) (*fakeClient, error) {
	client := &fakeClient{
		tests: make(chan sharedapi.Test, 100),
	}

	conn, _, err := websocket.DefaultDialer.Dial(addr, nil)
	if err != nil {
		return nil, err
	}

	client.conn = conn

	go func() {
		for {
			var test sharedapi.Test
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
