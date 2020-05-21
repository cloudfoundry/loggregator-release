module code.cloudfoundry.org/loggregator

go 1.12

require (
	code.cloudfoundry.org/go-batching v0.0.0-20171020220229-924d2a9b48ac
	code.cloudfoundry.org/go-diodes v0.0.0-20190809170250-f77fb823c7ee
	code.cloudfoundry.org/go-envstruct v1.5.0
	code.cloudfoundry.org/go-loggregator v0.0.0-20190809213911-969cb33bee6a // pinned
	code.cloudfoundry.org/go-pubsub v0.0.0-20180503211407-becd51dc37cb
	code.cloudfoundry.org/grpc-throughputlb v0.0.0-20180905204614-e98a1ee09867
	code.cloudfoundry.org/log-cache v2.3.1+incompatible
	code.cloudfoundry.org/tlsconfig v0.0.0-20200131000646-bbe0f8da39b3
	github.com/apoydence/onpar v0.0.0-20190519213022-ee068f8ea4d1 // indirect; pinned
	github.com/cloudfoundry/noaa v2.1.0+incompatible // pinned
	github.com/cloudfoundry/sonde-go v0.0.0-20171206171820-b33733203bb4 // pinned
	github.com/elazarl/goproxy v0.0.0-20200426045556-49ad98f6dac1 // indirect
	github.com/elazarl/goproxy/ext v0.0.0-20200426045556-49ad98f6dac1 // indirect
	github.com/gogo/protobuf v1.3.1 // pinned
	github.com/golang/protobuf v1.4.2 // pinned
	github.com/gorilla/handlers v1.4.2
	github.com/gorilla/mux v1.7.4
	github.com/gorilla/websocket v1.4.2
	github.com/grpc-ecosystem/grpc-gateway v1.14.5 // indirect
	github.com/kr/pretty v0.2.0 // indirect
	github.com/kr/text v0.2.0 // indirect
	github.com/mailru/easyjson v0.7.1 // indirect
	github.com/onsi/ginkgo v1.12.2
	github.com/onsi/gomega v1.10.1
	github.com/poy/onpar v0.0.0-20190519213022-ee068f8ea4d1 // indirect; pinned
	github.com/prometheus/client_golang v1.6.0
	golang.org/x/net v0.0.0-20200520182314-0ba52f642ac2
	google.golang.org/genproto v0.0.0-20200521103424-e9a78aa275b7 // indirect
	google.golang.org/grpc v1.29.1
)

replace code.cloudfoundry.org/log-cache => code.cloudfoundry.org/log-cache-release/src v0.0.0-20191002191123-8c97ac62a57b
