package syslogreader

import (
	"bytes"
	"code.google.com/p/gogoprotobuf/proto"
	"deaagent/metadataservice"
	"errors"
	"github.com/cloudfoundry/loggregatorlib/logmessage"
	"strings"
	"time"
)

func ReadMessage(metaDataService metadataservice.MetaDataService, input []byte) (*logmessage.LogMessage, error) {
	buffer := bytes.NewBuffer(input)

	priority, err := ReadStringWithoutTrailingCharacter(buffer, '>')
	if err != nil {
		return nil, err
	}

	priority = strings.TrimLeft(priority, "<")

	timestamp, err := ReadStringWithoutTrailingCharacter(buffer, ' ')
	if err != nil {
		return nil, err
	}

	// hostname
	_, err = buffer.ReadBytes(' ')
	if err != nil {
		return nil, err
	}

	tags, err := ReadStringWithoutTrailingCharacter(buffer, '[')
	if err != nil {
		return nil, err
	}

	// pid
	_, err = buffer.ReadBytes(' ')
	if err != nil {
		return nil, err
	}

	message := buffer.Next(buffer.Len())
	if err != nil {
		return nil, err
	}

	messageTime, err := time.Parse(time.RFC3339, timestamp)
	if err != nil {
		return nil, err
	}

	tagSegments := strings.Split(tags, ".")
	if len(tagSegments) != 3 {
		return nil, errors.New("Invalid Tags")
	}

	wardenHandle := tagSegments[len(tagSegments)-1]
	sourceName := tagSegments[len(tagSegments)-2]

	appMetaData, err := metaDataService.Lookup(wardenHandle)
	if err != nil {
		return nil, err
	}

	messageType, err := parseMessageType(priority)
	if err != nil {
		return nil, err
	}

	return &logmessage.LogMessage{
		Message:     bytes.TrimRight(message, "\n"),
		AppId:       proto.String(appMetaData.Guid),
		DrainUrls:   appMetaData.SyslogDrainUrls,
		MessageType: &messageType,
		SourceName:  proto.String(sourceName),
		SourceId:    &appMetaData.Index,
		Timestamp:   proto.Int64(messageTime.UnixNano()),
	}, nil

}

func ReadBytesWithoutTrailingCharacter(b *bytes.Buffer, delim byte) (line []byte, err error) {
	line, err = b.ReadBytes(delim)
	if err != nil {
		return nil, err
	}
	return line[:len(line)-1], nil
}

func ReadStringWithoutTrailingCharacter(b *bytes.Buffer, delim byte) (line string, err error) {
	bytesLine, err := ReadBytesWithoutTrailingCharacter(b, delim)
	return string(bytesLine), err
}

func parseMessageType(priority string) (logmessage.LogMessage_MessageType, error) {
	switch priority {
	case "11":
		return logmessage.LogMessage_ERR, nil
	case "14":
		return logmessage.LogMessage_OUT, nil
	}
	return logmessage.LogMessage_ERR, errors.New("Invalid Priority")
}
