package dopplerproxy_test

import (
	"errors"
	"log"
	"net/http"
	"testing"

	"github.com/cloudfoundry/gosteno"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestDopplerProxy(t *testing.T) {
	log.SetOutput(GinkgoWriter)
	RegisterFailHandler(Fail)
	RunSpecs(t, "DopplerProxy Suite")
}

type AuthorizerResult struct {
	Status       int
	ErrorMessage string
}

type LogAuthorizer struct {
	TokenParam string
	Target     string
	Result     AuthorizerResult
}

func (a *LogAuthorizer) Authorize(authToken string, target string, l *gosteno.Logger) (int, error) {
	a.TokenParam = authToken
	a.Target = target

	return a.Result.Status, errors.New(a.Result.ErrorMessage)
}

type AdminAuthorizer struct {
	TokenParam string
	Result     AuthorizerResult
}

func (a *AdminAuthorizer) Authorize(authToken string, l *gosteno.Logger) (bool, error) {
	a.TokenParam = authToken

	return a.Result.Status == http.StatusOK, errors.New(a.Result.ErrorMessage)
}
