package txpool

import (
	"crypto/tls"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/cryptoriums/mempmon/pkg/config"
	"github.com/ethereum/go-ethereum/common"
	"github.com/gorilla/websocket"
)

func (s *BlockNativeTxPool) Err() chan error {
	return s.errChan
}

// BlockNativeMessage implements the TxPoolSource Message interface.
type BlockNativeMessage struct {
	Version       int       `json:"version"`
	ServerVersion string    `json:"serverVersion"`
	TimeStamp     time.Time `json:"timeStamp"`
	ConnectionID  string    `json:"connectionId"`
	UserAgent     string    `json:"userAgent"`
	Status        string    `json:"status"`
	Event         struct {
		TimeStamp    time.Time `json:"timeStamp"`
		CategoryCode string    `json:"categoryCode"`
		EventCode    string    `json:"eventCode"`
		DappID       string    `json:"dappId"`
		Blockchain   struct {
			System  string `json:"system"`
			Network string `json:"network"`
		} `json:"blockchain"`
		ContractCall struct {
			ContractType    string `json:"contractType"`
			ContractAddress string `json:"contractAddress"`
			MethodName      string `json:"methodName"`
			Params          struct {
				To    string `json:"_to"`
				Value string `json:"_value"`
			} `json:"params"`
			ContractDecimals int    `json:"contractDecimals"`
			ContractName     string `json:"contractName"`
			DecimalValue     string `json:"decimalValue"`
		} `json:"contractCall"`
		Transaction struct {
			Status             string      `json:"status"`
			MonitorID          string      `json:"monitorId"`
			MonitorVersion     string      `json:"monitorVersion"`
			PendingTimeStamp   time.Time   `json:"pendingTimeStamp"`
			PendingBlockNumber int         `json:"pendingBlockNumber"`
			Hash               string      `json:"hash"`
			From               string      `json:"from"`
			To                 string      `json:"to"`
			Value              string      `json:"value"`
			Gas                int         `json:"gas"`
			GasPrice           string      `json:"gasPrice"`
			GasPriceGwei       int         `json:"gasPriceGwei"`
			Nonce              int         `json:"nonce"`
			BlockHash          interface{} `json:"blockHash"`
			BlockNumber        interface{} `json:"blockNumber"`
			Input              string      `json:"input"`
			Asset              string      `json:"asset"`
			WatchedAddress     string      `json:"watchedAddress"`
			Direction          string      `json:"direction"`
			Counterparty       string      `json:"counterparty"`
		} `json:"transaction"`
	} `json:"event"`
}

func (bt *BlockNativeMessage) TxInputData() ([]byte, error) {
	return hex.DecodeString(bt.Event.Transaction.Input)
}

func (bt *BlockNativeMessage) TxHash() string {
	return bt.Event.Transaction.Hash
}

// BlockNativeTxPool implements TxPoolInterface.
type BlockNativeTxPool struct {
	*websocket.Conn
	msg     chan *BlockNativeMessage
	errChan chan error
}

func (b *BlockNativeTxPool) subscribe(contractAddress common.Address, methodName string) error {
	tlsConfig := &tls.Config{
		InsecureSkipVerify: true,
	}
	dialer := websocket.DefaultDialer
	dialer.TLSClientConfig = tlsConfig
	wsSubscriber, _, err := dialer.Dial(os.Getenv(config.BlocknativeWSURL), nil)
	if err != nil {
		return err
	}

	initMsg := `{
	"categoryCode": "initialize",
	"eventCode": "checkDappId",
	"version": "1",
	"timeStamp": "2021-01-01T00:00:00.000Z",
	"dappId": "` + os.Getenv(config.BlocknativeDappID) + `",
	"blockchain": {
		"system": "ethereum",
		"network": "main"
	 }
	}`
	configMsg := `{
		"timeStamp": "2021-01-01T00:00:00.000Z",
		"dappId": "` + os.Getenv(config.BlocknativeDappID) + `",
		"version": "1",
		"blockchain": {
		   "system": "ethereum",
		   "network": "main"
		},
		"categoryCode": "configs",
		"eventCode": "put",
		"config": {
		  "scope": "` + contractAddress.Hex() + `",
		  "filters": [{"contractCall.methodName":"` + methodName + `"}],
		  "watchAddress": true
		}
	}`

	err = wsSubscriber.WriteMessage(websocket.TextMessage, []byte(initMsg))
	if err != nil {
		return err
	}

	err = wsSubscriber.WriteMessage(websocket.TextMessage, []byte(configMsg))
	if err != nil {
		return err
	}
	b.Conn = wsSubscriber
	return nil
}

func (b *BlockNativeTxPool) read() {
	// Close the connection on the exit.
	defer b.Conn.Close()

	for {
		_, nextNotification, err := b.Conn.ReadMessage()
		if err != nil {
			fmt.Printf("while read message from the ws connection: %v", err)
			b.errChan <- err
			return
		}

		msg := &BlockNativeMessage{}
		err = json.Unmarshal(nextNotification, msg)
		if err != nil {
			fmt.Printf("while parsing message from the ws connection: %v", err)
			b.errChan <- err
			return
		}
		b.msg <- msg
	}
}

func NewBlocknativeTxPool() (b *BlockNativeTxPool, err error) {
	return &BlockNativeTxPool{errChan: make(chan error), msg: make(chan *BlockNativeMessage)}, nil
}

// func (b *BlockNativeTxPool) Close() error {
// 	// Cleanly close the connection by sending a close message.
// 	err := b.Conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
// 	if err != nil {
// 		return err
// 	}
// 	return nil
// }

func (b *BlockNativeTxPool) WatchTxPool(contractAddress common.Address, methodName string) (*BlockNativeTxPool, <-chan *BlockNativeMessage, error) {
	err := b.subscribe(contractAddress, methodName)
	if err != nil {
		return nil, nil, err
	}

	// Reading from the websocket connection forever.
	go b.read()
	return b, b.msg, nil
}
