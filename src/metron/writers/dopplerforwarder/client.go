package dopplerforwarder

//go:generate hel --type Client --output mock_client_test.go

type Client interface {
	Write(message []byte) (sentLength int, err error)
	Close() error
}
