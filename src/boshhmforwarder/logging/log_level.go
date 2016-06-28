package logging

import (
	"bytes"
	"fmt"
)

type LogLevel int

const (
	SILENT LogLevel = iota
	FATAL
	ERROR
	WARNING
	INFO
	DEBUG
)

func (l LogLevel) String() string {
	switch l {
	case FATAL:
		return "FATAL"
	case ERROR:
		return "ERROR"
	case WARNING:
		return "WARNING"
	case INFO:
		return "INFO"
	case DEBUG:
		return "DEBUG"
	default:
		return "INVALID"
	}
}

func (l LogLevel) MarshalJSON() ([]byte, error) {
	return []byte("\"" + l.String() + "\""), nil
}

func (l *LogLevel) UnmarshalJSON(data []byte) error {
	data = bytes.Trim(data, `"`)
	strData := string(data)
	switch strData {
	case "FATAL":
		*l = FATAL
	case "ERROR":
		*l = ERROR
	case "WARNING":
		*l = WARNING
	case "INFO":
		*l = INFO
	case "DEBUG":
		*l = DEBUG
	default:
		return fmt.Errorf("Unknown LogLevel: %s", strData)
	}
	return nil
}

func ParseLogLevel(level string) LogLevel {
	switch level {
	case "SILENT":
		return SILENT
	case "FATAL":
		return FATAL
	case "ERROR":
		return ERROR
	case "WARNING":
		return WARNING
	case "INFO":
		return INFO
	case "DEBUG":
		return DEBUG
	default:
		panic(fmt.Sprintf("Unknown LogLevel: %s", level))
	}
}
