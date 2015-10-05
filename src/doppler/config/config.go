package config

import (
	"doppler/iprange"
	"errors"
	"time"

	"crypto/tls"
	"github.com/cloudfoundry/gosteno"
	"github.com/cloudfoundry/loggregatorlib/cfcomponent"
)

const HeartbeatInterval = 10 * time.Second

type TLSListenerConfig struct {
	Port               uint32
	CrtFile            string
	KeyFile            string
	Cert               tls.Certificate
	InsecureSkipVerify bool
}

type Config struct {
	cfcomponent.Config
	EtcdUrls                      []string
	EtcdMaxConcurrentRequests     int
	Index                         uint
	DropsondeIncomingMessagesPort uint32
	OutgoingPort                  uint32
	LogFilePath                   string
	MaxRetainedLogMessages        uint32
	MessageDrainBufferSize        uint
	SharedSecret                  string
	SkipCertVerify                bool
	BlackListIps                  []iprange.IPRange
	JobName                       string
	Zone                          string
	ContainerMetricTTLSeconds     int
	SinkInactivityTimeoutSeconds  int
	SinkIOTimeoutSeconds          int
	UnmarshallerCount             int
	MetronAddress                 string
	MonitorIntervalSeconds        uint
	SinkDialTimeoutSeconds        int
	EnableTLSTransport            bool
	TLSListenerConfig             *TLSListenerConfig
}

func (c *Config) Validate(logger *gosteno.Logger) (err error) {
	if c.MaxRetainedLogMessages == 0 {
		return errors.New("Need max number of log messages to retain per application")
	}

	if c.BlackListIps != nil {
		err = iprange.ValidateIpAddresses(c.BlackListIps)
		if err != nil {
			return err
		}
	}

	if c.UnmarshallerCount == 0 {
		c.UnmarshallerCount = 1
	}

	if c.EtcdMaxConcurrentRequests < 1 {
		c.EtcdMaxConcurrentRequests = 1
	}

	return err
}
