package dopplerproxy_test

import (
	"errors"
	"github.com/cloudfoundry/gosteno"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"testing"
)

func TestDopplerProxy(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "DopplerProxy Suite")
}

type AuthorizerResult struct {
	Authorized   bool
	ErrorMessage string
}

type LogAuthorizer struct {
	TokenParam string
	Target     string
	Result     AuthorizerResult
}

func (a *LogAuthorizer) Authorize(authToken string, target string, l *gosteno.Logger) (bool, error) {
	a.TokenParam = authToken
	a.Target = target

	return a.Result.Authorized, errors.New(a.Result.ErrorMessage)
}

type AdminAuthorizer struct {
	TokenParam string
	Result     AuthorizerResult
}

func (a *AdminAuthorizer) Authorize(authToken string, l *gosteno.Logger) (bool, error) {
	a.TokenParam = authToken

	return a.Result.Authorized, errors.New(a.Result.ErrorMessage)
}
