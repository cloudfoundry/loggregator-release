package logtarget

import "strings"

type LogTarget struct {
	OrgId		string
	SpaceId		string
	AppId		string
}

func (lt *LogTarget) Identifier() string {
	return strings.Join([]string{lt.OrgId, lt.SpaceId, lt.AppId}, ":")
}

func (lt *LogTarget) IsValid() bool {
	return (lt.OrgId != "" && lt.SpaceId != "" && lt.AppId != "") ||
			(lt.OrgId != "" && lt.SpaceId != "" && lt.AppId == "") ||
			(lt.OrgId != "" && lt.SpaceId == "" && lt.AppId == "")
}
