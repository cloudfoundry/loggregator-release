package cflib_test

import (
	"net/http"
	"net/http/httptest"
	"tools/logcounterapp/cflib"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("CC", func() {
	Describe("GetAppName", func() {
		It("calls with required http headers", func() {
			var request *http.Request

			handler := func(w http.ResponseWriter, r *http.Request) {
				request = r
			}
			testServer := httptest.NewServer(http.HandlerFunc(handler))
			defer testServer.Close()

			cc := cflib.CC{
				URL: testServer.URL,
			}
			cc.GetAppName("testguid", "testtoken")

			Expect(request.URL.Path).To(Equal("/v2/apps/testguid"))

			expectedAuthHeader := "bearer testtoken"
			Expect(request.Header.Get("Authorization")).To(Equal(expectedAuthHeader))
		})

		It("returns guid if authToken is empty", func() {
			var requestMade bool

			handler := func(w http.ResponseWriter, r *http.Request) {
				requestMade = true
			}
			testServer := httptest.NewServer(http.HandlerFunc(handler))
			defer testServer.Close()

			cc := cflib.CC{
				URL: testServer.URL,
			}

			Expect(cc.GetAppName("testguid", "")).To(Equal("testguid"))
			Expect(requestMade).To(BeFalse())
		})

		It("returns guid if response code is not 200", func() {
			handler := func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusInternalServerError)
			}
			testServer := httptest.NewServer(http.HandlerFunc(handler))
			defer testServer.Close()

			cc := cflib.CC{
				URL: testServer.URL,
			}

			Expect(cc.GetAppName("testguid", "testtoken")).To(Equal("testguid"))
		})

		It("returns the app name", func() {
			handler := func(w http.ResponseWriter, r *http.Request) {
				w.Write([]byte(`{"entity":{"name":"testapp"}}`))
			}
			testServer := httptest.NewServer(http.HandlerFunc(handler))
			defer testServer.Close()

			cc := cflib.CC{
				URL: testServer.URL,
			}

			Expect(cc.GetAppName("testguid", "testtoken")).To(Equal("testapp"))
		})
	})
})
