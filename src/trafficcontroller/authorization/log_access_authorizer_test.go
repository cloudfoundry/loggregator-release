package authorization_test

import (
	"trafficcontroller/authorization"
	"bytes"
	"github.com/cloudfoundry/loggregatorlib/loggertesthelper"
	"net/http"
	"net/http/httptest"
	"os"
	"regexp"
	"runtime/pprof"
	"testing"
	"time"
)

var accessTests = []struct {
	target         string
	authToken      string
	expectedResult bool
}{
	//Allowed domains
	{
		"myAppId",
		"bearer something",
		true,
	},
	//Not allowed stuff
	{
		"notMyAppId",
		"bearer something",
		false,
	},
	{
		"nonExistantAppId",
		"bearer something",
		false,
	},
}

func TestUserRoleAccessCombinations(t *testing.T) {
	server := startHTTPServer()
	defer server.Close()
	for i, test := range accessTests {
		authorizer := authorization.NewLogAccessAuthorizer(server.URL, true)
		result := authorizer(test.authToken, test.target, loggertesthelper.Logger())
		if result != test.expectedResult {
			t.Errorf("Access combination %d failed.", i)
		}
	}
}

func TestWorksIfServerIsSSLWithoutValidCertAndSkipVerifyCertIsTrue(t *testing.T) {
	logger := loggertesthelper.Logger()
	server := startHTTPSServer()
	defer server.Close()

	authorizer := authorization.NewLogAccessAuthorizer(server.URL, true)
	result := authorizer("bearer something", "myAppId", logger)
	if result != true {
		t.Errorf("Could not connect to secure server.")
	}

	authorizer = authorization.NewLogAccessAuthorizer(server.URL, false)
	result = authorizer("bearer something", "myAppId", logger)
	if result != false {
		t.Errorf("Should not be able to connect to secure server with a self signed cert if SkipVerifyCert is false.")
	}
}

func TestThatThereIsNoLeakingGoRoutine(t *testing.T) {
	logger := loggertesthelper.Logger()
	server := startHTTPServer()
	defer server.Close()

	authorizer := authorization.NewLogAccessAuthorizer(server.URL, true)
	authorizer("bearer something", "myAppId", logger)
	time.Sleep(10 * time.Millisecond)

	var buf bytes.Buffer
	goRoutineProfiles := pprof.Lookup("goroutine")
	goRoutineProfiles.WriteTo(&buf, 2)

	match, err := regexp.Match("readLoop", buf.Bytes())
	if err != nil {
		t.Error("Unable to match /readLoop/ regexp against goRoutineProfile")
		goRoutineProfiles.WriteTo(os.Stdout, 2)
	}

	if match {
		t.Error("We are leaking readLoop goroutines.")
	}

	match, err = regexp.Match("writeLoop", buf.Bytes())
	if err != nil {
		t.Error("Unable to match /writeLoop/ regexp against goRoutineProfile")
	}

	if match {
		t.Error("We are leaking writeLoop goroutines.")
		goRoutineProfiles.WriteTo(os.Stdout, 2)
	}
}

type handler struct{}

func (h *handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	re := regexp.MustCompile("^/v2/apps/([^/?]+)$")
	result := re.FindStringSubmatch(r.URL.Path)
	if len(result) != 2 {
		w.WriteHeader(500)
		return
	}

	switch result[1] {
	case "myAppId":
		w.Write([]byte("{}"))
	case "notMyAppId":
		w.WriteHeader(403)
	default:
		w.WriteHeader(404)
	}
}

func startHTTPServer() *httptest.Server {
	return httptest.NewServer(new(handler))
}

func startHTTPSServer() *httptest.Server {
	return httptest.NewTLSServer(new(handler))
}
