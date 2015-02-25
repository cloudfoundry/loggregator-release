set -e

go get -v github.com/onsi/gomega

`dirname $0`/test
