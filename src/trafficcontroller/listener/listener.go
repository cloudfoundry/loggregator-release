package listener

type StopChannel <-chan struct{}
type OutputChannel chan<- []byte

type Listener interface {
	Start(string, OutputChannel, StopChannel) error
	Wait()
}
