package collectorregistrar

import (
	"cfcomponent"
	"fmt"
)

const AnnounceComponentMessageSubject = "vcap.component.announce"
const DiscoverComponentMessageSubject = "vcap.component.discover"

type AnnounceComponentMessage struct {
	Type        string   `json:"type"`
	Index       uint     `json:"index"`
	Host        string   `json:"host"`
	UUID        string   `json:"uuid"`
	Credentials []string `json:"credentials"`
}

func NewAnnounceComponentMessage(cfc cfcomponent.Component) (message *AnnounceComponentMessage) {
	message = &AnnounceComponentMessage{
		Type:        cfc.Type,
		Index:       cfc.Index,
		Host:        fmt.Sprintf("%s:%d", cfc.IpAddress, cfc.StatusPort),
		UUID:        fmt.Sprintf("%d-%s", cfc.Index, cfc.UUID),
		Credentials: cfc.StatusCredentials,
	}

	return message
}
