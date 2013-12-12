package metadataservice

import (
	"encoding/json"
	"errors"
	"github.com/cloudfoundry/loggregatorlib/loggertesthelper"
	"github.com/stretchr/testify/assert"
	"net"
	"net/http"
	"strings"
	"sync"
	"testing"
)

type FakeMetadataServer struct {
	containers map[string]FakeContainer

	sync.RWMutex
}

type FakeContainer struct {
	Metadata FakeMetadata

	Services []FakeService
}

type FakeMetadata struct {
	Index   string `json:"index"`
	AppGUID string `json:"app_guid"`
}

type FakeService struct {
	SyslogDrainURI string `json:"syslog_drain_url"`
}

func NewFakeMetadataServer() *FakeMetadataServer {
	return &FakeMetadataServer{
		containers: make(map[string]FakeContainer),
	}
}

func (s *FakeMetadataServer) AddContainer(handle string, metadata FakeContainer) {
	s.Lock()
	defer s.Unlock()

	s.containers[handle] = metadata
}

func (s *FakeMetadataServer) RemoveContainer(handle string) {
	s.Lock()
	defer s.Unlock()

	delete(s.containers, handle)
}

func (s *FakeMetadataServer) GetContainer(handle string) (FakeContainer, bool) {
	s.RLock()
	defer s.RUnlock()

	metadata, found := s.containers[handle]

	return metadata, found
}

func (s *FakeMetadataServer) Start() (net.Listener, error) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, err
	}

	server := &http.Server{Handler: s}
	go server.Serve(listener)

	return listener, err
}

func (s *FakeMetadataServer) ServeHTTP(writer http.ResponseWriter, req *http.Request) {
	pathSegments := strings.Split(req.URL.Path, "/")
	if len(pathSegments) < 3 {
		writer.WriteHeader(400)
		return
	}

	handle := pathSegments[1]
	requestedAttribute := pathSegments[2]

	switch requestedAttribute {
	case "metadata":
		s.handleMetadata(writer, handle)
	case "services":
		s.handleServices(writer, handle)
	}
}

func (s *FakeMetadataServer) handleMetadata(writer http.ResponseWriter, handle string) {
	if handle == "bad_metadata_json" {
		writer.Write([]byte("this aint json"))
		return
	}
	jsonEncoder := json.NewEncoder(writer)

	if strings.Contains(handle, "services") {
		jsonEncoder.Encode(Metadata{})
		return
	}

	container, found := s.GetContainer(handle)
	if !found {
		writer.WriteHeader(404)
		return
	}

	jsonEncoder.Encode(container.Metadata)
}

func (s *FakeMetadataServer) handleServices(writer http.ResponseWriter, handle string) {
	if handle == "bad_services_json" {
		writer.Write([]byte("this aint json"))
		return
	}

	container, found := s.GetContainer(handle)
	if !found {
		writer.WriteHeader(404)
		return
	}

	jsonEncoder := json.NewEncoder(writer)
	jsonEncoder.Encode(container.Services)
}

var ServerEndpoint string

func init() {
	fakeServer := NewFakeMetadataServer()

	fakeMetaData := FakeMetadata{"12", "appguid"}
	fakeService := FakeService{"syslog://drains.co"}
	fakeService2 := FakeService{"syslog://brains.co"}
	fakeContainer := FakeContainer{fakeMetaData, []FakeService{fakeService, fakeService2}}

	fakeServer.AddContainer("warden_handle", fakeContainer)

	listener, _ := fakeServer.Start()

	ServerEndpoint = listener.Addr().String()
}

func TestRestMetaDataServiceReturnsValidMetadata(t *testing.T) {

	metaDataService := NewRestMetaDataService(ServerEndpoint, loggertesthelper.Logger())

	Metadata, err := metaDataService.Lookup("warden_handle")
	assert.NoError(t, err)

	assert.Equal(t, Metadata.Guid, "appguid")
	assert.Equal(t, Metadata.Index, "12")
	assert.Equal(t, Metadata.SyslogDrainUrls, []string{"syslog://drains.co", "syslog://brains.co"})
}

func TestReturnsAppropriateErrorWhenMetadataForWardenHandleIsNotFound(t *testing.T) {

	metaDataService := NewRestMetaDataService(ServerEndpoint, loggertesthelper.Logger())

	_, err := metaDataService.Lookup("nonexistent_metadata_warden_handle")
	assert.Equal(t, errors.New("Not Found"), err)
}

func TestReturnsAppropriateErrorWhenMetadataServiceReturnsBadJson(t *testing.T) {

	metaDataService := NewRestMetaDataService(ServerEndpoint, loggertesthelper.Logger())

	_, err := metaDataService.Lookup("bad_metadata_json")
	assert.IsType(t, &json.SyntaxError{}, err)
}

func TestReturnsAppropriateErrorWhenServiceForWardenHandleIsNotFound(t *testing.T) {

	metaDataService := NewRestMetaDataService(ServerEndpoint, loggertesthelper.Logger())

	_, err := metaDataService.Lookup("nonexistent_services_warden_handle")
	assert.Equal(t, errors.New("Not Found"), err)
}

func TestReturnsAppropriateErrorWhenServiceServiceReturnsBadJson(t *testing.T) {

	metaDataService := NewRestMetaDataService(ServerEndpoint, loggertesthelper.Logger())

	_, err := metaDataService.Lookup("bad_services_json")
	assert.IsType(t, &json.SyntaxError{}, err)
}
