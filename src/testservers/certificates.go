package testservers

//go:generate ../../scripts/generate-loggregator-certs no-bbs-ca
//go:generate go-bindata -nocompress -pkg testservers -prefix loggregator-certs/ loggregator-certs/

import (
	"io/ioutil"
	"log"
)

func Cert(filename string) string {
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
