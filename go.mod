module github.com/cryptoriums/mempmon

go 1.16

require (
	github.com/ethereum/go-ethereum v1.10.1
	github.com/go-kit/kit v0.10.0
	github.com/gorilla/websocket v1.4.2
	github.com/joho/godotenv v1.3.0
	github.com/oklog/run v1.1.0
	github.com/pkg/errors v0.9.1
	github.com/tellor-io/telliot v0.0.7-0.20210321204241-0375f992f898
)

replace github.com/tellor-io/telliot => ../../tellor-io/telliot
