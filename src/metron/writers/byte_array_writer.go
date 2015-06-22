package writers

type ByteArrayWriter interface {
	Write(message []byte)
}
