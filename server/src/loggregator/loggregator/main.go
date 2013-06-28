package main

import (
	"flag"
	"fmt"
)

var version = flag.Bool("version", false, "Version info")

var versionNumber = `0.0.1.TRAVIS_BUILD_NUMBER`
var gitSha = `TRAVIS_COMMIT`

func main() {
	flag.Parse()

	if *version {
		fmt.Printf("\n\nversion: %s\ngitSha: %s\n\n", versionNumber, gitSha)
		return
	}
}
