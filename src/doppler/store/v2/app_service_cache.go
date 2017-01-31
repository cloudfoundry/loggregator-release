package v2

import (
	"doppler/store"
	"sync"
)

type AppServiceCache interface {
	Add(appService store.AppService)
	Remove(appService store.AppService)
	RemoveApp(appId string) []store.AppService

	Get(appId string) []store.AppService
	Exists(AppService store.AppService) bool
}

type AppServiceWatcherCache interface {
	AppServiceCache
	GetAll() []store.AppService
	Size() int
}

type appServiceCache struct {
	sync.RWMutex
	appServicesByAppId map[string]map[string]store.AppService
}

func NewAppServiceCache() AppServiceWatcherCache {
	c := &appServiceCache{appServicesByAppId: make(map[string]map[string]store.AppService)}
	return c
}

func (c *appServiceCache) Add(appService store.AppService) {
	c.Lock()
	defer c.Unlock()
	appServicesById, ok := c.appServicesByAppId[appService.AppId()]
	if !ok {
		appServicesById = make(map[string]store.AppService)
		c.appServicesByAppId[appService.AppId()] = appServicesById
	}

	appServicesById[appService.Id()] = appService
}

func (c *appServiceCache) Remove(appService store.AppService) {
	c.Lock()
	defer c.Unlock()
	appCache := c.appServicesByAppId[appService.AppId()]
	delete(appCache, appService.Id())
	if len(appCache) == 0 {
		delete(c.appServicesByAppId, appService.AppId())
	}
}

func (c *appServiceCache) RemoveApp(appId string) []store.AppService {
	c.Lock()
	defer c.Unlock()
	appCache := c.appServicesByAppId[appId]
	delete(c.appServicesByAppId, appId)
	return values(appCache)
}

func (c *appServiceCache) Get(appId string) []store.AppService {
	c.RLock()
	defer c.RUnlock()
	return values(c.appServicesByAppId[appId])
}

func (c *appServiceCache) GetAll() []store.AppService {
	c.RLock()
	defer c.RUnlock()
	var result []store.AppService
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

func (c *appServiceCache) Exists(appService store.AppService) bool {
	c.RLock()
	defer c.RUnlock()
	serviceExists := false
	appServices, appExists := c.appServicesByAppId[appService.AppId()]
	if appExists {
		_, serviceExists = appServices[appService.Id()]
	}
	return serviceExists
}

func values(appCache map[string]store.AppService) []store.AppService {
	appServices := make([]store.AppService, len(appCache))
	i := 0
	for _, appService := range appCache {
		appServices[i] = appService
		i++
	}
	return appServices
}
