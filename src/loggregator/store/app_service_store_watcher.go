package store

import (
	"github.com/cloudfoundry/storeadapter"
	"path"

	"github.com/cloudfoundry/loggregatorlib/cfcomponent"
	"loggregator/domain"
	"loggregator/store/cache"
)

type AppServiceStoreWatcher struct {
	adapter                   storeadapter.StoreAdapter
	outAddChan, outRemoveChan chan<- domain.AppService
	cache                     cache.AppServiceWatcherCache
}

func NewAppServiceStoreWatcher(adapter storeadapter.StoreAdapter, cache cache.AppServiceWatcherCache) (*AppServiceStoreWatcher, <-chan domain.AppService, <-chan domain.AppService) {
	outAddChan := make(chan domain.AppService)
	outRemoveChan := make(chan domain.AppService)
	return &AppServiceStoreWatcher{
		adapter:       adapter,
		outAddChan:    outAddChan,
		outRemoveChan: outRemoveChan,
		cache:         cache,
	}, outAddChan, outRemoveChan
}

func (w *AppServiceStoreWatcher) Add(appService domain.AppService) {
	if !w.cache.Exists(appService) {
		w.cache.Add(appService)
		w.outAddChan <- appService
	}
}

func (w *AppServiceStoreWatcher) Remove(appService domain.AppService) {
	if w.cache.Exists(appService) {
		w.cache.Remove(appService)
		w.outRemoveChan <- appService
	}
}

func (w *AppServiceStoreWatcher) RemoveApp(appId string) []domain.AppService {
	appServices := w.cache.RemoveApp(appId)
	for _, appService := range appServices {
		w.outRemoveChan <- appService
	}
	return appServices
}

func (w *AppServiceStoreWatcher) Get(appId string) []domain.AppService {
	return w.cache.Get(appId)
}

func (w *AppServiceStoreWatcher) Exists(appService domain.AppService) bool {
	return w.cache.Exists(appService)
}

func (w *AppServiceStoreWatcher) Run() {
	defer func() {
		close(w.outAddChan)
		close(w.outRemoveChan)
	}()

	w.registerExistingServicesFromStore()

	events, _, _ := w.adapter.Watch("/loggregator/services")

	for event := range events {
		cfcomponent.Logger.Debugf("AppStoreWatcher: Got an event from store %s", event.Type)
		switch event.Type {
		case storeadapter.CreateEvent, storeadapter.UpdateEvent:
			if event.Node.Dir || len(event.Node.Value) == 0 {
				// we can ignore any directory nodes (app or other namespace additions)
				continue
			}
			w.Add(appServiceFromStoreNode(*(event.Node)))
		case storeadapter.DeleteEvent:
			w.deleteEvent(*(event.PrevNode))
		case storeadapter.ExpireEvent:
			w.deleteEvent(*(event.PrevNode))
		}
	}
}

func (w *AppServiceStoreWatcher) registerExistingServicesFromStore() {
	cfcomponent.Logger.Debug("AppStoreWatcher: Ensuring existing services are registered")
	services, _ := w.adapter.ListRecursively("/loggregator/services/")
	for _, node := range services.ChildNodes {
		for _, node := range node.ChildNodes {
			appService := appServiceFromStoreNode(node)
			w.Add(appService)
		}
	}
	cfcomponent.Logger.Debug("AppStoreWatcher: Existing services all registered")
}

func appServiceFromStoreNode(node storeadapter.StoreNode) domain.AppService {
	key := node.Key
	appId := path.Base(path.Dir(key))
	serviceUrl := string(node.Value)
	appService := domain.AppService{AppId: appId, Url: serviceUrl}

	return appService
}

func (w *AppServiceStoreWatcher) deleteEvent(node storeadapter.StoreNode) {
	if node.Dir {
		key := node.Key
		appId := path.Base(key)
		w.RemoveApp(appId)
	} else {
		w.Remove(appServiceFromStoreNode(node))
	}
}
