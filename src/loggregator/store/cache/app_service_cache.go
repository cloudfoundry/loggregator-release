package cache

import "loggregator/domain"

type AppServiceCache struct {
	appServicesByAppId map[string]map[string]domain.AppService
}

func NewAppServiceCache() AppServiceCache {
	return AppServiceCache{make(map[string]map[string]domain.AppService)}
}

func (s AppServiceCache) Add(appService domain.AppService) {
	appServicesById, ok := s.appServicesByAppId[appService.AppId]
	if !ok {
		appServicesById = make(map[string]domain.AppService)
		s.appServicesByAppId[appService.AppId] = appServicesById
	}

	appServicesById[appService.Id()] = appService
}

func (s AppServiceCache) Remove(appService domain.AppService) {
	appCache := s.appServicesByAppId[appService.AppId]
	delete(appCache, appService.Id())
	if len(appCache) == 0 {
		delete(s.appServicesByAppId, appService.AppId)
	}
}

func (s AppServiceCache) RemoveApp(appId string) []domain.AppService {
	appCache := s.appServicesByAppId[appId]
	delete(s.appServicesByAppId, appId)
	return values(appCache)
}

func (s AppServiceCache) Get(appId string) []domain.AppService {
	return values(s.appServicesByAppId[appId])
}

func (s AppServiceCache) Size() int {
	count := 0
	for _, m := range(s.appServicesByAppId) {
		serviceCountForApp := len(m)
		if serviceCountForApp > 0 {
			count += serviceCountForApp
		} else {
			count++
		}
	}
	return count
}

func (s AppServiceCache) Exists(appService domain.AppService) bool {
	serviceExists := false
	appServices, appExists := s.appServicesByAppId[appService.AppId]
	if appExists {
		_, serviceExists = appServices[appService.Id()]
	}
	return serviceExists
}

func values(appCache map[string]domain.AppService) []domain.AppService {
	appServices := make([]domain.AppService, len(appCache))
	i := 0
	for _, appService := range(appCache) {
		appServices[i] = appService
		i++
	}
	return appServices
}
