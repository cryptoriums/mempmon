package main

import (
	"context"
	"fmt"
	"os"
	"syscall"
	"time"

	stdlog "log"

	"github.com/cryptoriums/mempmon/pkg/config"
	"github.com/cryptoriums/mempmon/pkg/mempool/blocknative"
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
	ctxGlobal, close := context.WithCancel(context.Background())

	// Run groups.
	{
		g.Add(run.SignalHandler(context.Background(), syscall.SIGHUP, syscall.SIGINT, syscall.SIGTERM))

		mempool, err := blocknative.New(logger, os.Getenv(config.BlocknativeWSURL), os.Getenv(config.BlocknativeDappID))
		ExitOnErr(err, "creating mempool monitor")
		err = mempool.Subscribe(ctxGlobal, common.HexToAddress("0xdac17f958d2ee523a2206206994597c13d831ec7"), "transfer")
		ExitOnErr(err, "mempool monitor subscription")
		g.Add(func() error {
			for {
				select {
				case <-ctxGlobal.Done():
					mempool.Close()
					return nil
				default:
					msg, err := mempool.Read()
					ExitOnErr(err, "mempool subscription read")
					fmt.Printf("msg: %v \n", msg)
				}
			}
		}, func(error) {
			close()
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
