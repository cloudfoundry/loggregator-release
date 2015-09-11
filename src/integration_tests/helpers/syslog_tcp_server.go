package helpers
import (
	"io"
	"time"
	"net"
	"sync/atomic"
	"fmt"
	"sync"
)


type SyslogTCPServer struct {
	address string
	count int64
	listener net.Listener
	lastLogTimestamp int64
	lock sync.Mutex
}

func NewSyslogTCPServer(host string, port int) *SyslogTCPServer {
	syslogAddr := fmt.Sprintf("%s:%d", host, port)
	return &SyslogTCPServer{
		address: syslogAddr,
	}
}

func (s *SyslogTCPServer) Start() {
	var err error
	s.lock.Lock()
	s.listener, err = net.Listen("tcp", s.address)
	s.lock.Unlock()

	if err != nil {
		panic(err)
	}
	s.listen()
}

func (s *SyslogTCPServer) Stop() {
	s.lock.Lock()
	s.listener.Close()
	s.lock.Unlock()
}

func (s *SyslogTCPServer) Counter() int64 {
	return atomic.LoadInt64(&s.count)
}


func (s *SyslogTCPServer) URL() string {
	return fmt.Sprintf("syslog://%s", s.address)
}

func (s *SyslogTCPServer) ReceivedLogsRecently() bool {
	diff := time.Now().UnixNano() - atomic.LoadInt64(&s.lastLogTimestamp)
	return diff  < int64(time.Second)
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