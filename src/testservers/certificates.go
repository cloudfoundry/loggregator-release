package testservers

//go:generate bash -c "../../scripts/generate-loggregator-certs no-bbs-ca && go-bindata -nocompress -pkg testservers -prefix loggregator-certs/ loggregator-certs/"

import (
	"io/ioutil"
	"log"
)

func MetronCertPath() string {
	return createTempFile(MustAsset("metron.crt"))
}

func MetronKeyPath() string {
	return createTempFile(MustAsset("metron.key"))
}

func DopplerCertPath() string {
	return createTempFile(MustAsset("doppler.crt"))
}

func DopplerKeyPath() string {
	return createTempFile(MustAsset("doppler.key"))
}

func TrafficControllerCertPath() string {
	return createTempFile(MustAsset("trafficcontroller.crt"))
}

func TrafficControllerKeyPath() string {
	return createTempFile(MustAsset("trafficcontroller.key"))
}

func SyslogDrainBinderCertPath() string {
	return createTempFile(MustAsset("syslogdrainbinder.crt"))
}

func SyslogDrainBinderKeyPath() string {
	return createTempFile(MustAsset("syslogdrainbinder.key"))
}

func CACertPath() string {
	return createTempFile(MustAsset("loggregator-ca.crt"))
}

func createTempFile(contents []byte) string {
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
