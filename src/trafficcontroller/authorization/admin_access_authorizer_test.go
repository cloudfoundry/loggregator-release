package authorization_test

import (
	"trafficcontroller/authorization"

	"errors"
	"github.com/cloudfoundry/loggregatorlib/loggertesthelper"
	"trafficcontroller/uaa_client"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
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

	Context("Disable Access Control", func() {
		It("returns true", func() {
			client := &NonAdminUaaClient{}
			authorizer := authorization.NewAdminAccessAuthorizer(true, client)

			authorized, err := authorizer("", loggertesthelper.Logger())
			Expect(authorized).To(BeTrue())
			Expect(err).To(BeNil())
		})
	})

	Context("when no auth token is present", func() {
		It("returns false", func() {
			client := &LoggregatorAdminUaaClient{}
			authorizer := authorization.NewAdminAccessAuthorizer(false, client)

			authorized, err := authorizer("", loggertesthelper.Logger())

			Expect(authorized).To(BeFalse())
			Expect(err).To(Equal(errors.New(authorization.NO_AUTH_TOKEN_PROVIDED_ERROR_MESSAGE)))
		})
	})

	Context("when the UAA client has the loggregator.admin scope", func() {
		It("returns true", func() {
			client := &LoggregatorAdminUaaClient{}

			authorizer := authorization.NewAdminAccessAuthorizer(false, client)

			authorized, err := authorizer(authToken, loggertesthelper.Logger())
			Expect(authorized).To(BeTrue())
			Expect(err).To(BeNil())
			Expect(client.UsedToken).To(Equal("my-token"))
		})
	})

	Context("when the UAA client does not have the loggregator.admin scope", func() {
		It("returns false", func() {
			client := &NonAdminUaaClient{}

			authorizer := authorization.NewAdminAccessAuthorizer(false, client)

			authorized, err := authorizer(authToken, loggertesthelper.Logger())
			Expect(authorized).To(BeFalse())
			Expect(err).To(Equal(errors.New(authorization.INVALID_AUTH_TOKEN_ERROR_MESSAGE)))

			Expect(client.UsedToken).To(Equal("my-token"))
		})
	})
	Context("when the uaa client throws an error", func() {
		It("returns false", func() {
			client := &ErrorUaaClient{}

			authorizer := authorization.NewAdminAccessAuthorizer(false, client)

			authorized, err := authorizer(authToken, loggertesthelper.Logger())
			Expect(authorized).To(BeFalse())
			Expect(err).To(Equal(errors.New(authorization.INVALID_AUTH_TOKEN_ERROR_MESSAGE)))

			Expect(client.UsedToken).To(Equal("my-token"))
		})
	})
})
