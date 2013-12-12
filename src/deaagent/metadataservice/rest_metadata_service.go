package metadataservice

import (
	"encoding/json"
	"errors"
	"github.com/cloudfoundry/gosteno"
	"net/http"
)

type metadataResponse struct {
	Index   string `json:"index"`
	AppGUID string `json:"app_guid"`
}

type serviceResponse struct {
	SyslogDrainURI string `json:"syslog_drain_url"`
}

type RestMetaDataService struct {
	*gosteno.Logger
	serviceLocation string
	knownMetaData   map[string]Metadata
}

func NewRestMetaDataService(serviceLocation string, logger *gosteno.Logger) RestMetaDataService {
	knownMetaData := make(map[string]Metadata)
	return RestMetaDataService{logger, serviceLocation, knownMetaData}
}

func (s RestMetaDataService) Lookup(wardenHandle string) (Metadata, error) {

	var appMetaData metadataResponse
	var services []serviceResponse

	metadataResponse, err := http.Get("http://" + s.serviceLocation + "/" + wardenHandle + "/metadata")
	if err != nil {
		return Metadata{}, err
	}

	if metadataResponse.StatusCode >= 400 {
		return Metadata{}, errors.New("Not Found")
	}

	err = json.NewDecoder(metadataResponse.Body).Decode(&appMetaData)
	if err != nil {
		return Metadata{}, err
	}

	servicesResponse, err := http.Get("http://" + s.serviceLocation + "/" + wardenHandle + "/services")
	if err != nil {
		return Metadata{}, err
	}

	if servicesResponse.StatusCode >= 400 {
		return Metadata{}, errors.New("Not Found")
	}

	err = json.NewDecoder(servicesResponse.Body).Decode(&services)
	if err != nil {
		return Metadata{}, err
	}

	metaData := Metadata{appMetaData.Index, appMetaData.AppGUID, drainURIsFromServices(services)}

	return metaData, nil
}

func drainURIsFromServices(services []serviceResponse) []string {
	drainURIs := []string{}

	for _, svc := range services {
		if svc.SyslogDrainURI != "" {
			drainURIs = append(drainURIs, svc.SyslogDrainURI)
		}
	}

	return drainURIs
}
