package main

import (
	"fmt"
	"log"
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
	go alloc()

	log.Println(
		fasthttp.ListenAndServe("0.0.0.0:6060",
			func(ctx *fasthttp.RequestCtx) {
				if strings.HasPrefix(string(ctx.Path()), "/debug") {
					bpprof.Heap(ctx, string(ctx.FormValue("sort")))
				} else {
					requestHandler(ctx)
				}
			},
		),
	)
}

func requestHandler(ctx *fasthttp.RequestCtx) {
	fmt.Fprintf(ctx, "Hello, world!\n\n")

	fmt.Fprintf(ctx, "Request method is %q\n", ctx.Method())
	fmt.Fprintf(ctx, "RequestURI is %q\n", ctx.RequestURI())
	fmt.Fprintf(ctx, "Requested path is %q\n", ctx.Path())
	fmt.Fprintf(ctx, "Host is %q\n", ctx.Host())
	fmt.Fprintf(ctx, "Query string is %q\n", ctx.QueryArgs())
	fmt.Fprintf(ctx, "User-Agent is %q\n", ctx.UserAgent())
	fmt.Fprintf(ctx, "Connection has been established at %s\n", ctx.ConnTime())
	fmt.Fprintf(ctx, "Request has been started at %s\n", ctx.Time())
	fmt.Fprintf(ctx, "Serial request number for the current connection is %d\n", ctx.ConnRequestNum())
	fmt.Fprintf(ctx, "Your ip is %q\n\n", ctx.RemoteIP())

	fmt.Fprintf(ctx, "Raw request is:\n---CUT---\n%s\n---CUT---", &ctx.Request)

	ctx.SetContentType("text/plain; charset=utf8")

	// Set arbitrary headers
	ctx.Response.Header.Set("X-My-Header", "my-header-value")

	// Set cookies
	var c fasthttp.Cookie
	c.SetKey("cookie-name")
	c.SetValue("cookie-value")
	ctx.Response.Header.SetCookie(&c)
}
