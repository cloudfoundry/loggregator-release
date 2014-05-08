package listener

type StopChannel <-chan struct{}
type OutputChannel chan<- []byte

type Listener interface {
	Start(string, string, OutputChannel, StopChannel) error
}
