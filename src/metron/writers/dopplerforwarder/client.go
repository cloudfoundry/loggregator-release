package dopplerforwarder

type Client interface {
	Write(message []byte) (sentLength int, err error)
	Close() error
}
