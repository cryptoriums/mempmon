package blocknative

import (
	"context"
	"crypto/tls"
	"encoding/hex"
	"encoding/json"

	"github.com/ethereum/go-ethereum/common"
	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/log/level"
	"github.com/gorilla/websocket"
	"github.com/pkg/errors"
)

const (
	ComponentName = "blocknative-mempmon"
)

// Message implements the TxPoolSource Message interface.
type Message struct {
	Version       int    `json:"version"`
	ServerVersion string `json:"serverVersion"`
	ConnectionID  string `json:"connectionId"`
	UserAgent     string `json:"userAgent"`
	Status        string `json:"status"`
	Reason        string `json:"reason"`
	Event         struct {
		CategoryCode string `json:"categoryCode"`
		EventCode    string `json:"eventCode"`
		DappID       string `json:"dappId"`
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
			Status         string `json:"status"`
			MonitorID      string `json:"monitorId"`
			MonitorVersion string `json:"monitorVersion"`
			// PendingTimeStamp   time.Time   `json:"pendingTimeStamp"`
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
	apiUrl  string
	apiKey  string
	logger  log.Logger
	conn    *websocket.Conn
	netName string
}

func (self *Mempool) Subscribe(ctx context.Context, contractAddress common.Address, methodName string) error {
	dialer := websocket.DefaultDialer
	dialer.TLSClientConfig = &tls.Config{
		InsecureSkipVerify: true,
	}
	conn, _, err := dialer.DialContext(ctx, self.apiUrl, nil)
	if err != nil {
		return err
	}

	self.conn = conn

	msg := `{
	"timeStamp": "2021-01-01T00:00:00.000Z",
	"categoryCode": "initialize",
	"eventCode": "checkDappId",
	"version": "1",
	"dappId": "` + self.apiKey + `",
	"blockchain": {
		"system": "ethereum",
		"network": "` + self.netName + `"
	 }
	}`
	err = WriteAndCheckResp(conn, msg)
	if err != nil {
		return err
	}

	msg = `{
		"timeStamp": "2021-01-01T00:00:00.000Z",
		"dappId": "` + self.apiKey + `",
		"version": "1",
		"blockchain": {
		   "system": "ethereum",
		   "network": "` + self.netName + `"
		},
		"categoryCode": "configs",
		"eventCode": "put",
		"config": {
		  "scope": "` + contractAddress.Hex() + `",
		  "filters": [{"contractCall.methodName":"` + methodName + `"}],
		  "watchAddress": true
		}
	}`

	err = WriteAndCheckResp(conn, msg)
	if err != nil {
		return err
	}
	level.Info(self.logger).Log("msg", "successful subscription to the memppool for TXs", "methodName", methodName)
	return nil
}

func WriteAndCheckResp(conn *websocket.Conn, msg string) error {
	err := conn.WriteMessage(websocket.TextMessage, []byte(msg))
	if err != nil {
		return err
	}

	_, resp, err := conn.ReadMessage()
	if err != nil {
		return errors.Errorf("read subscription message: %v", err)
	}

	respParser := &Message{}
	err = json.Unmarshal(resp, respParser)
	if err != nil {
		return errors.Errorf("parsing subscription message err:%v rawMsg:%v", err, string(resp))
	}

	if respParser.Status != "ok" {
		return errors.Errorf("subscription response error: %v", respParser.Reason)
	}
	return nil
}

func (self *Mempool) Read() (*Message, error) {
	_, msg, err := self.conn.ReadMessage()
	if err != nil {
		return nil, errors.Errorf("read message: %v", err)
	}

	msgParsed := &Message{}
	err = json.Unmarshal(msg, msgParsed)
	if err != nil {
		return nil, errors.Errorf("parsing message: %v", err)
	}
	return msgParsed, nil
}

func New(logger log.Logger, apiUrl, apiKey string, netID int) (*Mempool, error) {
	var netName string
	switch netID {
	case 1:
		netName = "main"
	case 4:
		netName = "rinkeby"
	default:
		return nil, errors.Errorf("network id not supported id:%v", netID)
	}

	return &Mempool{
		logger:  log.With(logger, "component", ComponentName),
		apiUrl:  apiUrl,
		apiKey:  apiKey,
		netName: netName,
	}, nil
}

func (self *Mempool) Close() {
	if self.conn != nil {
		if err := self.conn.Close(); err != nil {
			level.Error(self.logger).Log("msg", "closing connection", "err", err)
		}
	}
	level.Info(self.logger).Log("msg", "closed")
}
