package helpers

import (
	"fmt"
	"io"
	"net"
	"sync/atomic"
	"time"
)

type SyslogTCPServer struct {
	count            int64
	listener         net.Listener
	lastLogTimestamp int64
}

func NewSyslogTCPServer(host string, port int) (*SyslogTCPServer, error) {
	address := fmt.Sprintf("%s:%d", host, port)
	listener, err := net.Listen("tcp", address)
	if err != nil {
		return nil, err
	}

	return &SyslogTCPServer{
		listener: listener,
	}, nil
}

func (s *SyslogTCPServer) Start() {
	s.listen()
}

func (s *SyslogTCPServer) Stop() {
	s.listener.Close()
}

func (s *SyslogTCPServer) Counter() int64 {
	return atomic.LoadInt64(&s.count)
}

func (s *SyslogTCPServer) URL() string {
	return fmt.Sprintf("syslog://%s", s.listener.Addr().String())
}

func (s *SyslogTCPServer) ReceivedLogsRecently() bool {
	diff := time.Now().UnixNano() - atomic.LoadInt64(&s.lastLogTimestamp)
	return diff < int64(time.Second)
}

func (s *SyslogTCPServer) incrementCounter() {
	atomic.AddInt64(&s.count, 1)
}

func (s *SyslogTCPServer) recordTimestamp() {
	ts := time.Now().UnixNano()
	atomic.StoreInt64(&s.lastLogTimestamp, ts)
}

func (s *SyslogTCPServer) listen() {
	for {
		conn, err := s.listener.Accept()
		if err != nil {
			return
		}

		go func() {
			defer conn.Close()
			buffer := make([]byte, 1024)
			for {
				conn.SetReadDeadline(time.Now().Add(100 * time.Millisecond))
				_, err := conn.Read(buffer)
				if err == io.EOF {
					return
				} else if err != nil {
					return
				}
				s.incrementCounter()
				s.recordTimestamp()
			}
		}()
	}
}
