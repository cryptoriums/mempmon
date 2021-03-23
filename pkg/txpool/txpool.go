package txpool

import (
	"github.com/ethereum/go-ethereum/common"
)

// Subscription will send an error message on TxPool subscription failure.
type Subscription interface {
	Err() <-chan error
}

// Message will be used to getting data from the message sent from the
// TxPoolSource implementation.
type Message interface {
	TxHash() string
	TxInput() ([]byte, error)
}

// TxPoolSource interface will be used for listening over ethereum network txpool data
// and will take care of the communication using internal implementation.
type TxPoolSource interface {
	// WatchTxPool will watch for ethereum txpool transactions, based on the provided contract address and
	// a methodName of the contract.
	WatchTxPool(contractAddress common.Address, methodName string) (Subscription, chan Message, error)
}
