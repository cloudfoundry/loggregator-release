package main

import (
	"flag"
	"fmt"
)

var version = flag.Bool("version", false, "Version info")

const versionNumber = `0.0.1.`
const gitSha = ``

func main() {
	flag.Parse()

	if *version {
		fmt.Printf("\n\nversion: %s\ngitSha: %s\n\n", versionNumber, gitSha)
		return
	}
}
