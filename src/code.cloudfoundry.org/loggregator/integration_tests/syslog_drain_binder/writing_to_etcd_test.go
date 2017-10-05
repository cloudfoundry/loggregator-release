package syslog_drain_binder_test

import (
	"net/http"
	"net/http/httptest"

	"code.cloudfoundry.org/loggregator/syslog_drain_binder/config"
	"code.cloudfoundry.org/loggregator/syslog_drain_binder/fake_cc"
	"code.cloudfoundry.org/loggregator/testservers"
	"code.cloudfoundry.org/workpool"
	"github.com/cloudfoundry/storeadapter"
	"github.com/cloudfoundry/storeadapter/etcdstoreadapter"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Syslog Drain Binder", func() {
	It("stores drain metadata in etcd", func() {
		conf, cleanup := setupSyslogDrainBinder(false)
		defer cleanup()
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

		key1 := "/loggregator/v2/services/app0/3bbc237e9c51de785466aa61c59819fa0132c85f"
		key2 := "/loggregator/v2/services/app1/38a1ee43a50a34ffdd853f074e9adf90136ff58c"
		Eventually(f, 2).Should(HaveLen(2))
		Expect(toMap(nodes)).To(Equal(map[string][]byte{
			key1: []byte(`{"hostname":"org.space.app1","drainURL":"http://example.com"}`),
			key2: []byte(`{"hostname":"org.space.app2","drainURL":"http://example.com"}`),
		}))
		Consistently(f, 2).Should(HaveLen(2))
	})

	It("does not start store any keys in etcd when disabled", func() {
		conf, cleanup := setupSyslogDrainBinder(true)
		defer cleanup()
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

		Consistently(f, 2).Should(HaveLen(0))
	})
})

func setupSyslogDrainBinder(disable bool) (config.Config, func()) {
	fakeCloudController := startFakeCC()
	etcdCleanup, etcdURL := testservers.StartTestEtcd()
	drainCleanup, conf := testservers.StartSyslogDrainBinder(
		testservers.BuildSyslogDrainBinderConfig(
			etcdURL,
			fakeCloudController.URL,
			disable,
		),
	)
	return conf, func() {
		drainCleanup()
		fakeCloudController.Close()
		etcdCleanup()
	}
}

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
