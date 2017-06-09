package shared_types

// AppID represents an app ID.
type AppID string

// DrainURL represents a drain URL.
type DrainURL string

type SyslogDrainBinding struct {
	Hostname  string   `json:"hostname"`
	DrainURLs []string `json:"drains"`
}

type AllSyslogDrainBindings map[AppID]SyslogDrainBinding
