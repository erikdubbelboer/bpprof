// +build ignore

package main

import (
	"log"
	"net/http"
	"time"

	_ "net/http/pprof"

	_ "github.com/erikdubbelboer/bpprof/http"
)

var memSink interface{}

func alloc() {
	for {
		time.Sleep(time.Second)

		for i := 0; i < 1024; i++ {
			memSink = make([]byte, 1024)
		}
	}
}

func main() {
	go alloc()
	log.Println(http.ListenAndServe("0.0.0.0:6060", nil))
}
