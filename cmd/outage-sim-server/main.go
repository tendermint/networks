package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"

	"github.com/tendermint/networks/cmd/outage-sim-server/internal"
)

func main() {
	var (
		addr = flag.String("addr", ":34000", "the address to which to bind this server")
	)
	flag.Usage = func() {
		fmt.Println(`Tendermint outage simulator server

Provides an HTTP interface through which one can bring a local Tendermint
service down or up.

NOTE: This requires root privileges to allow this process to interact with the
Tendermint service. This is a SECURITY RISK and thus this application must only
be used for testing purposes and with careful network restrictions as to who
can access this service.

Usage:
  outage-sim-server -addr 127.0.0.1:34000

Flags:`)
		flag.PrintDefaults()
		fmt.Println(`
Examples of how to bring Tendermint up/down:
  curl -s -X POST -d "up" http://127.0.0.1:34000
  curl -s -X POST -d "down" http://127.0.0.1:34000`)
		fmt.Println("")
	}
	flag.Parse()

	http.HandleFunc(
		"/",
		internal.MakeOutageEndpointHandler(
			internal.IsTendermintRunning,
			internal.ExecuteServiceCmd,
		),
	)
	log.Fatal(http.ListenAndServe(*addr, nil))
}
