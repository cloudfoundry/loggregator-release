package logtarget

import (
	"code.google.com/p/gogoprotobuf/proto"
	"errors"
	"fmt"
	"logmessage"
	"net/url"
	"strings"
)

func FromUrl(u *url.URL) *LogTarget {
	appId := u.Query().Get("app")
	spaceId := u.Query().Get("space")
	orgId := u.Query().Get("org")
	target := &LogTarget{orgId, spaceId, appId}
	return target
}

func FromLogMessage(data []byte) (lt *LogTarget, err error) {
	receivedMessage := &logmessage.LogMessage{}
	err = proto.Unmarshal(data, receivedMessage)
	if err != nil {
		err = errors.New(fmt.Sprintf("Log message could not be unmarshaled. Dropping it... Error: %v. Data: %v", err, data))
		return
	}

	lt = &LogTarget{
		OrgId:   *receivedMessage.OrganizationId,
		SpaceId: *receivedMessage.SpaceId,
		AppId:   *receivedMessage.AppId,
	}

	return
}

type LogTarget struct {
	OrgId   string
	SpaceId string
	AppId   string
}

func (lt *LogTarget) Identifier() string {
	return strings.Join([]string{lt.OrgId, lt.SpaceId, lt.AppId}, ":")
}

func (lt *LogTarget) IsValid() bool {
	return (lt.OrgId != "" && lt.SpaceId != "" && lt.AppId != "") ||
		(lt.OrgId != "" && lt.SpaceId != "" && lt.AppId == "") ||
		(lt.OrgId != "" && lt.SpaceId == "" && lt.AppId == "")
}

func (lt *LogTarget) String() string {
	return fmt.Sprintf("Org: %s, Space: %s, App: %s", lt.OrgId, lt.SpaceId, lt.AppId)
}
