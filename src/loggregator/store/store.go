package store

import (
	"github.com/cloudfoundry/storeadapter"
	"path"
)

type Store struct {
	adapter                   storeadapter.StoreAdapter
	outAddChan, outRemoveChan chan<- AppService
	incomingChan              <-chan AppServices
	cache                     map[string]map[string]AppService
}

func NewStore(adapter storeadapter.StoreAdapter, outAdd, outRemove chan<- AppService, in <-chan AppServices) *Store {
	return &Store{
		adapter:      adapter,
		outAddChan:   outAdd,
		outRemoveChan:   outRemove,
		incomingChan: in,
		cache:        make(map[string]map[string]AppService),
	}
}

func (s Store) Run() {
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

func (s Store) addToStore(appServices []AppService) {
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

func (s Store) removeFromStore(appServices []AppService) {
	keys := make([]string, len(appServices))
	for i, appService := range appServices {
		s.removeFromCache(appService)
		keys[i] = path.Join("/loggregator/services", appService.AppId, appService.Id())
	}

	s.adapter.Delete(keys...)
}

func (s Store) removeAppFromStore(appId string) {
	s.adapter.Delete(path.Join("/loggregator/services", appId))
}

func (s Store) addToCache(appService AppService) {
	appCache, ok := s.cache[appService.AppId]
	if !ok {
		appCache = make(map[string]AppService)
		s.cache[appService.AppId] = appCache
	}

	appCache[appService.Id()] = appService
}

func (s Store) removeFromCache(appService AppService) {
	appCache, ok := s.cache[appService.AppId]
	if !ok {
		return
	}
	delete(appCache, appService.Id())
	if len(appCache) == 0 {
		delete(s.cache, appService.AppId)
	}
}
