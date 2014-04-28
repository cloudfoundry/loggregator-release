package listener

type Listener interface {
	Start(string) (<-chan []byte, error)
	Stop()
}
