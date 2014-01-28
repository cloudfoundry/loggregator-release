package store

import (
	"crypto/sha1"
	"fmt"
)

type AppService struct {
	AppId string
	Url   string
}

func (a AppService) Id() string {
	hash := sha1.Sum([]byte(a.Url))
	return fmt.Sprintf("%x", hash)
}

type AppServices struct {
	AppId string
	Urls  []string
}
