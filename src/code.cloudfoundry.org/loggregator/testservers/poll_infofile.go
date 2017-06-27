package testservers

import (
	"log"
	"time"

	ini "github.com/go-ini"
)

func InfoPollString(path, name, key string) string {
	return infoPoll(path, name, key).String()
}

func InfoPollInt(path, name, key string) int {
	r := infoPoll(path, name, key)
	v, err := r.Int()
	if err != nil {
		log.Panicf("key is not an int: %s %#v", key, r)
	}
	return v
}

func infoPoll(path, name, key string) *ini.Key {
	done := time.After(5 * time.Second)
	for {
		k, err := infoRead(path, name, key)
		if err == nil {
			return k
		}
		select {
		case <-done:
			log.Panicf("timed out waiting for key to exist in infofile: %s, %s, %s", key, name, path)
		case <-time.After(100 * time.Millisecond):
		}
	}
}

func infoRead(path, name, key string) (*ini.Key, error) {
	iniFile, err := ini.Load(path)
	if err != nil {
		return nil, err
	}

	section, err := iniFile.GetSection(name)
	if err != nil {
		return nil, err
	}

	k, err := section.GetKey(key)
	if err != nil {
		return nil, err
	}

	return k, nil
}
