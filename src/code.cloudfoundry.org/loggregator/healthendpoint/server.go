package healthendpoint

import (
	"log"
	"net"
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func StartServer(addr string, gatherer prometheus.Gatherer) net.Listener {
	router := http.NewServeMux()
	router.Handle("/health", promhttp.HandlerFor(gatherer, promhttp.HandlerOpts{}))

	server := http.Server{
		Addr:         addr,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 5 * time.Second,
		Handler:      router,
	}

	lis, err := net.Listen("tcp", addr)
	if err != nil {
		log.Fatalf("Unable to setup Health endpoint (%s): %s", addr, err)
	}

	go func() {
		log.Printf("Metrics endpoint is listening on %s", lis.Addr().String())
		log.Fatalf("Metrics server closing: %s", server.Serve(lis))
	}()
	return lis
}
