package legacyproxy

import (
	"github.com/cloudfoundry/gosteno"
	"github.com/cloudfoundry/loggregatorlib/cfcomponent"
	"net/http"
)

type Proxy struct {
	handler http.Handler
	cfcomponent.Component
	requestTranslator RequestTranslator
	logger            *gosteno.Logger
}

type ProxyBuilder struct {
	handler           http.Handler
	component         cfcomponent.Component
	requestTranslator RequestTranslator
	logger            *gosteno.Logger
}

func NewProxyBuilder() *ProxyBuilder {
	return &ProxyBuilder{
		logger: gosteno.NewLogger("dummy logger"),
	}
}

func (proxyBuilder *ProxyBuilder) Handler(handler http.Handler) *ProxyBuilder {
	proxyBuilder.handler = handler
	return proxyBuilder
}

func (proxyBuilder *ProxyBuilder) Component(component cfcomponent.Component) *ProxyBuilder {
	proxyBuilder.component = component
	return proxyBuilder
}

func (proxyBuilder *ProxyBuilder) RequestTranslator(requestTranslator RequestTranslator) *ProxyBuilder {
	proxyBuilder.requestTranslator = requestTranslator
	return proxyBuilder
}

func (proxyBuilder *ProxyBuilder) Logger(logger *gosteno.Logger) *ProxyBuilder {
	proxyBuilder.logger = logger
	return proxyBuilder
}

func (proxyBuilder *ProxyBuilder) Build() *Proxy {
	return &Proxy{
		handler:           proxyBuilder.handler,
		Component:         proxyBuilder.component,
		requestTranslator: proxyBuilder.requestTranslator,
		logger:            proxyBuilder.logger,
	}
}

func (proxy *Proxy) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	request, err := proxy.requestTranslator.Translate(request)
	if err != nil {
		proxy.logger.Errorf("proxy: error translating request: %v", err)
		http.Error(writer, "invalid request: "+err.Error(), 400)
		return
	}
	proxy.handler.ServeHTTP(writer, request)
}
