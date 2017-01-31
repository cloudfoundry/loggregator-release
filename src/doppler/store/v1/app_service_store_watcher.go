package v1

import (
	"log"
	"path"

	"doppler/store"

	"github.com/cloudfoundry/storeadapter"
)

type AppServiceStoreWatcher struct {
	adapter                   storeadapter.StoreAdapter
	outAddChan, outRemoveChan chan<- store.AppService
	cache                     AppServiceWatcherCache

	done chan struct{}
}

func NewAppServiceStoreWatcher(
	adapter storeadapter.StoreAdapter,
	cache AppServiceWatcherCache,
) (*AppServiceStoreWatcher, <-chan store.AppService, <-chan store.AppService) {
	outAddChan := make(chan store.AppService)
	outRemoveChan := make(chan store.AppService)
	return &AppServiceStoreWatcher{
		adapter:       adapter,
		outAddChan:    outAddChan,
		outRemoveChan: outRemoveChan,
		cache:         cache,
		done:          make(chan struct{}),
	}, outAddChan, outRemoveChan
}

func (w *AppServiceStoreWatcher) Add(appService store.AppService) {
	if !w.cache.Exists(appService) {
		w.cache.Add(appService)
		w.outAddChan <- appService
	}
}

func (w *AppServiceStoreWatcher) Remove(appService store.AppService) {
	if w.cache.Exists(appService) {
		w.cache.Remove(appService)
		w.outRemoveChan <- appService
	}
}

func (w *AppServiceStoreWatcher) RemoveApp(appId string) []store.AppService {
	appServices := w.cache.RemoveApp(appId)
	for _, appService := range appServices {
		w.outRemoveChan <- appService
	}
	return appServices
}

func (w *AppServiceStoreWatcher) Get(appId string) []store.AppService {
	return w.cache.Get(appId)
}

func (w *AppServiceStoreWatcher) Exists(appService store.AppService) bool {
	return w.cache.Exists(appService)
}

func (w *AppServiceStoreWatcher) Stop() {
	close(w.done)
}

func (w *AppServiceStoreWatcher) Run() {
	defer func() {
		close(w.outAddChan)
		close(w.outRemoveChan)
	}()

	events, stopChan, errChan := w.adapter.Watch("/loggregator/services")

	w.registerExistingServicesFromStore()
	for {
		select {
		case <-w.done:
			close(stopChan)
			return
		case err, ok := <-errChan:
			if !ok {
				return
			}
			log.Printf("AppStoreWatcher: Got error while waiting for ETCD events: %s", err.Error())
			events, stopChan, errChan = w.adapter.Watch("/loggregator/services")
		case event, ok := <-events:
			if !ok {
				return
			}

			switch event.Type {
			case storeadapter.CreateEvent, storeadapter.UpdateEvent:
				if event.Node.Dir || len(event.Node.Value) == 0 {
					// we can ignore any directory nodes (app or other namespace additions)
					continue
				}
				w.Add(appServiceFromStoreNode(event.Node))
			case storeadapter.DeleteEvent, storeadapter.ExpireEvent:
				w.deleteEvent(event.PrevNode)
			}
		}
	}
}

func (w *AppServiceStoreWatcher) registerExistingServicesFromStore() {
	services, _ := w.adapter.ListRecursively("/loggregator/services/")
	for _, node := range services.ChildNodes {
		for _, node := range node.ChildNodes {
			appService := appServiceFromStoreNode(&node)
			w.Add(appService)
		}
	}
}

func appServiceFromStoreNode(node *storeadapter.StoreNode) store.AppService {
	key := node.Key
	appId := path.Base(path.Dir(key))
	serviceUrl := string(node.Value)
	appService := NewServiceInfo(appId, serviceUrl)

	return appService
}

func (w *AppServiceStoreWatcher) deleteEvent(node *storeadapter.StoreNode) {
	if node.Dir {
		key := node.Key
		appId := path.Base(key)
		w.RemoveApp(appId)
	} else {
		w.Remove(appServiceFromStoreNode(node))
	}
}
