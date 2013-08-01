package registrar

import "fmt"

const AnnounceComponentMessageSubject = "vcap.component.announce"
const DiscoverComponentMessageSubject = "vcap.component.discover"
const RouterUnregisterMessageSubject = "router.unregister"
const RouterRegisterMessageSubject = "router.register"
const RouterGreetMessageSubject = "router.greet"
const RouterStartMessageSubject = "router.start"

type AnnounceComponentMessage struct {
	Type        string   `json:"type"`
	Index       uint     `json:"index"`
	Host        string   `json:"host"`
	UUID        string   `json:"uuid"`
	Credentials []string `json:"credentials"`
}

type RouterMessage struct {
	Host string   `json:"host"`
	Port uint32   `json:"port"`
	Uris []string `json:"uris"`
}

func NewAnnounceComponentMessage(cfc *CfComponent) (message *AnnounceComponentMessage) {
	message = &AnnounceComponentMessage{
		Type:        cfc.Type,
		Index:       cfc.Index,
		Host:        fmt.Sprintf("%s:%d", cfc.IpAddress, cfc.StatusPort),
		UUID:        fmt.Sprintf("%d-%s", cfc.Index, cfc.UUID),
		Credentials: cfc.StatusCredentials,
	}

	return message
}
