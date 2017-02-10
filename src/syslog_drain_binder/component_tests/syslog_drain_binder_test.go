package component_test

import (
	"net/http"
	"net/http/httptest"

	"code.cloudfoundry.org/workpool"

	"github.com/cloudfoundry/storeadapter"
	"github.com/cloudfoundry/storeadapter/etcdstoreadapter"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"syslog_drain_binder/config"
	"syslog_drain_binder/fake_cc"
	"testservers"
)

var _ = Describe("Syslog Drain Binder", func() {
	Context("when a consumer is accepting gRPC connections", func() {
		var (
			drainCleanup        func()
			etcdCleanup         func()
			etcdURL             string
			conf                config.Config
			fakeCloudController *httptest.Server
		)

		BeforeEach(func() {
			fakeCloudController = startFakeCC()
			etcdCleanup, etcdURL = testservers.StartTestEtcd()
			drainCleanup, conf = testservers.StartSyslogDrainBinder(
				testservers.BuildSyslogDrainBinderConfig(
					etcdURL,
					fakeCloudController.URL,
				),
			)
		})

		AfterEach(func() {
			drainCleanup()
			fakeCloudController.Close()
			etcdCleanup()
		})

		It("works", func() {
			adapter := connectToEtcd(conf)

			var nodes []storeadapter.StoreNode
			f := func() []storeadapter.StoreNode {
				n, err := adapter.ListRecursively("/loggregator/v2/services")
				if err != nil {
					return nil
				}
				nodes = n.ChildNodes
				return nodes
			}

			Eventually(f, 2).Should(HaveLen(2))
			Expect(toMap(nodes)).To(Equal(map[string][]byte{
				"/loggregator/v2/services/app0/3bbc237e9c51de785466aa61c59819fa0132c85f": []byte(`{"hostname":"org.space.app1","drainURL":"http://example.com"}`),
				"/loggregator/v2/services/app1/38a1ee43a50a34ffdd853f074e9adf90136ff58c": []byte(`{"hostname":"org.space.app2","drainURL":"http://example.com"}`),
			}))
			Consistently(f, 2).Should(HaveLen(2))
		})
	})
})

func toMap(nodes []storeadapter.StoreNode) map[string][]byte {
	m := make(map[string][]byte)
	for _, n := range nodes {
		for _, n2 := range n.ChildNodes {
			m[n2.Key] = n2.Value
		}
	}
	return m
}

func startFakeCC() *httptest.Server {
	fakeCloudController := fake_cc.NewFakeCC([]fake_cc.AppEntry{
		{
			AppId: "app0",
			SyslogBinding: fake_cc.SysLogBinding{
				Hostname:  "org.space.app1",
				DrainURLs: []string{"http://example.com"},
			},
		},
		{
			AppId: "app1",
			SyslogBinding: fake_cc.SysLogBinding{
				Hostname: "org.space.app2",
				DrainURLs: []string{
					"http://example.net?drain-version=2.0",
					"http://example.com",
				},
			},
		},
		{
			AppId: "app2",
			SyslogBinding: fake_cc.SysLogBinding{
				Hostname: "org.space.app3",
				DrainURLs: []string{
					"http://example.net?drain-version=2.0",
				},
			},
		},
	})
	testServer := httptest.NewTLSServer(
		http.HandlerFunc(fakeCloudController.ServeHTTP),
	)
	return testServer
}

func connectToEtcd(conf config.Config) storeadapter.StoreAdapter {
	workPool, err := workpool.NewWorkPool(conf.EtcdMaxConcurrentRequests)
	Expect(err).ToNot(HaveOccurred())
	options := &etcdstoreadapter.ETCDOptions{
		ClusterUrls: conf.EtcdUrls,
	}
	etcdStoreAdapter, err := etcdstoreadapter.New(options, workPool)
	Expect(err).ToNot(HaveOccurred())
	err = etcdStoreAdapter.Connect()
	Expect(err).ToNot(HaveOccurred())
	return etcdStoreAdapter
}
