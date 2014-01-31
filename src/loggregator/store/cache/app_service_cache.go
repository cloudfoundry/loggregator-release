package cache

import "loggregator/domain"

type AppServiceCache struct {
	appServicesByAppId map[string]map[string]domain.AppService
}

func NewAppServiceCache() AppServiceCache {
	return AppServiceCache{make(map[string]map[string]domain.AppService)}
}

func (cache AppServiceCache) Add(appService domain.AppService) {
	appServicesById, ok := cache.appServicesByAppId[appService.AppId]
	if !ok {
		appServicesById = make(map[string]domain.AppService)
		cache.appServicesByAppId[appService.AppId] = appServicesById
	}

	appServicesById[appService.Id()] = appService
}

func (cache AppServiceCache) Remove(appService domain.AppService) {
	appCache := cache.appServicesByAppId[appService.AppId]
	delete(appCache, appService.Id())
	if len(appCache) == 0 {
		delete(cache.appServicesByAppId, appService.AppId)
	}
}

func (cache AppServiceCache) RemoveApp(appId string) []domain.AppService {
	appCache := cache.appServicesByAppId[appId]
	delete(cache.appServicesByAppId, appId)
	return values(appCache)
}

func (cache AppServiceCache) Get(appId string) []domain.AppService {
	return values(cache.appServicesByAppId[appId])
}

func (cache AppServiceCache) Size() int {
	count := 0
	for _, m := range cache.appServicesByAppId {
		serviceCountForApp := len(m)
		if serviceCountForApp > 0 {
			count += serviceCountForApp
		} else {
			count++
		}
	}
	return count
}

func (cache AppServiceCache) Exists(appService domain.AppService) bool {
	serviceExists := false
	appServices, appExists := cache.appServicesByAppId[appService.AppId]
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
