package logging

import (
	"os"

	gologging "github.com/op/go-logging"
)

const (
	defaultModule = "default"
)

var (
	backends []gologging.Backend
	Log      Logger
	format   gologging.Formatter
)

func init() {

	Log = Logger{
		log: gologging.MustGetLogger(defaultModule),
	}
	Log.log.ExtraCalldepth = 1
	format = gologging.MustStringFormatter("%{time:2006-01-02T15:04:05.000000Z} %{shortfunc:.6s} %{level}: %{message}")
	backend := gologging.NewLogBackend(os.Stdout, "", 0)
	backendFormatter := gologging.NewBackendFormatter(backend, format)
	leveledBackend := gologging.AddModuleLevel(backendFormatter)
	backends = append(backends, leveledBackend)
	gologging.SetBackend(backends...)
	SetLevel(SILENT)
}

func SetSysLogger(syslog string) {
	syslogBackend, err := gologging.NewSyslogBackend(syslog)
	if err != nil {
		panic(err)
	}
	backendFormatter := gologging.NewBackendFormatter(syslogBackend, format)
	syslogLeveledBackend := gologging.AddModuleLevel(backendFormatter)
	backends = append(backends, syslogLeveledBackend)

	gologging.SetBackend(backends...)
}

type Logger struct {
	log *gologging.Logger
}

func SetLevel(level LogLevel) {
	for _, backend := range backends {
		leveled := backend.(gologging.LeveledBackend)
		leveled.SetLevel(convertLogLevel(level), "")
	}
}

func (l Logger) Error(msg string, err error) {
	// NOTICE: go vet doesn't like the following line:
	//   l.log.Error("%s : %v", msg, err)
	// Therefore:
	l.log.Error(""+"%s : %v", msg, err)
}

func (l Logger) Errorf(msg string, values ...interface{}) {
	l.log.Errorf(msg, values...)
}

func (l Logger) Info(msg string) {
	l.log.Info(msg)
}

func (l Logger) Infof(msg string, values ...interface{}) {
	l.log.Infof(msg, values...)
}

func (l Logger) Debug(msg string) {
	l.log.Debug(msg)
}

func (l Logger) Debugf(msg string, values ...interface{}) {
	l.log.Debugf(msg, values...)
}

func (l Logger) Warning(msg string) {
	l.log.Warning(msg)
}

func (l Logger) Warningf(msg string, values ...interface{}) {
	l.log.Warningf(msg, values...)
}

func (l Logger) Panic(msg string, err error) {
	l.log.Panicf("%s : %v", msg, err)
}

func (l Logger) Panicf(msg string, values ...interface{}) {
	l.log.Panicf(msg, values...)
}

func convertLogLevel(level LogLevel) gologging.Level {
	switch level {
	case SILENT:
		return -1
	case FATAL:
		return gologging.CRITICAL
	case ERROR:
		return gologging.ERROR
	case WARNING:
		return gologging.WARNING
	case INFO:
		return gologging.INFO
	case DEBUG:
		return gologging.DEBUG
	default:
		Log.Panicf("Unknown LogLevel: %v", level)
		return gologging.CRITICAL
	}
}
