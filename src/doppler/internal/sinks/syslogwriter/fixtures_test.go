package syslogwriter_test

import (
	"io/ioutil"
	"log"
)

//go:generate go-bindata -nocompress -o bindata_test.go -pkg syslogwriter_test -prefix fixtures/ fixtures/

func fixture(filename string) string {
	contents := MustAsset(filename)

	tmpfile, err := ioutil.TempFile("", "")
	if err != nil {
		log.Fatal(err)
	}

	if _, err := tmpfile.Write(contents); err != nil {
		log.Fatal(err)
	}
	if err := tmpfile.Close(); err != nil {
		log.Fatal(err)
	}

	return tmpfile.Name()
}
