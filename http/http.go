package http

import (
	"net/http"

	"github.com/erikdubbelboer/bpprof"
)

func init() {
	http.Handle("/debug/bpprof/heap", http.HandlerFunc(heap))
}

func heap(w http.ResponseWriter, r *http.Request) {
	bpprof.Heap(w, r.FormValue("sort"))
}
