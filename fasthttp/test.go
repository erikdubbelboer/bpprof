package main

import (
	"fmt"
	"log"
	"runtime"
	"strings"
	"time"

	_ "net/http/pprof"

	"github.com/erikdubbelboer/bpprof"
	"github.com/valyala/fasthttp"
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
	// Log all allocations
	runtime.MemProfileRate = 1

	go alloc()

	log.Println(
		fasthttp.ListenAndServe("0.0.0.0:6060",
			func(ctx *fasthttp.RequestCtx) {
				if strings.HasPrefix(string(ctx.Path()), "/debug/bpprof/heap") {
					runtime.GC() // Trigger a GC to get an accurate dump.
					bpprof.Heap(ctx, string(ctx.FormValue("sort")))
				} else {
					exampleHandler(ctx)
				}
			},
		),
	)
}

func exampleHandler(ctx *fasthttp.RequestCtx) {
	fmt.Fprintf(ctx, "Hello, world!\n\n")
}
