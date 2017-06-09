package main

import (
	"net/url"

	"code.cloudfoundry.org/loggregator/syslog_drain_binder/shared_types"
)

func Filter(bindings shared_types.AllSyslogDrainBindings) shared_types.AllSyslogDrainBindings {
	newBindings := make(shared_types.AllSyslogDrainBindings)
	for appId, b := range bindings {
		drainUrls := []string{}
		for _, d := range b.DrainURLs {
			url, err := url.Parse(d)
			if err != nil {
				continue
			}
			if url.Query().Get("drain-version") != "2.0" {
				drainUrls = append(drainUrls, d)
			}
		}
		if len(drainUrls) > 0 {
			binding := shared_types.SyslogDrainBinding{
				Hostname:  b.Hostname,
				DrainURLs: drainUrls,
			}
			newBindings[appId] = binding
		}
	}
	return newBindings
}
