package dopplerwriter
import "github.com/cloudfoundry/gosteno"

type ClientPool interface {
	RandomClient() (Client, error)
}

type Client interface {
	Send([]byte)
}

type DopplerWriter struct {
	clientPool 	ClientPool
	logger 		*gosteno.Logger
}

func NewDopplerWriter(clientPool ClientPool, logger *gosteno.Logger) *DopplerWriter {
	return &DopplerWriter{
		clientPool:   clientPool,
		logger: logger,
	}
}

func (d *DopplerWriter) Write(message []byte) {
	client, err := d.clientPool.RandomClient()
	if err != nil {
		d.logger.Errorf("can't forward message: %v", err)
		return
	}
	client.Send(message)
}
