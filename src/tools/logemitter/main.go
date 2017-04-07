package main

import (
	"crypto/tls"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"time"

	uuid "github.com/nu7hatch/gouuid"
)

type Config struct {
	Rate      float64
	Time      time.Duration
	LogFinURL *url.URL
	Port      int
}

func (c Config) Max() int {
	return int(c.Rate * c.Time.Seconds())
}

func (c Config) Delay() time.Duration {
	return time.Duration(1 / c.Rate * float64(time.Second))
}

func main() {
	url, err := url.Parse(os.Getenv("LOGFIN_URL"))
	if err != nil {
		log.Fatal(err)
	}

	port, err := strconv.Atoi(os.Getenv("PORT"))
	if err != nil {
		log.Fatal(err)
	}

	conf := Config{
		Rate:      100,
		Time:      500 * time.Millisecond,
		LogFinURL: url,
		Port:      port,
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
				ServerName: "127.0.0.1",
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
