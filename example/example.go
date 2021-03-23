package main

import (
	"context"
	"fmt"
	"log"
	"syscall"

	stdlog "log"

	"github.com/cryptoriums/mempmon/pkg/logger"
	"github.com/cryptoriums/mempmon/pkg/txpool"
	"github.com/ethereum/go-ethereum/common"
	"github.com/joho/godotenv"
	"github.com/oklog/run"
	"github.com/pkg/errors"
)

// Watching Tether contract transfers using the Blocknative api.
func main() {
	err := godotenv.Load()
	if err != nil {
		log.Fatal("Error loading .env file")
	}

	var g run.Group
	ctxGlobal, close := context.WithCancel(context.Background())
	logger := logger.NewLogger()

	// Run groups.
	{
		g.Add(run.SignalHandler(context.Background(), syscall.SIGHUP, syscall.SIGINT, syscall.SIGTERM))

		txpool, msgCh, err := txpool.NewBlocknativeTxPool(ctxGlobal, logger)
		ExitOnErr(err, "creating mempool monitor")
		err = txpool.Subscribe(common.HexToAddress("0xdac17f958d2ee523a2206206994597c13d831ec7"), "transfer")
		ExitOnErr(err, "mempool monitor subscription")
		g.Add(func() error {
			return txpool.Run()
		}, func(error) {
			txpool.Stop()
			close()
		})

		go func() {
			for {
				select {
				case <-ctxGlobal.Done():
					return
				case msg := <-msgCh:
					fmt.Printf("msg: %v \n", msg)
				}
			}
		}()

		if err := g.Run(); err != nil {
			stdlog.Println(fmt.Sprintf("%+v", errors.Wrapf(err, "run group stacktrace")))
		}

	}
}

func ExitOnErr(err error, msg string) {
	if err != nil {
		stdlog.Fatalf("root execution error:%+v msg:%+v", err, msg)
	}
}
