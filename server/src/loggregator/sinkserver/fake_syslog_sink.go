package sinkserver

import (
	"net"
	"sync"
	"time"
)

type Service struct {
	ch           chan bool
	waitGroup    *sync.WaitGroup
	receivedChan chan []byte
	listener     *net.TCPListener
	readyChan    chan bool
}

func NewService(receivedChan chan []byte, host string) (s *Service, err error) {
	var (
		laddr    *net.TCPAddr
		listener *net.TCPListener
	)

	if laddr, err = net.ResolveTCPAddr("tcp", host); nil != err {
		return nil, err
	}
	if listener, err = net.ListenTCP("tcp", laddr); nil != err {
		return nil, err
	}

	s = &Service{
		ch:           make(chan bool),
		waitGroup:    &sync.WaitGroup{},
		receivedChan: receivedChan,
		listener:     listener,
		readyChan:    make(chan bool),
	}
	s.waitGroup.Add(1)
	return s, nil
}

func (s *Service) Serve() {
	defer s.waitGroup.Done()
	for {
		select {
		case <-s.ch:
			s.listener.Close()
			return
		default:
		}
		s.listener.SetDeadline(time.Now().Add(1e9))
		select {
		case s.readyChan <- true:
		default:
		}
		conn, err := s.listener.AcceptTCP()
		if nil != err {
			if opErr, ok := err.(*net.OpError); ok && opErr.Timeout() {
				continue
			}
		}
		s.waitGroup.Add(1)
		go s.serve(conn)
	}
}

func (s *Service) Stop() {
	close(s.ch)
	s.waitGroup.Wait()
}

func (s *Service) serve(conn *net.TCPConn) {
	defer conn.Close()
	defer s.waitGroup.Done()
	for {
		select {
		case <-s.ch:
			return
		default:
		}
		conn.SetDeadline(time.Now().Add(1e9))
		buf := make([]byte, 4096)
		readCount, err := conn.Read(buf)
		if err != nil {
			break
		}
		s.receivedChan <- buf[:readCount]
	}
}
