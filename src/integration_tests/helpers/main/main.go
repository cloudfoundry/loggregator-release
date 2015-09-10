package main
import (
//	"integration_tests/helpers"
//	"runtime"
//	"net/http"
//	"github.com/cloudfoundry/storeadapter/etcdstoreadapter"
//	"github.com/cloudfoundry/gunk/workpool"
//	"fmt"
	"sync"
	"time"
)


//func main() {
////	app := helpers.NewFakeApp("appID", 100)
////	go app.Start()
//
//	runtime.GOMAXPROCS(runtime.NumCPU())
////	workpool, _ := workpool.NewWorkPool(1)
////	etcdAdapter := etcdstoreadapter.NewETCDStoreAdapter([]string {}, workpool)
////	waitOnURL("http://localhost:4001")
////
////	registrar := helpers.NewSyslogRegistrar(etcdAdapter)
////	registrar.Register("appID", "localhost:5555")
//
//	server := helpers.NewSyslogTCPServer("localhost:5555")
//	server.Start()
//}
//
//func waitOnURL(url string) {
//	_, err := http.Get(url)
//	if err != nil {
//		panic(err.Error())
//	}
//}


type Waiter struct {
	wg sync.WaitGroup
}

func (w *Waiter) Start() {
	w.wg.Add(1)
	time.Sleep(1*time.Second)
	w.wg.Done()
}

func (w *Waiter) Stop() {
	w.wg.Wait()
}

func main() {
	var w Waiter
	go w.Start()
	w.Stop()
}
