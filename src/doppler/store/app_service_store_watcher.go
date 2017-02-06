package store

import (
	"encoding/json"
	"log"
	"path"

	"github.com/cloudfoundry/storeadapter"
)

const adapterWatchDir = "/loggregator/v2/services"

type appServiceMetadata struct {
	Hostname string `json:"hostname"`
	DrainURL string `json:"drainURL"`
}

type AppServiceStoreWatcher struct {
	adapter                   storeadapter.StoreAdapter
	outAddChan, outRemoveChan chan<- AppService
	cache                     AppServiceWatcherCache

	done chan struct{}
}

func NewAppServiceStoreWatcher(
	adapter storeadapter.StoreAdapter,
	cache AppServiceWatcherCache,
) (*AppServiceStoreWatcher, <-chan AppService, <-chan AppService) {
	outAddChan := make(chan AppService)
	outRemoveChan := make(chan AppService)
	return &AppServiceStoreWatcher{
		adapter:       adapter,
		outAddChan:    outAddChan,
		outRemoveChan: outRemoveChan,
		cache:         cache,
		done:          make(chan struct{}),
	}, outAddChan, outRemoveChan
}

func (w *AppServiceStoreWatcher) Add(appService AppService) {
	if !w.cache.Exists(appService) {
		w.cache.Add(appService)
		w.outAddChan <- appService
	}
}

func (w *AppServiceStoreWatcher) Remove(appService AppService) {
	if w.cache.Exists(appService) {
		w.cache.Remove(appService)
		w.outRemoveChan <- appService
	}
}

func (w *AppServiceStoreWatcher) RemoveApp(appId string) []AppService {
	appServices := w.cache.RemoveApp(appId)
	for _, appService := range appServices {
		w.outRemoveChan <- appService
	}
	return appServices
}

func (w *AppServiceStoreWatcher) Get(appId string) []AppService {
	return w.cache.Get(appId)
}

func (w *AppServiceStoreWatcher) Exists(appService AppService) bool {
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

	events, stopChan, errChan := w.adapter.Watch(adapterWatchDir)

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
			events, stopChan, errChan = w.adapter.Watch(adapterWatchDir)
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
				appService, err := appServiceFromStoreNode(event.Node)
				if err != nil {
					continue
				}
				w.Add(appService)
			case storeadapter.DeleteEvent, storeadapter.ExpireEvent:
				w.deleteEvent(event.PrevNode)
			}
		}
	}
}

func (w *AppServiceStoreWatcher) registerExistingServicesFromStore() {
	services, _ := w.adapter.ListRecursively(adapterWatchDir)
	for _, node := range services.ChildNodes {
		for _, node := range node.ChildNodes {
			appService, err := appServiceFromStoreNode(&node)
			if err != nil {
				continue
			}
			w.Add(appService)
		}
	}
}

func appServiceFromStoreNode(node *storeadapter.StoreNode) (AppService, error) {
	key := node.Key
	appId := path.Base(path.Dir(key))

	var metadata appServiceMetadata
	err := json.Unmarshal(node.Value, &metadata)
	if err != nil {
		return nil, err
	}

	return NewServiceInfo(appId, metadata.DrainURL, metadata.Hostname), nil
}

func (w *AppServiceStoreWatcher) deleteEvent(node *storeadapter.StoreNode) {
	if node.Dir {
		key := node.Key
		appId := path.Base(key)
		w.RemoveApp(appId)
		return
	}

	appService, err := appServiceFromStoreNode(node)
	if err != nil {
		return
	}
	w.Remove(appService)
}
