package elector_test

import (
	"errors"
	"time"

	"code.cloudfoundry.org/loggregator/syslog_drain_binder/elector"

	"github.com/cloudfoundry/storeadapter"
	"github.com/cloudfoundry/storeadapter/fakestoreadapter"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Elector", func() {
	var fakeStore *fakestoreadapter.FakeStoreAdapter

	BeforeEach(func() {
		fakeStore = fakestoreadapter.New()
	})

	Context("at initialization", func() {
		It("connects to the store", func() {
			elector.NewElector("name", fakeStore, 1*time.Millisecond)
			Expect(fakeStore.DidConnect).To(BeTrue())
		})
	})

	Describe("RunForElection", func() {
		var candidate *elector.Elector

		BeforeEach(func() {
			candidate = elector.NewElector("name", fakeStore, time.Second)
		})

		It("makes a bid to become leader", func() {
			err := candidate.RunForElection()
			Expect(err).NotTo(HaveOccurred())

			node, err := fakeStore.Get("syslog_drain_binder/leader")
			Expect(err).NotTo(HaveOccurred())
			Expect(node.Value).To(BeEquivalentTo("name"))
			Expect(node.TTL).To(Equal(uint64(1)))
		})

		It("sets the IsLeader flag to true", func() {
			candidate.RunForElection()
			Expect(candidate.IsLeader()).To(BeTrue())
		})

		It("re-attempts on an interval if key already exists", func() {
			key := "syslog_drain_binder/leader"
			err := fakeStore.Create(storeadapter.StoreNode{
				Key:   key,
				Value: []byte("some-other-instance"),
			})
			Expect(err).NotTo(HaveOccurred())

			go candidate.RunForElection()

			Consistently(candidate.IsLeader).Should(BeFalse())

			Expect(fakeStore.Delete(key)).To(Succeed())
			Eventually(candidate.IsLeader, 3).Should(BeTrue())
		})

		It("returns an error if any other error occurs while setting key", func() {
			testError := errors.New("test error")
			fakeStore.CreateErrInjector = fakestoreadapter.NewFakeStoreAdapterErrorInjector("syslog_drain_binder", testError)

			err := candidate.RunForElection()
			Expect(err).To(Equal(testError))
			Expect(candidate.IsLeader()).To(BeFalse())
		})

		It("returns nil when it eventually wins", func() {
			preExistingNode := storeadapter.StoreNode{
				Key:   "syslog_drain_binder/leader",
				Value: []byte("some-other-instance"),
			}
			err := fakeStore.Create(preExistingNode)
			Expect(err).NotTo(HaveOccurred())

			errChan := make(chan error)
			go func() {
				errChan <- candidate.RunForElection()
			}()

			time.Sleep(1 * time.Second)
			fakeStore.Delete(preExistingNode.Key)
			Eventually(errChan, 2).Should(Receive(BeNil()))
		})
	})

	Describe("StayAsLeader", func() {
		var candidate *elector.Elector

		BeforeEach(func() {
			fakeStore.Create(storeadapter.StoreNode{
				Key:   "syslog_drain_binder/leader",
				Value: []byte("candidate1"),
			})
		})

		Context("when already leader", func() {
			It("maintains leadership of cluster if successful", func() {
				candidate = elector.NewElector("candidate1", fakeStore, time.Second)

				err := candidate.StayAsLeader()
				Expect(err).NotTo(HaveOccurred())

				node, _ := fakeStore.Get("syslog_drain_binder/leader")
				Expect(node.Value).To(BeEquivalentTo("candidate1"))
				Expect(node.TTL).To(Equal(uint64(1)))
				Expect(candidate.IsLeader()).To(BeTrue())
			})
		})

		Context("when not the cluster leader", func() {
			It("returns an error", func() {
				candidate = elector.NewElector("candidate2", fakeStore, time.Second)

				err := candidate.StayAsLeader()
				Expect(err).To(HaveOccurred())
				Expect(candidate.IsLeader()).To(BeFalse())
			})

			It("does not replace the existing leader", func() {
				candidate = elector.NewElector("candidate2", fakeStore, time.Second)

				candidate.StayAsLeader()

				node, _ := fakeStore.Get("syslog_drain_binder/leader")
				Expect(node.Value).To(BeEquivalentTo("candidate1"))
			})
		})
	})

	Describe("Vacate", func() {
		var candidate *elector.Elector

		BeforeEach(func() {
			candidate = elector.NewElector("candidate1", fakeStore, time.Second)
			candidate.RunForElection()
		})

		It("sets the IsLeader flag to false", func() {
			candidate.Vacate()
			Expect(candidate.IsLeader()).To(BeFalse())
		})

		Context("when leader", func() {
			It("deletes the node and returns nil", func() {
				err := candidate.Vacate()
				Expect(err).NotTo(HaveOccurred())

				node, err := fakeStore.Get("syslog_drain_binder/leader")
				Expect(node).To(Equal(storeadapter.StoreNode{}))
				Expect(err.(storeadapter.Error).Type()).To(Equal(storeadapter.ErrorKeyNotFound))
			})
		})

		Context("when already not leader", func() {
			BeforeEach(func() {
				fakeStore.Delete("syslog_drain_binder/leader")
				fakeStore.Create(storeadapter.StoreNode{
					Key:   "syslog_drain_binder/leader",
					Value: []byte("candidate2"),
				})
			})

			It("returns an error", func() {
				err := candidate.Vacate()
				Expect(err.(storeadapter.Error).Type()).To(Equal(storeadapter.ErrorKeyComparisonFailed))
			})

			It("does not affect the existing leader", func() {
				candidate.Vacate()

				node, _ := fakeStore.Get("syslog_drain_binder/leader")
				Expect(node).To(Equal(storeadapter.StoreNode{
					Key:   "syslog_drain_binder/leader",
					Value: []byte("candidate2"),
				}))
			})
		})
	})
})
