package main_test

import (
	main "metron"

	"errors"
	"github.com/cloudfoundry/gosteno"
	"github.com/cloudfoundry/loggregatorlib/cfcomponent"
	"github.com/cloudfoundry/yagnats"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

type fakeRegistrarFactory struct {
	fakeRegistrar *fakeCollectorRegistrar
}

func (f *fakeRegistrarFactory) Create(mBusClient yagnats.NATSClient, logger *gosteno.Logger) main.CollectorRegistrar {
	return f.fakeRegistrar
}

type fakeCollectorRegistrar struct {
	component *cfcomponent.Component
}

func (r *fakeCollectorRegistrar) RegisterWithCollector(cfc cfcomponent.Component) error {
	r.component = &cfc
	return nil
}

var _ = Describe("InitializeComponent", func() {
	var (
		fakeRegistrar *fakeCollectorRegistrar
		fakeFactory   *fakeRegistrarFactory
		config        *main.Config
		logger        *gosteno.Logger
	)

	BeforeEach(func() {
		fakeRegistrar = &fakeCollectorRegistrar{}
		fakeFactory = &fakeRegistrarFactory{fakeRegistrar}
		config = &main.Config{}
		logger = cfcomponent.NewLogger(false, "/dev/null", "", config.Config)
	})

	It("registers with the collector", func() {
		component := main.InitializeComponent(fakeFactory.Create, *config, logger)
		Expect(fakeRegistrar.component).To(Equal(component))
	})

	It("panics if the default yagnats provider returns an error", func() {
		config.NatsHosts = []string{"fake-host"}
		cfcomponent.DefaultYagnatsClientProvider = func(logger *gosteno.Logger, c *cfcomponent.Config) (yagnats.NATSClient, error) {
			return nil, errors.New("fake error")
		}

		Expect(func() {
			main.InitializeComponent(fakeFactory.Create, *config, logger)
		}).To(Panic())
	})
})
