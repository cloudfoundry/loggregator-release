package cache

import (
	"loggregator/domain"
	"sync"
)

type AppServiceCache interface {
	Add(appService domain.AppService)
	Remove(appService domain.AppService)
	RemoveApp(appId string) []domain.AppService

	Get(appId string) []domain.AppService
	Exists(appService domain.AppService) bool
}

type AppServiceWatcherCache interface {
	AppServiceCache
	GetAll() []domain.AppService
	Size() int
}

type appServiceCache struct {
	sync.RWMutex
	appServicesByAppId map[string]map[string]domain.AppService
}

func NewAppServiceCache() AppServiceWatcherCache {
	c := &appServiceCache{appServicesByAppId: make(map[string]map[string]domain.AppService)}
	return c
}

func (c *appServiceCache) Add(appService domain.AppService) {
	c.Lock()
	defer c.Unlock()
	appServicesById, ok := c.appServicesByAppId[appService.AppId]
	if !ok {
		appServicesById = make(map[string]domain.AppService)
		c.appServicesByAppId[appService.AppId] = appServicesById
	}

	appServicesById[appService.Id()] = appService
}

func (c *appServiceCache) Remove(appService domain.AppService) {
	c.Lock()
	defer c.Unlock()
	appCache := c.appServicesByAppId[appService.AppId]
	delete(appCache, appService.Id())
	if len(appCache) == 0 {
		delete(c.appServicesByAppId, appService.AppId)
	}
}

func (c *appServiceCache) RemoveApp(appId string) []domain.AppService {
	c.Lock()
	defer c.Unlock()
	appCache := c.appServicesByAppId[appId]
	delete(c.appServicesByAppId, appId)
	return values(appCache)
}

func (c *appServiceCache) Get(appId string) []domain.AppService {
	c.RLock()
	defer c.RUnlock()
	return values(c.appServicesByAppId[appId])
}

func (c *appServiceCache) GetAll() []domain.AppService {
	c.RLock()
	defer c.RUnlock()
	var result []domain.AppService
	for _, appServices := range c.appServicesByAppId {
		result = append(result, values(appServices)...)
	}
	return result
}

func (c *appServiceCache) Size() int {
	c.RLock()
	defer c.RUnlock()
	count := 0
	for _, m := range c.appServicesByAppId {
		serviceCountForApp := len(m)
		if serviceCountForApp > 0 {
			count += serviceCountForApp
		} else {
			count++
		}
	}
	return count
}

func (c *appServiceCache) Exists(appService domain.AppService) bool {
	c.RLock()
	defer c.RUnlock()
	serviceExists := false
	appServices, appExists := c.appServicesByAppId[appService.AppId]
	if appExists {
		_, serviceExists = appServices[appService.Id()]
	}
	return serviceExists
}

func values(appCache map[string]domain.AppService) []domain.AppService {
	appServices := make([]domain.AppService, len(appCache))
	i := 0
	for _, appService := range appCache {
		appServices[i] = appService
		i++
	}
	return appServices
}
