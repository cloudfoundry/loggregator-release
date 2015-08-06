package helpers
import (
	"github.com/cloudfoundry-incubator/cf-test-helpers/cf"
	"time"
	"encoding/json"
	"strings"
	"github.com/cloudfoundry-incubator/cf-test-helpers/generator"
	"github.com/onsi/gomega/gexec"
	"regexp"
	. "github.com/onsi/gomega"
)

type apiInfo struct {
	DopplerLoggingEndpoint string `json:"doppler_logging_endpoint"`
}

func GetDopplerEndpoint() string {
	info := apiInfo{}
	ccInfo := cf.Cf("curl", "/v2/info").Wait(5 * time.Second)
	json.Unmarshal(ccInfo.Out.Contents(), &info)
	return info.DopplerLoggingEndpoint
}

func GetAppGuid(appName string) string {
	appGuid := cf.Cf("app", appName, "--guid").Wait(5 * time.Second).Out.Contents()
	return strings.TrimSpace(string(appGuid))
}

func PushApp() string {
	appName := generator.PrefixedRandomName("LATS-App-")
	appPush := cf.Cf("push", appName, "-p", "assets/dora").Wait(60 * time.Second)
	Expect(appPush).To(gexec.Exit(0))

	return appName
}

func FetchOAuthToken() string {
	reg := regexp.MustCompile(`(bearer.*)`)
	output := string(cf.Cf("oauth-token").Wait().Out.Contents())
	authToken := reg.FindString(output)
	return authToken
}