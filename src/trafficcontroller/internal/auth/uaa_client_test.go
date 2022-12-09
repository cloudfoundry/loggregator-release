package auth_test

import (
	"crypto/tls"
	"encoding/json"
	"net/http"
	"net/http/httptest"

	"code.cloudfoundry.org/loggregator-release/trafficcontroller/internal/auth"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("UaaClient", func() {
	var (
		handler       = FakeUaaHandler{}
		fakeUaaServer = httptest.NewTLSServer(&handler)

		transport *http.Transport
		client    *http.Client
	)

	BeforeEach(func() {
		transport = &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true, //nolint:gosec
			},
		}
		client = &http.Client{
			Transport: transport,
		}
	})

	Context("when the user is an admin", func() {
		It("Determines permissions from correct credentials", func() {
			uaaClient := auth.NewUaaClient(client, fakeUaaServer.URL, "bob", "yourUncle")

			authData, err := uaaClient.GetAuthData("iAmAnAdmin")
			Expect(err).ToNot(HaveOccurred())

			Expect(authData.HasPermission("doppler.firehose")).To(Equal(true))
			Expect(authData.HasPermission("uaa.not-admin")).To(Equal(false))

		})
	})

	Context("when the user is not an admin", func() {
		It("Determines permissions from correct credentials", func() {
			uaaClient := auth.NewUaaClient(client, fakeUaaServer.URL, "bob", "yourUncle")

			authData, err := uaaClient.GetAuthData("iAmNotAnAdmin")
			Expect(err).ToNot(HaveOccurred())

			Expect(authData.HasPermission("doppler.firehose")).To(Equal(false))
			Expect(authData.HasPermission("uaa.not-admin")).To(Equal(true))
		})
	})

	Context("the token is expired", func() {
		It("returns the proper error", func() {
			uaaClient := auth.NewUaaClient(client, fakeUaaServer.URL, "bob", "yourUncle")

			_, err := uaaClient.GetAuthData("expiredToken")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal("Token has expired"))
		})
	})

	Context("the token is invalid", func() {
		It("returns the proper error", func() {
			uaaClient := auth.NewUaaClient(client, fakeUaaServer.URL, "bob", "yourUncle")

			_, err := uaaClient.GetAuthData("invalidToken")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal("Invalid token (could not decode): invalidToken"))
		})
		Context("the server returns an unparseable response", func() {
			It("returns the json parsing error", func() {
				uaaClient := auth.NewUaaClient(client, fakeUaaServer.URL, "bob", "yourUncle")

				_, err := uaaClient.GetAuthData("invalidTokenWithBadResponse")
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("JSON"))
			})
		})
	})

	Context("the server returns a 500 ", func() {
		It("returns the proper error", func() {
			uaaClient := auth.NewUaaClient(client, fakeUaaServer.URL, "bob", "yourUncle")

			_, err := uaaClient.GetAuthData("500Please")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal("Unknown error occurred"))
		})
	})

	Context("the un/pwd is invalid", func() {
		It("returns the proper error", func() {
			uaaClient := auth.NewUaaClient(client, fakeUaaServer.URL, "wrongUser", "yourUncle")

			_, err := uaaClient.GetAuthData("iAmAnAdmin")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal("Invalid username/password"))
		})
	})

	Context("the UAA address is invalid", func() {
		It("returns a server error", func() {
			uaaClient := auth.NewUaaClient(client, "%%m*+broken", "bob", "yourUncle")

			_, err := uaaClient.GetAuthData("iAmAnAdmin")
			Expect(err.Error()).To(ContainSubstring(`parse`))
			Expect(err.Error()).To(ContainSubstring(`%%m*+broken/check_token`))
		})
	})

	Context("insecure skip verify is false", func() {
		BeforeEach(func() {
			transport.TLSClientConfig.InsecureSkipVerify = false
		})

		It("returns an error status", func() {
			uaaClient := auth.NewUaaClient(client, fakeUaaServer.URL, "bob", "yourUncle")

			_, err := uaaClient.GetAuthData("iAmAnAdmin")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(HaveSuffix("x509: certificate signed by unknown authority"))
		})
	})
})

type FakeUaaHandler struct {
}

func (h *FakeUaaHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/check_token" {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	if r.Header.Get("Authorization") != "Basic Ym9iOnlvdXJVbmNsZQ==" {
		w.WriteHeader(http.StatusUnauthorized)
		_, err := w.Write([]byte("{\"error\":\"unauthorized\",\"error_description\":\"No client with requested id: wrongUser\"}"))
		Expect(err).ToNot(HaveOccurred())
		return
	}

	token := r.FormValue("token")

	if token == "iAmAnAdmin" {
		authData := map[string]interface{}{
			"scope": []string{
				"doppler.firehose",
			},
		}

		marshaled, err := json.Marshal(authData)
		Expect(err).ToNot(HaveOccurred())

		_, err = w.Write(marshaled)
		Expect(err).ToNot(HaveOccurred())
	} else if token == "iAmNotAnAdmin" {
		authData := map[string]interface{}{
			"scope": []string{
				"uaa.not-admin",
			},
		}

		marshaled, err := json.Marshal(authData)
		Expect(err).ToNot(HaveOccurred())

		_, err = w.Write(marshaled)
		Expect(err).ToNot(HaveOccurred())
	} else if token == "expiredToken" {
		w.WriteHeader(http.StatusBadRequest)
		_, err := w.Write([]byte("{\"error\":\"invalid_token\",\"error_description\":\"Token has expired\"}"))
		Expect(err).ToNot(HaveOccurred())
	} else if token == "invalidToken" {
		w.WriteHeader(http.StatusBadRequest)
		_, err := w.Write([]byte("{\"invalidToken\":\"invalid_token\",\"error_description\":\"Invalid token (could not decode): invalidToken\"}"))
		Expect(err).ToNot(HaveOccurred())
	} else if token == "invalidTokenWithBadResponse" {
		w.WriteHeader(http.StatusBadRequest)
		_, err := w.Write([]byte("{"))
		Expect(err).ToNot(HaveOccurred())
	} else {
		w.WriteHeader(http.StatusInternalServerError)
	}
}
