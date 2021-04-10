package main

import (
	"context"
	"fmt"
	"os"
	"syscall"
	"time"

	stdlog "log"

	"github.com/cryptoriums/mempmon/pkg/blocknative"
	"github.com/ethereum/go-ethereum/common"
	"github.com/go-kit/kit/log"
	"github.com/joho/godotenv"
	"github.com/oklog/run"
	"github.com/pkg/errors"
)

// Watching Tether contract transfers using the Blocknative api.
func main() {
	logger := log.With(log.NewLogfmtLogger(log.NewSyncWriter(os.Stderr)), "ts", log.TimestampFormat(func() time.Time { return time.Now().UTC() }, "Jan 02 15:04:05.99"), "caller", log.DefaultCaller)

	ExitOnErr(godotenv.Load(), "loading .env file")

	var g run.Group

	// Run groups.
	{
		g.Add(run.SignalHandler(context.Background(), syscall.SIGHUP, syscall.SIGINT, syscall.SIGTERM))

		mempool, err := blocknative.New(logger, os.Getenv("BLOCKNATIVE_WS_URL"), os.Getenv("BLOCKNATIVE_DAPP_ID"), 4)
		ExitOnErr(err, "creating mempool monitor")
		err = mempool.Subscribe(context.Background(), common.HexToAddress("0x88dF592F8eb5D7Bd38bFeF7dEb0fBc02cf3778a0"), "submitMiningSolution")
		ExitOnErr(err, "mempool monitor subscription")
		g.Add(func() error {
			for {
				msg, err := mempool.Read()
				ExitOnErr(err, "mempool read")
				fmt.Printf("msg: %v \n", msg)
			}
		}, func(error) {
			mempool.Close()
		})

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
