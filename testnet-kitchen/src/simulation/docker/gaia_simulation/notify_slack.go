package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
)

type Message struct {
	Channel string `json:"channel"`
	Thread  string `json:"thread_ts,omitempty"`
	Text    string `json:"text"`
	//	Payload Payload
}

/*
type Payload struct {
	Blocks []Block `json:"blocks"`
}

type Block struct {
	Type string `json:"type"`
	Text *Text  `json:"text,omitempty"`
}

type Text struct {
	Type string `json:"type"`
	Text string `json:"text"`
}*/

func (r *Message) Marshal() ([]byte, error) {
	return json.Marshal(r)
}

func main() {
	fail_message := "Simulation with *seed " + os.Getenv("SEED") + "* failed! To replicate, run the command:  ```" +
		`go test ./cmd/gaia/app -run TestFullGaiaSimulation \
								-SimulationEnabled=true \
								-SimulationNumBlocks=` + os.Getenv("BLOCKS") + ` \
								-SimulationVerbose=true \
								-SimulationCommit=true \
								-SimulationSeed=` + os.Getenv("SEED") + ` \
								-v -timeout 24h` + "```"

	var message string

	if os.Args[2] == "0" {
		message = "Seed " + os.Getenv("SEED") + " *PASS*"
	} else {
		message = fail_message
	}

	//	text := &Text{"mrkdwn", message}
	//	block := Block{"section", text}
	//	divider := Block{"divider", nil}
	//	payload := Payload{[]Block{block, divider}}
	msg := Message{os.Getenv("SLACK_CHANNEL_ID"), os.Getenv("SLACK_MSG_TS"), message}

	encoded_payload, err := json.Marshal(msg)
	if err != nil {
		panic(err)
	}

	api_url := os.Args[1]

	u, _ := url.ParseRequestURI(api_url)
	url_str := u.String()

	req, _ := http.NewRequest("POST", url_str, bytes.NewBuffer(encoded_payload))
	req.Header.Set("Content-Type", "application/json;charset=UTF-8")
	req.Header.Set("Authorization", "Bearer "+os.Getenv("SLACK_TOKEN"))

	client := &http.Client{}
	resp, err := client.Do(req)

	if err != nil {
		panic(err)
	}

	defer resp.Body.Close()

	body, _ := ioutil.ReadAll(resp.Body)
	fmt.Println("response Body:", string(body))
}
