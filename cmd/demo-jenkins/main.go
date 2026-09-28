// demo-jenkins serves a simulated Jenkins for docs recordings and for
// trying jenklod-batman without a real controller:
//
//	go run ./cmd/demo-jenkins &
//	JENKINS_TOKEN=demo jenklod-batman --config docs/tapes/demo.toml
package main

import (
	"flag"
	"log"
	"net/http"

	"github.com/krank56/jenklod-batman/internal/demo"
)

func main() {
	addr := flag.String("addr", "127.0.0.1:8765", "listen address")
	flag.Parse()
	log.Printf("demo Jenkins on http://%s (user bruce, any token)", *addr)
	log.Fatal(http.ListenAndServe(*addr, demo.New()))
}
