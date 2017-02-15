package main

import "log"

func main() {
	log.Printf("Starting Reverse Log Proxy...")
	defer log.Printf("Reverse Log Proxy closing.")
}
