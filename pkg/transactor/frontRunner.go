package frontrunner

import (
	"context"
	"fmt"
	"log"
	"math/big"
	"strings"

	"github.com/cryptoriums/mempmon/pkg/txpool"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
)

type Transactor interface {
	Transact(context.Context, string, [5]*big.Int, [5]*big.Int) (*types.Transaction, *types.Receipt, error)
}

// FrontRunner implements Transactor interface.
type FrontRunner struct {
	currentTx      context.Context
	closeCurrentTx context.CancelFunc
	f              func(context.Context, string, [5]*big.Int, [5]*big.Int) (*types.Transaction, *types.Receipt, error)
}

func (f FrontRunner) Transact(ctx context.Context, nonce string, reqIds [5]*big.Int, reqVals [5]*big.Int) (*types.Transaction, *types.Receipt, error) {
	if f.currentTx != nil {
		// TODO: What is the best logic here?
	}
	f.currentTx, f.closeCurrentTx = context.WithCancel(ctx)
	// Call Transact on actual Transactor.
	return f.f(ctx, nonce, reqIds, reqVals)
}

func (f FrontRunner) DecodeInputData(txInput []byte) (string, [5]*big.Int, [5]*big.Int, error) {
	// load contract ABI
	abi, err := abi.JSON(strings.NewReader(tellorAbi))
	if err != nil {
		log.Fatal(err)
	}

	// Recover Method from signature and ABI.
	method, err := abi.MethodById(txInput[:4])
	if err != nil {
		log.Fatal(err)
	}

	// Unpack method inputs.
	inputs, err := method.Inputs.Unpack(txInput[4:])
	if err != nil {
		return "", [5]*big.Int{}, [5]*big.Int{}, fmt.Errorf("upacking method inputs: %v", err)
	}
	return inputs[0].(string), inputs[1].([5]*big.Int), inputs[2].([5]*big.Int), nil
}

func (f FrontRunner) WatchForTxPool(contractAddress common.Address, methodName string) {
	txpool, err := txpool.NewBlocknativeTxPool()
	if err != nil {
		panic(err)
	}
	sub, sink, err := txpool.WatchTxPool(contractAddress, methodName)
	if err != nil {
		panic(err)
	}
	for {
		select {
		case err := <-sub.Err():
			panic(err)
		case msg := <-sink:
			// Decode input here.
			data, err := msg.TxInputData()
			if err != nil {
				fmt.Printf("while getting tx input data: %v", err)
				continue
			}
			nonce, reqIds, reqVals, err := f.DecodeInputData(data)
			if err != nil {
				fmt.Printf("while parsing tx input data: %v", err)
				continue
			}
			f.Transact(context.Background(), nonce, reqIds, reqVals)
		}
	}
}

// NewFrontRunner creates a transactor that tries to front run other opponents tx in the eth txpool.
func NewFrontRunner(t Transactor, contractAddress common.Address, methodName string) Transactor {
	f := &FrontRunner{f: t.Transact}
	go f.WatchForTxPool(contractAddress, methodName)
	return f
}
