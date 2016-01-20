@echo on
go version
SET PATH=%GOROOT%;%PATH%;%DEVENV_PATH%

SET GOBIN=%CD%\bin
SET PATH=%GOBIN%;%PATH%
:: Install metron, it contains all relevant gocode inside itself.
SET GOPATH=%CD%
go install github.com/onsi/ginkgo/ginkgo || exit /b 1
ginkgo -r -noColor src\metron || exit /b 1
go install metron || exit /b 1
