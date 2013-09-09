package sinks

import (
	"errors"
	"fmt"
	"github.com/cloudfoundry/gosteno"
	"math"
	"net"
	"strings"
	"sync"
	"time"
)

type retryStrategy func(counter int) time.Duration

func newExponentialRetryStrategy() retryStrategy {
	exponential := func(counter int) time.Duration {
		duration := math.Pow(2, float64(counter))
		return time.Duration(int(duration)) * time.Millisecond
	}
	return exponential
}

type writer struct {
	appId                string
	network              string
	raddr                string
	getNextSleepDuration retryStrategy
	logger               *gosteno.Logger

	mu   sync.Mutex // guards conn
	conn net.Conn
}

// connect makes a connection to the syslog server.
// It must be called with w.mu held.
func (w *writer) connect() (err error) {
	if w.conn != nil {
		// ignore err from close, it makes sense to continue anyway
		w.conn.Close()
		w.conn = nil
	}
	var c net.Conn
	c, err = net.Dial(w.network, w.raddr)
	if err == nil {
		w.conn = c
	}
	return
}

func (w *writer) writeStdout(b []byte) (int, error) {
	return w.writeAndRetry(6, string(b))
}

func (w *writer) writeStderr(b []byte) (int, error) {
	return w.writeAndRetry(3, string(b))
}

func (w *writer) close() error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.conn != nil {
		err := w.conn.Close()
		w.conn = nil
		return err
	}
	return nil
}

func (w *writer) writeAndRetry(p int, s string) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.conn != nil {
		if n, err := w.write(p, s); err == nil {
			return n, err
		}
	}
	if err := w.connectWithRetry(1); err != nil {
		return 0, err
	}
	return w.write(p, s)
}

func (w *writer) connectWithRetry(counter int) error {
	if counter > 13 {
		return errors.New("Exceeded maximum wait time for establishing a connection to write to.")
	}
	if err := w.connect(); err != nil {
		w.logger.Warnf("Syslog socket on %s not reachable, retrying in %s", w.raddr, w.getNextSleepDuration(counter))
		time.Sleep(w.getNextSleepDuration(counter))
		return w.connectWithRetry(counter + 1)
	}
	return nil
}

func (w *writer) write(p int, msg string) (int, error) {
	// ensure it ends in a \n
	nl := ""
	if !strings.HasSuffix(msg, "\n") {
		nl = "\n"
	}

	timestamp := time.Now().Format(time.RFC3339)
	fmt.Fprintf(w.conn, "<%d>%s %s %s: %s%s",
		p, timestamp, "loggregator",
		w.appId, msg, nl)
	return len(msg), nil
}

func dial(network, raddr string, appId string, logger *gosteno.Logger) (*writer, error) {

	w := &writer{
		appId:                appId,
		network:              network,
		raddr:                raddr,
		getNextSleepDuration: newExponentialRetryStrategy(),
		logger:               logger,
	}

	w.mu.Lock()
	defer w.mu.Unlock()

	err := w.connect()
	if err != nil {
		return nil, err
	}
	return w, err
}
