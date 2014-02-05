package sinkserver

import (
	"net"
	"sync"
)

type Service struct {
	ch           chan bool
	waitGroup    sync.WaitGroup
	receivedChan chan []byte
	listener     *net.TCPListener
	ReadyChan    chan bool
}

func NewFakeService(receivedChan chan []byte, host string) (s *Service, err error) {
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
		receivedChan: receivedChan,
		listener:     listener,
		ReadyChan:    make(chan bool),
	}
	return s, nil
}

func (s *Service) Serve() {
	s.waitGroup.Add(1)
	defer s.waitGroup.Done()
	go func() {
		select {
		case s.ReadyChan <- true:
		default:
		}
		for {
			conn, err := s.listener.AcceptTCP()
			if err != nil {
				return
			}
			go s.serve(conn)
		}
	}()
}

func (s *Service) Stop() {
	close(s.ch)
	s.listener.Close()
	s.waitGroup.Wait()
}

func (s *Service) serve(conn *net.TCPConn) {
	go func() {
		<-s.ch
		conn.Close()
	}()
	s.waitGroup.Add(1)
	defer s.waitGroup.Done()
	for {
		buf := make([]byte, 4096)
		readCount, err := conn.Read(buf)
		if err != nil {
			return
		}
		s.receivedChan <- buf[:readCount]
	}
}
