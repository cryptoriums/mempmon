package main

import (
	"fmt"
	"log"

	"github.com/cryptoriums/mempmon/pkg/txpool"
	"github.com/ethereum/go-ethereum/common"
	"github.com/joho/godotenv"
)

// Watching Tether contract transfers using the Blocknative api.
func main() {
	err := godotenv.Load()
	if err != nil {
		log.Fatal("Error loading .env file")
	}
	txpool, err := txpool.NewBlocknativeTxPool()
	if err != nil {
		panic(err)
	}
	sub, sink, err := txpool.WatchTxPool(common.HexToAddress("0xdac17f958d2ee523a2206206994597c13d831ec7"), "transfer")
	if err != nil {
		panic(err)
	}
	for {
		select {
		case err := <-sub.Err():
			panic(err)
		case msg := <-sink:
			fmt.Printf("msg: %v", msg)
		}
	}
}
