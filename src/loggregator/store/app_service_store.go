package store

import (
	"github.com/cloudfoundry/storeadapter"
	"path"

	"loggregator/domain"
	"loggregator/store/cache"
)

type AppServiceStore struct {
	adapter                   storeadapter.StoreAdapter
	outAddChan, outRemoveChan chan<- domain.AppService
	incomingChan              <-chan domain.AppServices
	cache                     cache.AppServiceCache
}

func NewAppServiceStore(adapter storeadapter.StoreAdapter, outAdd, outRemove chan<- domain.AppService, in <-chan domain.AppServices) *AppServiceStore {
	return &AppServiceStore{
		adapter:       adapter,
		outAddChan:    outAdd,
		outRemoveChan: outRemove,
		incomingChan:  in,
		cache:         cache.NewAppServiceCache(),
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
			appService := domain.AppService{AppId: appId, Url: string(serviceNode.Value)}
			s.cache.Add(appService)
			s.outAddChan <- appService
		}
	}

	for appServices := range s.incomingChan {
		cachedAppServices := s.cache.Get(appServices.AppId)

		if len(appServices.Urls) == 0 {
			s.removeAppFromStore(appServices.AppId)
			continue
		}

		appServiceToAdd := []domain.AppService{}
		appServiceToRemove := []domain.AppService{}
		serviceUrls := make(map[string]bool)

		for _, serviceUrl := range appServices.Urls {
			serviceUrls[serviceUrl] = true

			appService := domain.AppService{AppId: appServices.AppId, Url: serviceUrl}
			ok := s.cache.Exists(appService)
			if !ok {
				appServiceToAdd = append(appServiceToAdd, appService)
			}
		}

		for _, appService := range cachedAppServices {
			if !serviceUrls[appService.Url] {
				appServiceToRemove = append(appServiceToRemove, appService)
			}
		}

		s.addToStore(appServiceToAdd)
		s.removeFromStore(appServiceToRemove)
	}
}

func (s AppServiceStore) addToStore(appServices []domain.AppService) {
	nodes := make([]storeadapter.StoreNode, len(appServices))
	for i, appService := range appServices {
		s.cache.Add(appService)
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

func (s AppServiceStore) removeFromStore(appServices []domain.AppService) {
	keys := make([]string, len(appServices))
	for i, appService := range appServices {
		s.cache.Remove(appService)
		keys[i] = path.Join("/loggregator/services", appService.AppId, appService.Id())
	}

	s.adapter.Delete(keys...)
	for _, appService := range appServices {
		s.outRemoveChan <- appService
	}
}

func (s AppServiceStore) removeAppFromStore(appId string) {
	s.adapter.Delete(path.Join("/loggregator/services", appId))
	appServices := s.cache.RemoveApp(appId)
	for _, appService := range appServices {
		s.outRemoveChan <- appService
	}
}
