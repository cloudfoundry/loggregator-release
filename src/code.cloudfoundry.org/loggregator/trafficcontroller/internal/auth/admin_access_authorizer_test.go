package auth_test

import (
	"errors"

	"code.cloudfoundry.org/loggregator/trafficcontroller/internal/auth"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

type LoggregatorAdminUaaClient struct {
	UsedToken string
}

func (client *LoggregatorAdminUaaClient) GetAuthData(token string) (*auth.AuthData, error) {
	client.UsedToken = token
	return &auth.AuthData{Scope: []string{"doppler.firehose"}}, nil
}

type NonAdminUaaClient struct {
	UsedToken string
}

func (client *NonAdminUaaClient) GetAuthData(token string) (*auth.AuthData, error) {
	client.UsedToken = token
	return &auth.AuthData{Scope: []string{"uaa.admin"}}, nil
}

type ErrorUaaClient struct {
	UsedToken string
}

func (client *ErrorUaaClient) GetAuthData(token string) (*auth.AuthData, error) {
	client.UsedToken = token
	return nil, errors.New("An error occurred")
}

var _ = Describe("AdminAccessAuthorizer", func() {
	authToken := "bearer my-token"

	Context("Disable Access Control", func() {
		It("returns true", func() {
			client := &NonAdminUaaClient{}
			authorizer := auth.NewAdminAccessAuthorizer(true, client)

			authorized, err := authorizer("")
			Expect(authorized).To(BeTrue())
			Expect(err).To(BeNil())
		})
	})

	Context("when no auth token is present", func() {
		It("returns false", func() {
			client := &LoggregatorAdminUaaClient{}
			authorizer := auth.NewAdminAccessAuthorizer(false, client)

			authorized, err := authorizer("")

			Expect(authorized).To(BeFalse())
			Expect(err).To(Equal(errors.New(auth.NO_AUTH_TOKEN_PROVIDED_ERROR_MESSAGE)))
		})
	})

	Context("when the UAA client has the loggregator.admin scope", func() {
		It("returns true", func() {
			client := &LoggregatorAdminUaaClient{}

			authorizer := auth.NewAdminAccessAuthorizer(false, client)

			authorized, err := authorizer(authToken)
			Expect(authorized).To(BeTrue())
			Expect(err).To(BeNil())
			Expect(client.UsedToken).To(Equal("my-token"))
		})
	})

	Context("when the UAA client does not have the loggregator.admin scope", func() {
		It("returns false", func() {
			client := &NonAdminUaaClient{}

			authorizer := auth.NewAdminAccessAuthorizer(false, client)

			authorized, err := authorizer(authToken)
			Expect(authorized).To(BeFalse())
			Expect(err).To(Equal(errors.New(auth.INVALID_AUTH_TOKEN_ERROR_MESSAGE)))

			Expect(client.UsedToken).To(Equal("my-token"))
		})
	})
	Context("when the uaa client throws an error", func() {
		It("returns false", func() {
			client := &ErrorUaaClient{}

			authorizer := auth.NewAdminAccessAuthorizer(false, client)

			authorized, err := authorizer(authToken)
			Expect(authorized).To(BeFalse())
			Expect(err).To(Equal(errors.New(auth.INVALID_AUTH_TOKEN_ERROR_MESSAGE)))

			Expect(client.UsedToken).To(Equal("my-token"))
		})
	})
})
