package main

import (
	"crypto/tls"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"time"

	"github.com/bradylove/envstruct"
	"github.com/nu7hatch/gouuid"
)

type Config struct {
	Rate           float64       `env:"rate"`
	Time           time.Duration `env:"time"`
	LogFinURL      *url.URL      `env:"logfin_url,required"`
	Port           uint16        `env:"port,required"`
	SkipCertVerify bool          `env:"skip_cert_verify"`
}

func (c Config) Max() int {
	return int(float64(c.Rate) * c.Time.Seconds())
}

func (c Config) Delay() time.Duration {
	return time.Duration(1 / c.Rate * float64(time.Second))
}

func main() {
	conf := Config{
		Rate: 1000,
		Time: 5 * time.Minute,
	}

	err := envstruct.Load(&conf)
	if err != nil {
		log.Fatal(err)
	}

	uuid, err := uuid.NewV4()
	if err != nil {
		log.Fatal(err)
	}
	guid := uuid.String()

	go func() {
		if conf.Max() == 0 {
			printForever(guid, conf.Delay())
		}
		printUntil(guid, conf.Delay(), conf.Max())
		http.DefaultClient.Transport = &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: conf.SkipCertVerify,
				ServerName:         "127.0.0.1",
			},
		}
		_, err := http.Get(conf.LogFinURL.String() + "/count")
		if err != nil {
			log.Fatalf("Couldn't send done message to logfin: %s", err)
		}
	}()

	http.HandleFunc("/", func(rw http.ResponseWriter, r *http.Request) {
		rw.WriteHeader(200)
	})
	err = http.ListenAndServe(fmt.Sprintf(":%d", conf.Port), nil)
	if err != nil {
		log.Fatal(err)
	}
}

func printForever(guid string, delay time.Duration) {
	for i := 0; ; i++ {
		fmt.Printf("logemitter guid: %s msg: %d\n", guid, i)
		time.Sleep(delay)
	}
}

func printUntil(guid string, delay time.Duration, max int) {
	for i := 0; i < max; i++ {
		fmt.Printf("logemitter guid: %s msg: %d\n", guid, i)
		time.Sleep(delay)
	}
}
