package main

import (
	"bytes"
	"fmt"
	"time"
	"tools/metronbenchmark/experiment"
	"tools/metronbenchmark/messagewriter"

	"tools/metronbenchmark/messagereader"

	"github.com/cloudfoundry/loggregatorlib/loggertesthelper"
	"github.com/cloudfoundry/sonde-go/events"
	"github.com/cloudfoundry/storeadapter"
	"github.com/cloudfoundry/gunk/workpool"
	"github.com/cloudfoundry/storeadapter/etcdstoreadapter"
	"github.com/cloudfoundry/dropsonde/dropsonde_unmarshaller"
	"github.com/cloudfoundry/loggregatorlib/agentlistener"
	"github.com/onsi/gomega/format"
)

var fakeDoppler agentlistener.AgentListener
//var testDoppler net.PacketConn
var envelopes chan *events.Envelope

func main() {
	// spin up 1 reader
	// spin up 3 writers

	// spin up metron? -possibility

	// writers will write for as long as the program runs
	// writers will print out how many messages they are asked to send and how many they are actually sending

	// reader will total new results
	// reader print results


	var signedMessages <-chan []byte
	var unsignedMessages = make(chan []byte)
	envelopes = make(chan *events.Envelope, 1024)

	fakeDoppler, signedMessages = agentlistener.NewAgentListener("localhost:3457", loggertesthelper.Logger(), "fakeDoppler")
//	testDoppler, _ = net.ListenPacket("udp", "localhost:3457")

	b := &bytes.Buffer{}
	writer := messagewriter.NewMessageWriter(b)
	reader := messagereader.NewMessageReader(logReader{})
	exp := experiment.New(writer, reader, fakeFactory)

	unmarshaller := dropsonde_unmarshaller.NewDropsondeUnmarshaller(loggertesthelper.Logger())

	go unmarshaller.Run(unsignedMessages, envelopes)
	go func() {
		for signedMessage := range signedMessages {
			unsignedMessages <- signedMessage[32:]
		}
	}()

	go fakeDoppler.Start()

	announceToEtcd()

	go func() {
		for {
			reader.Start()
			time.Sleep(time.Second)
		}
	}()
	exp.Start(make(chan struct{}))

}
func NewStoreAdapter(urls []string, concurrentRequests int) storeadapter.StoreAdapter {
	workPool, err := workpool.NewWorkPool(concurrentRequests)
	if err != nil {
		panic(err)
	}
	etcdStoreAdapter := etcdstoreadapter.NewETCDStoreAdapter(urls, workPool)
	etcdStoreAdapter.Connect()
	return etcdStoreAdapter
}

func announceToEtcd() {
	etcdUrls := []string {"http://localhost:4001"}
	etcdMaxConcurrentRequests := 10
	storeAdapter := NewStoreAdapter(etcdUrls, etcdMaxConcurrentRequests)
	node := storeadapter.StoreNode{
		Key:   "/healthstatus/doppler/z1/0",
		Value: []byte("localhost"),
	}
	storeAdapter.Create(node)
	storeAdapter.Disconnect()
	time.Sleep(50 * time.Millisecond)
}

type logReader struct {
}

func (r logReader) Read() (*events.LogMessage, bool) {
	for {
		select {
		case env := <-envelopes:
			if env.GetValueMetric() != nil {
				fmt.Printf("Got a value metric: %s", format.Object(env.GetValueMetric(),1))
				return env.GetLogMessage(), false
			}else {
	fmt.Printf("No Message... got %v", env)

			}
		}
	}
	fmt.Print("Reading Message...")
	// listen on envelopes channel
	return &events.LogMessage{}, false
}

func fakeFactory(id uint, rate uint, d time.Duration) experiment.Round {
	fake := fakeRound{
		id:       id,
		rate:     rate,
		duration: d,
	}

	return &fake
}

type fakeRound struct {
	id       uint
	rate     uint
	duration time.Duration

	performCalled bool
}

func (r *fakeRound) Id() uint {
	return r.id
}

func (r *fakeRound) Rate() uint {
	return r.rate
}

func (r *fakeRound) Duration() time.Duration {
	return r.duration
}

func (r *fakeRound) Perform(writer experiment.MessageWriter) {
	r.performCalled = true
	writer.Send(r.id, time.Now())
	fmt.Printf("**** %d, %v \n", r.id, time.Now())
	time.Sleep(time.Second)
}
