package cflib_test

import (
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"tools/logcounterapp/cflib"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("UAA", func() {
	Describe("GetAuthToken", func() {
		It("calls with required http headers", func() {
			var request *http.Request

			handler := func(w http.ResponseWriter, r *http.Request) {
				request = r
			}
			testServer := httptest.NewServer(http.HandlerFunc(handler))
			defer testServer.Close()

			uaa := cflib.UAA{
				URL:          testServer.URL,
				Username:     "testuser",
				Password:     "testpass",
				ClientID:     "testclient",
				ClientSecret: "testsecret",
			}
			_, err := uaa.GetAuthToken()
			Expect(err).ToNot(HaveOccurred())

			Expect(request.URL.Path).To(Equal("/oauth/token"))

			expectedAuthHeader := "Basic " + base64.StdEncoding.EncodeToString([]byte("testclient:testsecret"))
			Expect(request.Header.Get("Authorization")).To(Equal(expectedAuthHeader))
			Expect(request.Header.Get("Content-Type")).To(Equal("application/x-www-form-urlencoded"))
		})

		It("requests with the correct POST data", func() {
			var request *http.Request

			handler := func(w http.ResponseWriter, r *http.Request) {
				r.ParseForm()
				request = r
			}
			testServer := httptest.NewServer(http.HandlerFunc(handler))
			defer testServer.Close()

			uaa := cflib.UAA{
				URL:          testServer.URL,
				Username:     "testuser",
				Password:     "testpass",
				ClientID:     "testclient",
				ClientSecret: "testsecret",
			}

			_, err := uaa.GetAuthToken()
			Expect(err).ToNot(HaveOccurred())

			Expect(request.FormValue("grant_type")).To(Equal("password"))
			Expect(request.FormValue("client_id")).To(Equal(uaa.ClientID))
			Expect(request.FormValue("client_secret")).To(Equal(uaa.ClientSecret))
			Expect(request.FormValue("username")).To(Equal(uaa.Username))
			Expect(request.FormValue("password")).To(Equal(uaa.Password))
			Expect(request.FormValue("response_type")).To(Equal("token"))
			Expect(request.FormValue("scope")).To(Equal(""))
		})

		It("returns a token for successful POST", func() {
			handler := func(w http.ResponseWriter, r *http.Request) {
				w.Write([]byte(`{"access_token":"fake-token"}`))
			}
			testServer := httptest.NewServer(http.HandlerFunc(handler))
			defer testServer.Close()

			uaa := cflib.UAA{
				URL:          testServer.URL,
				Username:     "testuser",
				Password:     "testpass",
				ClientID:     "testclient",
				ClientSecret: "testsecret",
			}

			token, err := uaa.GetAuthToken()
			Expect(err).ToNot(HaveOccurred())
			Expect(token).To(Equal("fake-token"))
		})

		It("returns error and empty token for unsuccessful POST", func() {
			handler := func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusNotFound)
			}
			testServer := httptest.NewServer(http.HandlerFunc(handler))
			defer testServer.Close()

			uaa := cflib.UAA{
				URL:          testServer.URL,
				Username:     "testuser",
				Password:     "testpass",
				ClientID:     "testclient",
				ClientSecret: "testsecret",
			}

			token, err := uaa.GetAuthToken()
			Expect(token).To(BeEmpty())
			Expect(err.Error()).To(ContainSubstring("response not 200"))
		})
	})
})
