package store

import (
	"github.com/cloudfoundry/storeadapter"
	"path"

	"loggregator/domain"
	"loggregator/store/cache"
	"github.com/cloudfoundry/loggregatorlib/cfcomponent"
)

type AppServiceStoreWatcher struct {
	adapter                   storeadapter.StoreAdapter
	outAddChan, outRemoveChan chan<- domain.AppService
	cache                     cache.AppServiceCache
}

func NewAppServiceStoreWatcher(adapter storeadapter.StoreAdapter) (*AppServiceStoreWatcher, <-chan domain.AppService, <-chan domain.AppService) {
	outAddChan := make(chan domain.AppService)
	outRemoveChan := make(chan domain.AppService)
	return &AppServiceStoreWatcher{
		adapter:       adapter,
		outAddChan:    outAddChan,
		outRemoveChan: outRemoveChan,
		cache:         cache.NewAppServiceCache(),
	}, outAddChan, outRemoveChan
}

func (w *AppServiceStoreWatcher) Run() {
	defer func() {
		close(w.outAddChan)
		close(w.outRemoveChan)
	}()

	w.warmUpCache()

	events, _, _ := w.adapter.Watch("/loggregator/services")

	for event := range events {
		cfcomponent.Logger.Debugf("AppStoreWatcher: Got an event from store %s", event.Type)
		switch event.Type {
		case storeadapter.CreateEvent, storeadapter.UpdateEvent:
			if event.Node.Dir || len(event.Node.Value) == 0 {
				// we can ignore any directory nodes (app or other namespace additions)
				continue
			}
			w.serviceCreatedOrUpdated(appServiceFromStoreNode(event.Node))
		case storeadapter.DeleteEvent:
			if event.Node.Dir {
				w.appDeleted(event.Node)
			} else {
				w.serviceDeleted(appServiceFromStoreNode(event.Node))
			}
		case storeadapter.ExpireEvent:
			w.appExpired(event.Node)
		}
	}
}

func (w *AppServiceStoreWatcher) warmUpCache() {
	cfcomponent.Logger.Debug("AppStoreWatcher: Lighting the fires to warm the cache")
	services, _ := w.adapter.ListRecursively("/loggregator/services/")
	for _, node := range services.ChildNodes {
		for _, node := range node.ChildNodes {
			appService := appServiceFromStoreNode(node)
			w.serviceCreatedOrUpdated(appService)
		}
	}
	cfcomponent.Logger.Debug("AppStoreWatcher: Cache all warm and cozy")
}

func appServiceFromStoreNode(node storeadapter.StoreNode) domain.AppService {
	key := node.Key
	appId := path.Base(path.Dir(key))
	serviceUrl := string(node.Value)
	appService := domain.AppService{AppId: appId, Url: serviceUrl}

	return appService
}

func (w *AppServiceStoreWatcher) serviceCreatedOrUpdated(appService domain.AppService) {
	if !w.cache.Exists(appService) {
		w.cache.Add(appService)
		w.outAddChan <- appService
	}
}

func (w *AppServiceStoreWatcher) serviceDeleted(appService domain.AppService) {
	if w.cache.Exists(appService) {
		w.cache.Remove(appService)
		w.outRemoveChan <- appService
	}
}

func (w *AppServiceStoreWatcher) appDeleted(node storeadapter.StoreNode) {
	key := node.Key
	appId := path.Base(key)
	appServices := w.cache.RemoveApp(appId)
	for _, appService := range appServices {
		w.outRemoveChan <- appService
	}
}

func (w *AppServiceStoreWatcher) appExpired(node storeadapter.StoreNode) {
	key := node.Key
	appId := path.Base(key)
	w.cache.RemoveApp(appId)
}
