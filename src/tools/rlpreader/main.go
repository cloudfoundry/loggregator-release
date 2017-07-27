// rlpreader: a tool that reads messages from RLP.
//
package main

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"sync/atomic"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"

	"code.cloudfoundry.org/loggregator/plumbing"
	v2 "code.cloudfoundry.org/loggregator/plumbing/v2"
)

var (
	target   = flag.String("target", "localhost:3457", "the host:port of the target rlp")
	httpAddr = flag.String("http-addr", "localhost:8081", "the host:port for HTTP to listen")
	appID    = flag.String("app-id", "", "app-id to stream data")
	certFile = flag.String("cert", "", "cert to use to connect to rlp")
	keyFile  = flag.String("key", "", "key to use to connect to rlp")
	caFile   = flag.String("ca", "", "ca cert to use to connect to rlp")
	delay    = flag.Duration("delay", time.Second, "delay inbetween reading messages")
)

func main() {
	flag.Parse()

	tlsConfig, err := plumbing.NewClientMutualTLSConfig(
		*certFile,
		*keyFile,
		*caFile,
		"reverselogproxy",
	)
	if err != nil {
		log.Fatal(err)
	}
	transportCreds := credentials.NewTLS(tlsConfig)

	conn, err := grpc.Dial(*target, grpc.WithTransportCredentials(transportCreds))
	if err != nil {
		log.Fatal(err)
	}
	client := v2.NewEgressClient(conn)
	receiver, err := client.Receiver(context.TODO(), &v2.EgressRequest{
		ShardId: buildShardId(),
		Filter: &v2.Filter{
			SourceId: *appID,
			Message: &v2.Filter_Log{
				Log: &v2.LogFilter{},
			},
		},
		UsePreferredTags: true,
	})
	if err != nil {
		log.Fatal(err)
	}

	reporter := &idxReporter{}
	go func() {
		var lastIdx int64
		for {
			env, err := receiver.Recv()
			if err != nil {
				fmt.Printf("stopping reader, lastIdx: %d\n", lastIdx)
				return
			}
			lastIdx, err = strconv.ParseInt(env.Tags["idx"], 10, 0)
			if err != nil {
				log.Fatal(err)
			}
			reporter.set(lastIdx)
			time.Sleep(*delay)
		}
	}()
	log.Fatal(http.ListenAndServe(*httpAddr, reporter))
}

type idxReporter struct {
	lastIdx int64
}

func (r *idxReporter) ServeHTTP(rw http.ResponseWriter, _ *http.Request) {
	json.NewEncoder(rw).Encode(atomic.LoadInt64(&r.lastIdx))
}

func (r *idxReporter) set(v int64) {
	atomic.StoreInt64(&r.lastIdx, v)
}

func buildShardId() string {
	return "rlp-reader-" + randString()
}

func randString() string {
	b := make([]byte, 20)
	_, err := rand.Read(b)
	if err != nil {
		log.Panicf("unable to read randomness %s:", err)
	}
	return fmt.Sprintf("%x", b)
}
