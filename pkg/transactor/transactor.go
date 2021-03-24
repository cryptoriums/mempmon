package transactor

import (
	"context"
	"errors"
	"fmt"
	"math/big"
	"os"
	"time"

	"github.com/cryptoriums/mempmon/pkg/config"
	"github.com/cryptoriums/mempmon/pkg/mempool/blocknative"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/log/level"
	"github.com/joho/godotenv"
	telliotCfg "github.com/tellor-io/telliot/pkg/config"
)

// FrontRunner implements Transactor interface.
type FrontRunner struct {
	logger  log.Logger
	mempMon *blocknative.Mempool
}

// New creates a transactor that tries to front run other opponents tx in the eth mempool.
func New(ctx context.Context, logger log.Logger) (*FrontRunner, error) {
	err := godotenv.Load()
	if err != nil {
		if err != nil {
			return nil, err
		}
	}
	mempMon, err := blocknative.New(logger, os.Getenv(config.BlocknativeWSURL), os.Getenv(config.BlocknativeDappID))
	if err != nil {
		return nil, err
	}
	err = mempMon.Subscribe(ctx, common.HexToAddress(telliotCfg.TellorAddress), "submitMiningSolution")
	if err != nil {
		return nil, err
	}
	return &FrontRunner{
		logger:  logger,
		mempMon: mempMon,
	}, nil
}

func (self *FrontRunner) Transact(ctx context.Context, nonce string, reqIds [5]*big.Int, reqVals [5]*big.Int) (*types.Transaction, *types.Receipt, error) {
	ticker := time.NewTicker(1 * time.Second)

	for {
		select {
		case <-ctx.Done():
			return nil, nil, errors.New("context canceled")
		default:
			tx, err := self.mempMon.Read()
			if err != nil {
				level.Error(self.logger).Log("msg", "read mempool tx", "err", err)
				err = self.mempMon.Subscribe(ctx, common.HexToAddress(telliotCfg.TellorAddress), "submitMiningSolution")
				if err != nil {
					level.Error(self.logger).Log("msg", "mempool subscribe", "err", err)
					<-ticker.C
					continue
				}
			}

			fmt.Printf("tx: %v \n", tx)

		}
	}
	// Use a ticker to check the price. When no tx arrives to front run submit when the profit is 50%
	// When a tx arrives always front run when it will not cause a loss.
	//  Later will also add logic to cancel a tx when it will cause a loss or when the other wallet cancels his tx. I have noticed that he sometimes submits another transaction to cancel it.
	return nil, nil, nil
}

func (self FrontRunner) Close() {
	self.mempMon.Close()
}

// // Run groups.
// {
// 	g.Add(run.SignalHandler(context.Background(), syscall.SIGHUP, syscall.SIGINT, syscall.SIGTERM))

// 	g.Add(func() error {
// 		return txpool.Run()
// 	}, func(error) {
// 		txpool.Stop()
// 		close()
// 	})

// 	go func() {
// 		for {
// 			select {
// 			case <-ctxGlobal.Done():
// 				return
// 			case msg := <-msgCh:
// 				fmt.Printf("msg: %v \n", msg)
// 			}
// 		}
// 	}()

// 	if err := g.Run(); err != nil {
// 		stdlog.Println(fmt.Sprintf("%+v", errors.Wrapf(err, "run group stacktrace")))
// 	}

// }

// func (self FrontRunner) DecodeInputData(txInput []byte) (string, [5]*big.Int, [5]*big.Int, error) {
// 	// load contract ABI
// 	abi, err := abi.JSON(strings.NewReader(tellorAbi))
// 	if err != nil {
// 		return "",nil,nil,err
// 	}

// 	// Recover Method from signature and ABI.
// 	method, err := abi.MethodById(txInput[:4])
// 	if err != nil {
// 		log.Fatal(err)
// 	}

// 	// Unpack method inputs.
// 	inputs, err := method.Inputs.Unpack(txInput[4:])
// 	if err != nil {
// 		return "", [5]*big.Int{}, [5]*big.Int{}, fmt.Errorf("upacking method inputs: %v", err)
// 	}
// 	return inputs[0].(string), inputs[1].([5]*big.Int), inputs[2].([5]*big.Int), nil
// }

// func (self FrontRunner) WatchForTxPool(contractAddress common.Address, methodName string) {
// 	txpool,msgCh, err := txpool.NewBlocknativeTxPool()
// 	if err != nil {
// 		panic(err)
// 	}
// 	sub, sink, err := txpool.WatchTxPool(contractAddress, methodName)
// 	if err != nil {
// 		panic(err)
// 	}
// 	for {
// 		select {
// 		case err := <-sub.Err():
// 			panic(err)
// 		case msg := <-sink:
// 			// Decode input here.
// 			data, err := msg.TxInputData()
// 			if err != nil {
// 				fmt.Printf("while getting tx input data: %v", err)
// 				continue
// 			}
// 			nonce, reqIds, reqVals, err := f.DecodeInputData(data)
// 			if err != nil {
// 				fmt.Printf("while parsing tx input data: %v", err)
// 				continue
// 			}
// 			f.Transact(context.Background(), nonce, reqIds, reqVals)
// 		}
// 	}
// }
