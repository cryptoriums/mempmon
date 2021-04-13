package blocknative

import (
	"context"
	"crypto/tls"
	"encoding/json"

	"github.com/bonedaddy/go-blocknative/client"
	"github.com/ethereum/go-ethereum/common"
	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/log/level"
	"github.com/gorilla/websocket"
	"github.com/pkg/errors"
)

const (
	ComponentName = "blocknative-mempmon"
)

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
		  "abi": [{
			"constant": false,
			"inputs": [
				{"name": "_nonce", "type": "string"},
				{"name": "_requestIds", "type": "uint256[5]"},
				{"name": "_values", "type": "uint256[5]"}
			],
			"name": "submitMiningSolution",
			"outputs": [],
			"payable": false,
			"stateMutability": "nonpayable",
			"type": "function"
		  }],
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

	respParser := &client.EthTxPayload{}
	err = json.Unmarshal(resp, respParser)
	if err != nil {
		return errors.Errorf("parsing subscription message err:%v rawMsg:%v", err, string(resp))
	}

	if respParser.Status != "ok" {
		return errors.Errorf("subscription response error")
	}
	return nil
}

func (self *Mempool) Read() (*client.EthTxPayload, error) {
	_, msg, err := self.conn.ReadMessage()
	if err != nil {
		return nil, errors.Errorf("read message: %v", err)
	}

	msgParsed := &client.EthTxPayload{}
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
