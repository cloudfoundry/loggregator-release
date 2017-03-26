package store

import (
	"sync"
)

type AppServiceCache interface {
	Add(appService AppService)
	Remove(appService AppService)
	RemoveApp(appId string) []AppService

	Get(appId string) []AppService
	Exists(AppService AppService) bool
}

type AppServiceWatcherCache interface {
	AppServiceCache
	GetAll() []AppService
	Size() int
}

type appServiceCache struct {
	sync.RWMutex
	appServicesByAppId map[string]map[string]AppService
}

func NewAppServiceCache() AppServiceWatcherCache {
	c := &appServiceCache{appServicesByAppId: make(map[string]map[string]AppService)}
	return c
}

func (c *appServiceCache) Add(appService AppService) {
	c.Lock()
	defer c.Unlock()
	appServicesById, ok := c.appServicesByAppId[appService.AppId()]
	if !ok {
		appServicesById = make(map[string]AppService)
		c.appServicesByAppId[appService.AppId()] = appServicesById
	}

	appServicesById[appService.Id()] = appService
}

func (c *appServiceCache) Remove(appService AppService) {
	c.Lock()
	defer c.Unlock()
	appCache := c.appServicesByAppId[appService.AppId()]
	delete(appCache, appService.Id())
	if len(appCache) == 0 {
		delete(c.appServicesByAppId, appService.AppId())
	}
}

func (c *appServiceCache) RemoveApp(appId string) []AppService {
	c.Lock()
	defer c.Unlock()
	appCache := c.appServicesByAppId[appId]
	delete(c.appServicesByAppId, appId)
	return values(appCache)
}

func (c *appServiceCache) Get(appId string) []AppService {
	c.RLock()
	defer c.RUnlock()
	return values(c.appServicesByAppId[appId])
}

func (c *appServiceCache) GetAll() []AppService {
	c.RLock()
	defer c.RUnlock()
	var result []AppService
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

func (c *appServiceCache) Exists(appService AppService) bool {
	c.RLock()
	defer c.RUnlock()
	serviceExists := false
	appServices, appExists := c.appServicesByAppId[appService.AppId()]
	if appExists {
		_, serviceExists = appServices[appService.Id()]
	}
	return serviceExists
}

func values(appCache map[string]AppService) []AppService {
	appServices := make([]AppService, len(appCache))
	i := 0
	for _, appService := range appCache {
		appServices[i] = appService
		i++
	}
	return appServices
}
