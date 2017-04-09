package main

import (
	"crypto/tls"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net"
	"time"
)

func main() {
	ssl := flag.Bool("ssl", false, "Use SSL")
	certPath := flag.String("cert", "", "TLS certificate")
	keyPath := flag.String("key", "", "TLS private key")
	address := flag.String("address", "", "Listener address")
	flag.Parse()

	s := NewServer(*ssl, *certPath, *keyPath, *address)
	s.Start()
	defer s.Stop()
}

const (
	readDeadline = 100 * time.Millisecond
)

func NewServer(listenTLS bool, certPath, keyPath, address string) *server {
	s := &server{}
	if listenTLS {
		s.listenTLS(address, certPath, keyPath)
		return s
	}

	s.listen(address)
	return s
}

type server struct {
	listener net.Listener
}

func (s *server) Start() {
	log.Printf("tcp echo server listening on %s", s.listener.Addr())

	for {
		conn, err := s.listener.Accept()
		if err != nil {
			return
		}

		go s.handleConnection(conn)
	}
}

func (s *server) Stop() {
	err := s.listener.Close()
	if err != nil {
		log.Fatalf("could not stop listener: %s", err)
	}
}

func (s *server) listen(address string) {
	listener, err := net.Listen("tcp", address)
	if err != nil {
		panic(err)
	}

	s.listener = listener
}

func (s *server) listenTLS(address string, certPath, keyPath string) {
	certContent, err := ioutil.ReadFile(certPath)
	if err != nil {
		panic(err)
	}

	keyContent, err := ioutil.ReadFile(keyPath)
	if err != nil {
		panic(err)
	}

	cert, err := tls.X509KeyPair(certContent, keyContent)
	if err != nil {
		panic(err)
	}
	config := &tls.Config{
		InsecureSkipVerify:     true,
		Certificates:           []tls.Certificate{cert},
		SessionTicketsDisabled: true,
	}

	listener, err := tls.Listen("tcp", address, config)
	if err != nil {
		panic(err)
	}

	s.listener = listener
}

func (s *server) handleConnection(conn net.Conn) {
	defer conn.Close()
	conn.SetReadDeadline(time.Now().Add(readDeadline))

	buffer := make([]byte, 1024)
	for {
		readCount, err := conn.Read(buffer)
		if err == io.EOF {
			return
		}

		if err != nil {
			continue
		}

		buffer2 := make([]byte, readCount)
		copy(buffer2, buffer[:readCount])
		fmt.Printf("%s", buffer2)
	}
}
