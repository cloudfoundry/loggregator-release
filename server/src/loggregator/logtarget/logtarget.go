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
	target := &LogTarget{appId}
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
		AppId: *receivedMessage.AppId,
	}

	return
}

type LogTarget struct {
	AppId string
}

func (lt *LogTarget) Identifier() string {
	return strings.Join([]string{lt.AppId}, ":")
}

func (lt *LogTarget) IsValid() bool {
	return lt.AppId != ""
}

func (lt *LogTarget) String() string {
	return fmt.Sprintf("App: %s", lt.AppId)
}
