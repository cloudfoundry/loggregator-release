package authorization_test

import (
	"trafficcontroller/authorization"

	"errors"
	"github.com/cloudfoundry/loggregatorlib/loggertesthelper"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"trafficcontroller/uaa_client"
)

type LoggregatorAdminUaaClient struct {
	UsedToken string
}

func (client *LoggregatorAdminUaaClient) GetAuthData(token string) (*uaa_client.AuthData, error) {
	client.UsedToken = token
	return &uaa_client.AuthData{Scope: []string{"doppler.firehose"}}, nil
}

type NonAdminUaaClient struct {
	UsedToken string
}

func (client *NonAdminUaaClient) GetAuthData(token string) (*uaa_client.AuthData, error) {
	client.UsedToken = token
	return &uaa_client.AuthData{Scope: []string{"uaa.admin"}}, nil
}

type ErrorUaaClient struct {
	UsedToken string
}

func (client *ErrorUaaClient) GetAuthData(token string) (*uaa_client.AuthData, error) {
	client.UsedToken = token
	return nil, errors.New("An error occurred")
}

var _ = Describe("AdminAccessAuthorizer", func() {
	authToken := "bearer my-token"

	Context("when the UAA client has the loggregator.admin scope", func() {
		It("returns true", func() {
			client := &LoggregatorAdminUaaClient{}

			authorizer := authorization.NewAdminAccessAuthorizer(client)

			Expect(authorizer(authToken, loggertesthelper.Logger())).To(BeTrue())
			Expect(client.UsedToken).To(Equal("my-token"))
		})
	})

	Context("when the UAA client does not have the loggregator.admin scope", func() {
		It("returns false", func() {
			client := &NonAdminUaaClient{}

			authorizer := authorization.NewAdminAccessAuthorizer(client)

			Expect(authorizer(authToken, loggertesthelper.Logger())).To(BeFalse())
			Expect(client.UsedToken).To(Equal("my-token"))
		})
	})
	Context("when the uaa client throws an error", func() {
		It("returns false", func() {
			client := &ErrorUaaClient{}

			authorizer := authorization.NewAdminAccessAuthorizer(client)

			Expect(authorizer(authToken, loggertesthelper.Logger())).To(BeFalse())
			Expect(client.UsedToken).To(Equal("my-token"))
		})
	})
})
