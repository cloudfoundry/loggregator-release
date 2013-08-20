package routerregistrar

import (
	"time"
)

const RouterUnregisterMessageSubject = "router.unregister"
const RouterRegisterMessageSubject = "router.register"
const RouterGreetMessageSubject = "router.greet"
const RouterStartMessageSubject = "router.start"

type RouterMessage struct {
	Host string   `json:"host"`
	Port uint32   `json:"port"`
	Uris []string `json:"uris"`
}

type RouterResponse struct {
	RegisterInterval time.Duration `json:"minimumRegisterIntervalInSeconds"`
}
