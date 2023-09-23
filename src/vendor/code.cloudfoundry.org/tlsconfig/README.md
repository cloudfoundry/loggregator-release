# tlsconfig

[![Go Report Card](https://goreportcard.com/badge/code.cloudfoundry.org/tlsconfig)](https://goreportcard.com/report/code.cloudfoundry.org/tlsconfig)
[![Go Reference](https://pkg.go.dev/badge/code.cloudfoundry.org/tlsconfig.svg)](https://pkg.go.dev/code.cloudfoundry.org/tlsconfig)

tlsconfig generates shared [crypto/tls configurations](https://pkg.go.dev/crypto/tls#Config) for internal and external-facing services in Cloud Foundry. This module is considered internal to Cloud Foundry, and does not provide any stability guarantees for external usage.

## Getting Started

### Usage

Import this module as `code.cloudfoundry.org/tlsconfig`.

Update to the latest version of the library off the main branch with:
```
go get -u code.cloudfoundry.org/tlsconfig@main
```

### Running the tests

All the tests use the standard go testing library and can be run with:
```
go test ./...
```

## Contributing

Cloud Foundry uses GitHub to manage reviews of pull requests and issues.

* If you have a trivial fix or improvement, go ahead and create a pull request.
* If you plan to do something more involved, first discuss your ideas in [Slack](cloudfoundry.slack.com). This will help avoid unnecessary work :).
* Make sure you've signed the CLA!

## Versioning

This module is not currently versioned. Whatever is on the `main` branch is considered to be the latest release of the module.

## License

This project is licensed under the Apache License 2.0 - see the [LICENSE](LICENSE) file for details.
