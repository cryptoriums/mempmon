package main

import (
	"crypto/tls"
	"fmt"
	"log"
	"os"
	"os/signal"
	"time"

	"github.com/gorilla/websocket"
	"github.com/joho/godotenv"
)

func connect() (*websocket.Conn, error) {
	tlsConfig := &tls.Config{
		InsecureSkipVerify: true,
	}
	dialer := websocket.DefaultDialer
	dialer.TLSClientConfig = tlsConfig
	wsSubscriber, _, err := dialer.Dial("wss://api.blocknative.com/v0", nil)
	if err != nil {
		return nil, err
	}

	initMsg := `{
	"categoryCode": "initialize",
	"eventCode": "checkDappId",
	"version": "1",
	"timeStamp": "2021-01-01T00:00:00.000Z",
	"dappId": "` + os.Getenv("DAPP_ID") + `",
	"blockchain": {
		"system": "ethereum",
		"network": "main"
	 }
	}`
	configMsg := `{
		"timeStamp": "2021-01-01T00:00:00.000Z",
		"dappId": "` + os.Getenv("DAPP_ID") + `",
		"version": "1",
		"blockchain": {
		   "system": "ethereum",
		   "network": "main"
		},
		"categoryCode": "configs",
		"eventCode": "put",
		"config": {
		  "scope": "0x88df592f8eb5d7bd38bfef7deb0fbc02cf3778a0",
		  "filters": [{"contractCall.methodName":"submitMiningSolution"}],
		  "watchAddress": true
		}
	}`

	err = wsSubscriber.WriteMessage(websocket.TextMessage, []byte(initMsg))
	if err != nil {
		return nil, err
	}

	err = wsSubscriber.WriteMessage(websocket.TextMessage, []byte(configMsg))
	if err != nil {
		return nil, err
	}
	return wsSubscriber, nil
}

func read(wsSubscriber *websocket.Conn, done chan struct{}) {
	defer close(done)
	for {
		_, nextNotification, err := wsSubscriber.ReadMessage()
		if err != nil {
			fmt.Println(err)
			return
		}
		// Here is the response.
		fmt.Println(string(nextNotification))
	}
}

var (
	ws *websocket.Conn
)

func main() {
	err := godotenv.Load()
	if err != nil {
		log.Fatal("Error loading .env file")
	}

	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt)

	ws, err = connect()
	if err != nil {
		panic(err)
	}
	defer ws.Close()

	done := make(chan struct{})
	go read(ws, done)

	for {
		select {
		case <-done:
			for {
				select {
				case <-interrupt:
					return
				default:
				}
				ws, err = connect()
				if err != nil {
					log.Printf("while reconnecting to websocket: %v", err)
					time.Sleep(time.Second)
					continue
				}
				go read(ws, done)
				break
			}
		case <-interrupt:
			log.Println("interrupt")
			// Cleanly close the connection by sending a close message and then
			// waiting (with timeout) for the server to close the connection.
			err = ws.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
			if err != nil {
				log.Println("write close:", err)
				return
			}
			select {
			case <-done:
			case <-time.After(time.Second):
			}
			return
		}
	}
}
