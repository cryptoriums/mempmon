package blocknative

import (
	"context"
	"crypto/tls"
	"encoding/hex"
	"encoding/json"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/log/level"
	"github.com/gorilla/websocket"
	"github.com/pkg/errors"
)

const ComponentName = "blocknative-mempmon"

// Message implements the TxPoolSource Message interface.
type Message struct {
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

func (self *Message) TxInputData() ([]byte, error) {
	return hex.DecodeString(self.Event.Transaction.Input)
}

func (self *Message) TxHash() string {
	return self.Event.Transaction.Hash
}

// Mempool implements TxPoolInterface.
type Mempool struct {
	apiUrl string
	apiKey string
	logger log.Logger
	*websocket.Conn
}

func (self *Mempool) Subscribe(ctx context.Context, contractAddress common.Address, methodName string) error {
	dialer := websocket.DefaultDialer
	dialer.TLSClientConfig = &tls.Config{
		InsecureSkipVerify: true,
	}
	wsSubscriber, _, err := dialer.DialContext(ctx, self.apiUrl, nil)
	if err != nil {
		return err
	}

	initMsg := `{
	"categoryCode": "initialize",
	"eventCode": "checkDappId",
	"version": "1",
	"timeStamp": "2021-01-01T00:00:00.000Z",
	"dappId": "` + self.apiKey + `",
	"blockchain": {
		"system": "ethereum",
		"network": "main"
	 }
	}`
	configMsg := `{
		"timeStamp": "2021-01-01T00:00:00.000Z",
		"dappId": "` + self.apiKey + `",
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

	self.Conn = wsSubscriber

	return nil
}

func (self *Mempool) Read() (*Message, error) {
	for {
		_, nextNotification, err := self.Conn.ReadMessage()
		if err != nil {
			return nil, errors.Errorf("read message from the ws connection: %v", err)
		}

		msg := &Message{}
		err = json.Unmarshal(nextNotification, msg)
		if err != nil {
			return nil, errors.Errorf("parsing message from the ws connection: %v", err)
		}
		if msg.TxHash() == "" { // For some rason the first few messages are empty.
			continue
		}
		return msg, nil
	}
}

func New(logger log.Logger, apiUrl, apiKey string) (*Mempool, error) {
	return &Mempool{
		logger: log.With(logger, "component", ComponentName),
		apiUrl: apiUrl,
		apiKey: apiKey,
	}, nil
}

func (self *Mempool) Close() {
	if self.Conn.Close() != nil {
		if err := self.Conn.Close(); err != nil {
			level.Error(self.logger).Log("msg", "closing grpc connection", "err", err)
		}
	}
	level.Info(self.logger).Log("msg", "closed")
}
