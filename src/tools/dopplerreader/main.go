package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"plumbing"
	"sync"

	"github.com/gorilla/websocket"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
)

var (
	useRPC           bool
	dopplers, appIDs []string
)

func main() {
	flag.BoolVar(&useRPC, "rpc", false, "Use RPC to connect to doppler.  Defaults to using websocket.")
	flag.Var(newMultiString(&dopplers), "d", "Doppler's address.  Specify once per doppler.")
	flag.Var(newMultiString(&appIDs), "a", "The ID of the application.  Specify once per app.")
	flag.Parse()

	wg := &sync.WaitGroup{}
	for _, app := range appIDs {
		log.Printf("Streaming logs for app %s", app)
		streamApp(app, wg)
	}
	wg.Wait()
}

func streamApp(app string, wg *sync.WaitGroup) {
	for _, d := range dopplers {
		wg.Add(1)
		log.Printf("Connecting to doppler %s for app %s", d, app)
		go streamFrom(d, app, wg)
	}
}

func streamFrom(doppler, app string, wg *sync.WaitGroup) {
	defer wg.Done()

	switch useRPC {
	case true:
		rpcConnect(doppler, app)
	case false:
		wsConnect(fmt.Sprintf("ws://%s/apps/%s/stream", doppler, app))
	}
}

func rpcConnect(doppler, app string) {
	conn, err := grpc.Dial(doppler, grpc.WithInsecure())
	if err != nil {
		panic(err)
	}
	defer conn.Close()

	client := plumbing.NewDopplerClient(conn)

	stream, err := client.Stream(context.Background(), &plumbing.StreamRequest{AppID: app})
	if err != nil {
		panic(err)
	}
	for {
		_, err = stream.Recv()
		if err != nil {
			panic(err)
		}
	}
}

func wsConnect(url string) {
	conn, resp, err := websocket.DefaultDialer.Dial(url, nil)
	if err != nil {
		panic(err)
	}
	if resp.StatusCode != http.StatusSwitchingProtocols {
		panic(fmt.Errorf("Unexpected response: %s", resp.Status))
	}
	log.Printf("Reading from connection to %s", url)
	for {
		_, _, err := conn.ReadMessage()
		if err != nil {
			panic(err)
		}
	}
}

type multiString []string

func newMultiString(v *[]string) *multiString {
	return (*multiString)(v)
}

func (s *multiString) Set(v string) error {
	*s = append(*s, v)
	return nil
}

func (s *multiString) Get() interface{} {
	return []string(*s)
}

func (s *multiString) String() string {
	return fmt.Sprintf("%v", *s)
}
