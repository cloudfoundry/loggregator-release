module code.cloudfoundry.org/loggregator

go 1.21.0

toolchain go1.21.9

require (
	code.cloudfoundry.org/go-batching v0.0.0-20240325232529-c21ea48767e2
	code.cloudfoundry.org/go-diodes v0.0.0-20240325171903-fa19631aa7ba
	code.cloudfoundry.org/go-envstruct v1.7.0
	code.cloudfoundry.org/go-log-cache v1.0.1-0.20220808235537-54ad6006c0c4
	code.cloudfoundry.org/go-loggregator/v9 v9.2.0
	code.cloudfoundry.org/go-metric-registry v0.0.0-20240325232813-eb1144b007e4
	code.cloudfoundry.org/go-pubsub v0.0.0-20240220213308-3a13ece7c8f3
	code.cloudfoundry.org/tlsconfig v0.0.0-20240417163319-a2cf10de323a
	github.com/cloudfoundry/noaa/v2 v2.4.0
	github.com/cloudfoundry/sonde-go v0.0.0-20240311165458-423aa0d4dfc8
	github.com/gorilla/handlers v1.5.2
	github.com/gorilla/mux v1.8.1
	github.com/gorilla/websocket v1.5.1
	github.com/onsi/ginkgo v1.16.5
	github.com/onsi/gomega v1.33.0
	github.com/prometheus/client_golang v1.19.0
	golang.org/x/net v0.24.0
	google.golang.org/grpc v1.63.2
)

require (
	filippo.io/edwards25519 v1.1.0 // indirect
	github.com/elazarl/goproxy v0.0.0-20230731152917-f99041a5c027 // indirect
	github.com/pkg/errors v0.9.1 // indirect
	go.step.sm/crypto v0.44.6 // indirect
	golang.org/x/crypto v0.22.0 // indirect
	google.golang.org/genproto/googleapis/api v0.0.0-20240415180920-8c6c420018be // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20240415180920-8c6c420018be // indirect
)

require (
	github.com/beorn7/perks v1.0.1 // indirect
	github.com/blang/semver v3.5.1+incompatible // indirect
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/felixge/httpsnoop v1.0.4 // indirect
	github.com/fsnotify/fsnotify v1.7.0 // indirect
	github.com/go-task/slim-sprig v0.0.0-20230315185526-52ccab3ef572 // indirect
	github.com/google/go-cmp v0.6.0 // indirect
	github.com/google/pprof v0.0.0-20230912144702-c363fe2c2ed8 // indirect
	github.com/grpc-ecosystem/grpc-gateway/v2 v2.19.1 // indirect
	github.com/nxadm/tail v1.4.11 // indirect
	github.com/prometheus/client_model v0.6.1 // indirect
	github.com/prometheus/common v0.53.0 // indirect
	github.com/prometheus/procfs v0.13.0 // indirect
	github.com/square/certstrap v1.3.0 // indirect
	golang.org/x/sys v0.19.0 // indirect
	golang.org/x/text v0.14.0 // indirect
	golang.org/x/tools v0.20.0 // indirect
	google.golang.org/protobuf v1.33.0
	gopkg.in/tomb.v1 v1.0.0-20141024135613-dd632973f1e7 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

replace github.com/cloudfoundry/noaa/v2 => github.com/cloudfoundry/noaa/v2 v2.2.0 // for recent logs

replace github.com/gorilla/websocket => github.com/gorilla/websocket v1.5.0 // 1.5.1 is broken, 1.5.2 should work when they cut it
