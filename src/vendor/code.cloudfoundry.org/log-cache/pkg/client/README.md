# Log Cache Client

[![GoDoc][go-doc-badge]][go-doc] [![slack.cloudfoundry.org][slack-badge]][log-cache-slack]

This is a golang client library for [Log-Cache][log-cache].

## Usage

This repository should be imported as:

`import logcache "code.cloudfoundry.org/log-cache/pkg/client"`

**NOTE**: This client library is compatible with `log-cache` versions 1.5.x and
above. For versions 1.4.x and below, please use [`go-log-cache`][go-log-cache].

Also, the master branch of `log-cache/pkg/client` reflects current (potentially
development) state of `log-cache-release`. Branches are provided for
compatibility with released versions of `log-cache`.

[slack-badge]:              https://slack.cloudfoundry.org/badge.svg
[log-cache-slack]:          https://cloudfoundry.slack.com/archives/log-cache
[log-cache]:                https://code.cloudfoundry.org/log-cache
[go-doc-badge]:             https://godoc.org/code.cloudfoundry.org/log-cache/client?status.svg
[go-doc]:                   https://godoc.org/code.cloudfoundry.org/log-cache/pkg/client
[go-log-cache]:             https://github.com/cloudfoundry/go-log-cache
