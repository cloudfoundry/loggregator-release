package store

import (
	"github.com/cloudfoundry/storeadapter"
	"path"
)

type AppServiceStore struct {
	adapter                   storeadapter.StoreAdapter
	outAddChan, outRemoveChan chan<- AppService
	incomingChan              <-chan AppServices
	cache                     map[string]map[string]AppService
}

func NewAppServiceStore(adapter storeadapter.StoreAdapter, outAdd, outRemove chan<- AppService, in <-chan AppServices) *AppServiceStore {
	return &AppServiceStore{
		adapter:       adapter,
		outAddChan:    outAdd,
		outRemoveChan: outRemove,
		incomingChan:  in,
		cache:         make(map[string]map[string]AppService),
	}
}

func (s AppServiceStore) Run() {
	defer func() {
		close(s.outAddChan)
		close(s.outRemoveChan)
	}()
	services, _ := s.adapter.ListRecursively("/loggregator/services/")
	for _, appNode := range services.ChildNodes {
		appId := path.Base(appNode.Key)
		for _, serviceNode := range appNode.ChildNodes {
			appService := AppService{AppId: appId, Url: string(serviceNode.Value)}
			s.addToCache(appService)
			s.outAddChan <- appService
		}
	}

	for appServices := range s.incomingChan {
		appCache, ok := s.cache[appServices.AppId]
		if !ok {
			appCache = make(map[string]AppService)
		}

		if len(appServices.Urls) == 0 {
			s.removeAppFromStore(appServices.AppId)
			continue
		}

		appServiceToAdd := []AppService{}
		appServiceToRemove := []AppService{}
		serviceUrls := make(map[string]bool)

		for _, serviceUrl := range appServices.Urls {
			serviceUrls[serviceUrl] = true

			appService := AppService{AppId: appServices.AppId, Url: serviceUrl}
			_, ok := appCache[appService.Id()]
			if !ok {
				appServiceToAdd = append(appServiceToAdd, appService)
			}
		}

		for _, appService := range appCache {
			if !serviceUrls[appService.Url] {
				appServiceToRemove = append(appServiceToRemove, appService)
			}
		}

		s.addToStore(appServiceToAdd)
		s.removeFromStore(appServiceToRemove)
	}
}

func (s AppServiceStore) addToStore(appServices []AppService) {
	nodes := make([]storeadapter.StoreNode, len(appServices))
	for i, appService := range appServices {
		s.addToCache(appService)
		nodes[i] = storeadapter.StoreNode{
			Key:   path.Join("/loggregator/services", appService.AppId, appService.Id()),
			Value: []byte(appService.Url),
		}
	}

	s.adapter.SetMulti(nodes)
	for _, appService := range appServices {
		s.outAddChan <- appService
	}
}

func (s AppServiceStore) removeFromStore(appServices []AppService) {
	keys := make([]string, len(appServices))
	for i, appService := range appServices {
		s.removeFromCache(appService)
		keys[i] = path.Join("/loggregator/services", appService.AppId, appService.Id())
	}

	s.adapter.Delete(keys...)
	for _, appService := range appServices {
		s.outRemoveChan <- appService
	}
}

func (s AppServiceStore) removeAppFromStore(appId string) {
	s.adapter.Delete(path.Join("/loggregator/services", appId))
	appServices := s.removeAppFromCache(appId)
	for _, appService := range appServices {
		s.outRemoveChan <- appService
	}
}

func (s AppServiceStore) addToCache(appService AppService) {
	appCache, ok := s.cache[appService.AppId]
	if !ok {
		appCache = make(map[string]AppService)
		s.cache[appService.AppId] = appCache
	}

	appCache[appService.Id()] = appService
}

func (s AppServiceStore) removeFromCache(appService AppService) {
	appCache, ok := s.cache[appService.AppId]
	if !ok {
		return
	}
	delete(appCache, appService.Id())
	if len(appCache) == 0 {
		delete(s.cache, appService.AppId)
	}
}

func (s AppServiceStore) removeAppFromCache(appId string) map[string]AppService {
	appCache := s.cache[appId]
	delete(s.cache, appId)
	return appCache
}
