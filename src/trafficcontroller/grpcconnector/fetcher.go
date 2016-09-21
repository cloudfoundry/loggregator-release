package grpcconnector

import (
	"doppler/dopplerservice"
	"fmt"
	"plumbing"
	"strings"
	"sync"

	"golang.org/x/net/context"

	"github.com/cloudfoundry/gosteno"

	"google.golang.org/grpc"
)

type Finder interface {
	Next() dopplerservice.Event
}

type Fetcher struct {
	logger    *gosteno.Logger
	port      int
	lock      sync.RWMutex
	grpcConns []connInfo

	finder Finder
}

type connInfo struct {
	conn   *grpc.ClientConn
	client plumbing.DopplerClient
}

func NewFetcher(port int, finder Finder, logger *gosteno.Logger) *Fetcher {
	f := &Fetcher{
		logger: logger,
		port:   port,
		finder: finder,
	}

	go f.run()
	return f
}

func (f *Fetcher) FetchStream(ctx context.Context, req *plumbing.StreamRequest, opts ...grpc.CallOption) ([]Receiver, error) {
	f.lock.RLock()
	defer f.lock.RUnlock()

	var result []Receiver
	for _, conn := range f.grpcConns {
		rx, err := conn.client.Stream(ctx, req, opts...)
		if err != nil {
			f.logger.Errorf("Stream() returned an error: %s", err)
			return nil, err
		}

		result = append(result, rx)
	}

	if len(result) == 0 {
		return nil, fmt.Errorf("unable to find a single doppler")
	}

	return result, nil
}

func (f *Fetcher) FetchFirehose(ctx context.Context, req *plumbing.FirehoseRequest, opts ...grpc.CallOption) ([]Receiver, error) {
	f.lock.RLock()
	defer f.lock.RUnlock()

	var result []Receiver
	for _, conn := range f.grpcConns {
		rx, err := conn.client.Firehose(ctx, req, opts...)
		if err != nil {
			f.logger.Errorf("Firehose() returned an error: %s", err)
			return nil, err
		}

		result = append(result, rx)
	}

	if len(result) == 0 {
		return nil, fmt.Errorf("unable to find a single doppler")
	}

	return result, nil
}

func (f *Fetcher) run() {
	for {
		event := f.finder.Next()
		f.lock.Lock()
		f.disposeOld()
		f.grpcConns = f.establishConns(event)
		f.lock.Unlock()
	}
}

func (f *Fetcher) establishConns(event dopplerservice.Event) []connInfo {
	var result []connInfo

	for _, udpURI := range event.UDPDopplers {
		grpcURI := f.extractURI(udpURI)
		conn, err := grpc.Dial(grpcURI, grpc.WithInsecure())
		if err != nil {
			f.logger.Errorf("unable to connect to '%s': %s", grpcURI, err)
			continue
		}

		c := plumbing.NewDopplerClient(conn)
		result = append(result, connInfo{
			conn:   conn,
			client: c,
		})
	}

	return result
}

func (f *Fetcher) extractURI(udpURI string) string {
	extracted := strings.TrimPrefix(udpURI, "udp://")
	if idx := strings.IndexRune(extracted, ':'); idx != -1 {
		extracted = extracted[:idx]
	}
	return fmt.Sprintf("%s:%d", extracted, f.port)
}

func (f *Fetcher) disposeOld() {
	for _, conn := range f.grpcConns {
		conn.conn.Close()
	}
}
