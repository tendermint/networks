package main

import (
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os/exec"
)

func isTendermintRunning() bool {
	cmd := exec.Command("pidof", "tendermint")
	return cmd.Run() == nil
}

func executeServiceCmd(status string) error {
	cmd := exec.Command("service", "tendermint", status)
	return cmd.Run()
}

func main() {
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "POST" {
			if r.Body != nil {
				body, err := ioutil.ReadAll(r.Body)
				if err != nil {
					w.WriteHeader(http.StatusInternalServerError)
					fmt.Fprint(w, "Internal server error while reading request body")
				}
				switch string(body) {
				case "up":
					if !isTendermintRunning() {
						if err := executeServiceCmd("start"); err != nil {
							w.WriteHeader(http.StatusInternalServerError)
							fmt.Fprint(w, "Failed to start Tendermint process")
						} else {
							w.WriteHeader(http.StatusOK)
							fmt.Fprint(w, "Service successfully started")
						}
					} else {
						w.WriteHeader(http.StatusOK)
						fmt.Fprint(w, "Service is already running")
					}
				case "down":
					if isTendermintRunning() {
						if err := executeServiceCmd("stop"); err != nil {
							w.WriteHeader(http.StatusInternalServerError)
							fmt.Fprint(w, "Failed to stop Tendermint process")
						} else {
							w.WriteHeader(http.StatusOK)
							fmt.Fprint(w, "Service successfully stopped")
						}
					} else {
						w.WriteHeader(http.StatusOK)
						fmt.Fprint(w, "Service is already stopped")
					}
				default:
					w.WriteHeader(http.StatusBadRequest)
					fmt.Fprint(w, "Unrecognised command")
				}
			} else {
				w.WriteHeader(http.StatusBadRequest)
				fmt.Fprint(w, "Missing command in request")
			}
		} else {
			w.WriteHeader(http.StatusMethodNotAllowed)
			fmt.Fprintf(w, "Unsupported method: %s", r.Method)
		}
	})
	log.Fatal(http.ListenAndServe(":34000", nil))
}
