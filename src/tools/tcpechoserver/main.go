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

var useSSL = flag.Bool("ssl", false, "Use SSL")
var certFile = flag.String("cert", "", "TLS certificate")
var keyFile = flag.String("key", "", "TLS private key")

var address = flag.String("address", "", "Listener address")

func main() {
	flag.Parse()

	var listener net.Listener
	if *useSSL {
		cert, err := ioutil.ReadFile(*certFile)
		if err != nil {
			panic(err)
		}

		key, err := ioutil.ReadFile(*keyFile)
		if err != nil {
			panic(err)
		}

		listener = secureListener(*address, cert, key)
	} else {
		listener = insecureListener(*address)
	}

	go listen(listener)

	log.Printf("Startup: tcp echo server listening")
	blocker := make(chan bool)
	<-blocker
}

func insecureListener(address string) net.Listener {
	listener, err := net.Listen("tcp", address)
	if err != nil {
		panic(err)
	}

	return listener
}

func secureListener(address string, certContent []byte, keyContent []byte) net.Listener {
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

	return listener
}

func listen(listener net.Listener) {
	for {
		conn, err := listener.Accept()
		if err != nil {
			return
		}

		go func() {
			defer conn.Close()
			buffer := make([]byte, 1024)
			for {
				conn.SetReadDeadline(time.Now().Add(100 * time.Millisecond))
				readCount, err := conn.Read(buffer)
				if err == io.EOF {
					return
				} else if err != nil {
					continue
				}

				buffer2 := make([]byte, readCount)
				copy(buffer2, buffer[:readCount])
				fmt.Printf("%s", buffer2)
			}
		}()
	}
}
