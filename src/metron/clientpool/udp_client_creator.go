package clientpool

import "github.com/cloudfoundry/gosteno"

type UDPClientCreator struct {
	logger *gosteno.Logger
}

func NewUDPClientCreator(logger *gosteno.Logger) *UDPClientCreator {
	return &UDPClientCreator{logger: logger}
}

func (u *UDPClientCreator) CreateClient(address string) (Client, error) {
	client, err := NewUDPClient(u.logger, address)
	if err != nil {
		return nil, err
	}
	if err = client.Connect(); err != nil {
		return nil, err
	}
	return client, nil
}
