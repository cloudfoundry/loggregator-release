// rlpreader: a tool that reads messages from RLP.
//
package main

import (
	"context"
	"crypto/rand"
	"flag"
	"fmt"
	"log"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"

	"code.cloudfoundry.org/loggregator/plumbing"
	v2 "code.cloudfoundry.org/loggregator/plumbing/v2"
)

var (
	target   = flag.String("target", "localhost:3457", "the host:port of the target rlp")
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
	})
	if err != nil {
		log.Fatal(err)
	}

	for {
		env, err := receiver.Recv()
		if err != nil {
			log.Fatal(err)
		}
		fmt.Println(string(env.GetLog().GetPayload()))
		time.Sleep(*delay)
	}
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
