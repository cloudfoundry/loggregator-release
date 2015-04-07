package integration_test

import (
	"github.com/apcera/nats"
	"github.com/cloudfoundry/gunk/natsrunner"
	"github.com/cloudfoundry/loggregatorlib/cfcomponent/registrars/collectorregistrar"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

const natsPort = 24484

var _ = Describe("Collector registration", func() {
	It("registers itself with the collector", func() {
		natsRunner := natsrunner.NewNATSRunner(natsPort)
		natsRunner.Start()
		defer natsRunner.Stop()

		messageChan := make(chan []byte)
		natsClient := natsRunner.MessageBus
		natsClient.Subscribe(collectorregistrar.AnnounceComponentMessageSubject, func(msg *nats.Msg) {
			messageChan <- msg.Data
		})

		Eventually(messageChan).Should(Receive(MatchRegexp(`^\{"type":"MetronAgent","index":42,"host":"[^:]*:1234","uuid":"42-[0-9a-f-]{36}","credentials":\["admin","admin"\]\}$`)))
	})
})
