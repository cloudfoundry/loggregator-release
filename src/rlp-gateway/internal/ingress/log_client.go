package ingress

import (
	"context"
	"log"

	"code.cloudfoundry.org/go-loggregator/v9/rpc/loggregator_v2"
	"code.cloudfoundry.org/loggregator-release/src/rlp-gateway/internal/web"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

// LogClient handles dialing and opening streams to the logs provider.
type LogClient struct {
	c          loggregator_v2.EgressClient
	connection *grpc.ClientConn
}

// NewClient dials the logs provider and returns a new log client.
func NewLogClient(creds credentials.TransportCredentials, logsProviderAddr string) *LogClient {
	conn, err := grpc.NewClient(logsProviderAddr,
		grpc.WithTransportCredentials(creds),
		grpc.WithDefaultCallOptions(grpc.MaxCallRecvMsgSize(10*1024*1024)),
	)
	if err != nil {
		log.Fatalf("failed to dial logs provider: %s", err)
	}
	client := loggregator_v2.NewEgressClient(conn)
	return &LogClient{
		c:          client,
		connection: conn,
	}
}

// Stream opens a new stream on the log client.
func (c *LogClient) Stream(ctx context.Context, req *loggregator_v2.EgressBatchRequest) (web.Receiver, error) {
	receiver, err := c.c.BatchedReceiver(ctx, req)
	if err != nil {
		log.Printf("failed to open stream from logs provider: %s", err)
		return nil, err
	}

	return receiver.Recv, nil
}

func (c *LogClient) Close() error {
	return c.connection.Close()
}
