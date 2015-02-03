package heartbeatrequester

import (
	"github.com/cloudfoundry/dropsonde/control"
	"github.com/cloudfoundry/dropsonde/factories"
	"github.com/gogo/protobuf/proto"
	uuid "github.com/nu7hatch/gouuid"
	"net"
	"sync"
	"time"
)

const METRON_ORIGIN = "MET"

type HeartbeatRequester struct {
	pingTargets  map[string]pingTarget
	pingInterval time.Duration
	pingTimeout  time.Duration
	sync.Mutex
}

func NewHeartbeatRequester(interval time.Duration) *HeartbeatRequester {
	return &HeartbeatRequester{
		pingTargets:  make(map[string]pingTarget),
		pingInterval: interval,
		pingTimeout:  interval * 5,
	}
}

type pingTarget struct {
	stopChan        chan (struct{})
	deregisterTimer *time.Timer
}

func (requester *HeartbeatRequester) Start(senderAddr net.Addr, connection net.PacketConn) {
	if requester.senderKnown(senderAddr) {
		requester.resetTimer(senderAddr)
		return
	}

	requester.register(senderAddr)
	requester.sendPings(senderAddr, connection)
}

func (requester *HeartbeatRequester) Stop(target net.Addr) {
	requester.Lock()
	defer requester.Unlock()

	pTarget, ok := requester.pingTargets[target.String()]
	if ok {
		close(pTarget.stopChan)
	}
}

func (requester *HeartbeatRequester) register(senderAddr net.Addr) {
	requester.Lock()
	defer requester.Unlock()

	pTarget := pingTarget{
		stopChan:        make(chan struct{}),
		deregisterTimer: time.AfterFunc(requester.pingTimeout, func() { requester.Stop(senderAddr) }),
	}
	requester.pingTargets[senderAddr.String()] = pTarget
}

func (requester *HeartbeatRequester) sendPings(senderAddr net.Addr, connection net.PacketConn) {
	intervalTicker := time.NewTicker(requester.pingInterval)

	requester.Lock()
	pTarget, ok := requester.pingTargets[senderAddr.String()]
	requester.Unlock()

	if ok {
		for {
			select {
			case <-pTarget.stopChan:
				intervalTicker.Stop()
				requester.Lock()
				delete(requester.pingTargets, senderAddr.String())
				requester.Unlock()
				return
			case <-intervalTicker.C:
				connection.WriteTo(newHeartbeatRequest(), senderAddr)
			}
		}
	}
}

func newHeartbeatRequest() []byte {
	id, _ := uuid.NewV4()

	heartbeatRequest := &control.ControlMessage{
		Origin:      proto.String(METRON_ORIGIN),
		Identifier:  factories.NewControlUUID(id),
		Timestamp:   proto.Int64(time.Now().UnixNano()),
		ControlType: control.ControlMessage_HeartbeatRequest.Enum(),
	}

	bytes, err := proto.Marshal(heartbeatRequest)
	if err != nil {
		panic(err.Error())
	}
	return bytes
}

func (requester *HeartbeatRequester) resetTimer(senderAddr net.Addr) {
	requester.Lock()
	pTarget, ok := requester.pingTargets[senderAddr.String()]
	requester.Unlock()

	if ok {
		pTarget.deregisterTimer.Reset(requester.pingTimeout)
	}
}

func (requester *HeartbeatRequester) senderKnown(senderAddr net.Addr) bool {
	requester.Lock()
	_, ok := requester.pingTargets[senderAddr.String()]
	requester.Unlock()

	return ok
}
