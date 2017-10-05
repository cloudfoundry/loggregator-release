package main

import (
	"log"
	"os"

	"code.cloudfoundry.org/loggregator/plumbing"
	"code.cloudfoundry.org/loggregator/plumbing/v2"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
)

func main() {
	dopplerAddr := os.Getenv("DOPPLER_ADDR")
	shardID := os.Getenv("SHARD_ID")
	caPath := os.Getenv("CA_PATH")
	certPath := os.Getenv("CERT_PATH")
	keyPath := os.Getenv("KEY_PATH")

	var missing []string
	if dopplerAddr == "" {
		missing = append(missing, "DOPPLER_ADDR")
	}
	if shardID == "" {
		missing = append(missing, "SHARD_ID")
	}
	if caPath == "" {
		missing = append(missing, "CA_PATH")
	}
	if certPath == "" {
		missing = append(missing, "CERT_PATH")
	}
	if keyPath == "" {
		missing = append(missing, "KEY_PATH")
	}

	if len(missing) > 0 {
		log.Fatalf("missing required environment variables: %v", missing)
	}

	creds, err := plumbing.NewClientCredentials(
		certPath,
		keyPath,
		caPath,
		"doppler",
	)
	if err != nil {
		log.Fatalf("failed to load client credentials: %s", err)
	}

	conn, err := grpc.Dial(dopplerAddr, grpc.WithTransportCredentials(creds))
	if err != nil {
		log.Fatalf("failed to dial doppler: %s", err)
	}

	client := loggregator_v2.NewEgressClient(conn)

	stream, err := client.BatchedReceiver(
		context.Background(),
		&loggregator_v2.EgressBatchRequest{
			ShardId: shardID,
		},
	)
	if err != nil {
		log.Fatalf("failed to open stream from doppler V2: %s", err)
	}

	for {
		batch, err := stream.Recv()
		if err != nil {
			log.Fatalf("failed to read from stream: %s", err)
		}

		for _, e := range batch.GetBatch() {
			log.Printf("%+v", e)
		}
	}
}
