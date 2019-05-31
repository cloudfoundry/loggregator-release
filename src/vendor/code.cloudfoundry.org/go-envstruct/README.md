# envstruct

[![GoDoc][go-doc-badge]][go-doc] [![travis][travis-badge]][travis] [![slack.cloudfoundry.org][slack-badge]][loggregator-slack]

envstruct is a simple library for populating values on structs from environment
variables.

## Usage

Export some environment variables.

```
$ export HOST_IP="127.0.0.1"
$ export HOST_PORT="443"
$ export PASSWORD="abc123"
```

*Note:* The environment variables are case
sensitive. The casing of the set environment variable must match the casing in
the struct tag.

Write some code. In this example, `Ip` requires that the `HOST_IP` environment
variable is set to non empty value and `Port` defaults to `80` if `HOST_PORT` is
an empty value. Then we use the `envstruct.WriteReport()` to print a table with
a report of what fields are on the struct, the type, the environment variable
where the value is read from, whether or not it is required, and the value.
All values are omitted by default, if you wish to display the value for a
field you can add `report` to the `env` struct tag.

```
package main

import envstruct "code.cloudfoundry.org/go-envstruct"

type Credentials struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

func (c *Credentials) UnmarshalEnv(data string) error {
	return json.Unmarshal([]byte(data), c)
}

type HostInfo struct {
	Credentials Credentials `env:"CREDENTIALS, required"`
	IP          string      `env:"HOST_IP,     required, report"`
	Port        int         `env:"HOST_PORT,             report"`
}

func main() {
	hi := HostInfo{Port: 80}

	err := envstruct.Load(&hi)
	if err != nil {
		panic(err)
	}

	envstruct.WriteReport(&hi)
}
```

Run your code and rejoice!

```
$ go run example/example.go
FIELD NAME:  TYPE:             ENV:         REQUIRED:  VALUE:
Credentials  main.Credentials  CREDENTIALS  true       (OMITTED)
IP           string            HOST_IP      true       10.0.0.1
Port         int               HOST_PORT    false      80
Credentials: {Username:my-user Password:my-password}
```

## Supported Types

- [x] string
- [x] bool (`true` and `1` results in true value, anything else results in false value)
- [x] int
- [x] int8
- [x] int16
- [x] int32
- [x] int64
- [x] uint
- [x] uint8
- [x] uint16
- [x] uint32
- [x] uint64
- [x] float32
- [x] float64
- [x] complex64
- [x] complex128
- [x] []slice (Slices of any other supported type. Environment variable should
  have coma separated values)
- [x] time.Duration
- [x] \*url.URL
- [x] Struct
- [x] Pointer to Struct
- [x] map[string]string (Environment variable should have comma separated
  `key:value`. Keys cannot contain colons and neither key nor value can
  contain commas. e.g. `key_one:value_one, key_two:value_two`
- [x] Custom Unmarshaller (see Credentials in example above)

## Running Tests

Run tests using ginkgo.

```
$ go get github.com/onsi/ginkgo/ginkgo
$ go get github.com/onsi/gomega
$ ginkgo
```

[slack-badge]:       https://slack.cloudfoundry.org/badge.svg
[loggregator-slack]: https://cloudfoundry.slack.com/archives/loggregator
[go-doc-badge]:      https://godoc.org/code.cloudfoundry.org/go-envstruct?status.svg
[go-doc]:            https://godoc.org/code.cloudfoundry.org/go-envstruct
[travis-badge]:      https://travis-ci.org/cloudfoundry/go-envstruct.svg?branch=master
[travis]:            https://travis-ci.org/cloudfoundry/go-envstruct?branch=master
