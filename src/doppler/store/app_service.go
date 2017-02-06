package store

type AppService interface {
	AppId() string
	Url() string
	Hostname() string
	Id() string
}
