package clientpool

import "google.golang.org/grpc"

type Dialer struct {
	dopplerAddr string
	zonePrefix  string
	opts        []grpc.DialOption
}

func NewDialer(dopplerAddr, zonePrefix string, opts ...grpc.DialOption) Dialer {
	return Dialer{
		dopplerAddr: dopplerAddr,
		zonePrefix:  zonePrefix,
		opts:        opts,
	}
}

func (d Dialer) Dial() (*grpc.ClientConn, error) {
	return grpc.Dial(d.dopplerAddr, d.opts...)
}

func (d Dialer) String() string {
	// todo: return the exact addr we are connecting to, may be zone specific
	return d.dopplerAddr
}
