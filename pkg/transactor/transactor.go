package transactor

import (
	"context"
	"math/big"
	"os"
	"time"

	"github.com/cryptoriums/mempmon/pkg/config"
	"github.com/cryptoriums/mempmon/pkg/mempool/blocknative"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/log/level"
	"github.com/joho/godotenv"
	"github.com/pkg/errors"
	telliotCfg "github.com/tellor-io/telliot/pkg/config"
	"github.com/tellor-io/telliot/pkg/contracts"
	"github.com/tellor-io/telliot/pkg/db"
	"github.com/tellor-io/telliot/pkg/profitChecker"
)

const gasUsge = 170000
const profitThreshold = 50

type txMined struct {
	Err     error
	Receipt *types.Receipt
	Tx      *types.Transaction
}

// FrontRunner implements Transactor interface.
type FrontRunner struct {
	logger           log.Logger
	mempMon          *blocknative.Mempool
	client           contracts.ETHClient
	contractInstance *contracts.ITellor
	proxy            db.DataServerProxy
	account          *telliotCfg.Account
	whiteListedAccs  map[string]bool
	profitChecker    *profitChecker.ProfitChecker
	txMined          chan *txMined
	txPending        *types.Transaction
	txPendingCncl    context.CancelFunc
}

// New creates a transactor that tries to front run other opponents tx in the eth mempool.
func New(
	ctx context.Context,
	logger log.Logger,
	client contracts.ETHClient,
	contractInstance *contracts.ITellor,
	proxy db.DataServerProxy,
	account *telliotCfg.Account,
	whiteListedAccs map[string]bool,
) (*FrontRunner, error) {
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
		logger:           logger,
		mempMon:          mempMon,
		client:           client,
		contractInstance: contractInstance,
		proxy:            proxy,
		account:          account,
		profitChecker:    profitChecker.NewProfitChecker(logger, client, contractInstance, proxy),
	}, nil
}

func (self *FrontRunner) Transact(ctx context.Context, solution string, reqIds [5]*big.Int, reqVals [5]*big.Int) (*types.Transaction, *types.Receipt, error) {
	ticker := time.NewTicker(1 * time.Second)
	for {
		select {
		case <-ctx.Done():
			return nil, nil, errors.New("context canceled")
		case txMined := <-self.txMined:
			// When the err is context cancelled, need to ignore and continue listening.
			return txMined.Tx, txMined.Receipt, txMined.Err
		case txMemp := <-self.waitMempool(ctx):
			// Don't front run white listed account.
			if _, ok := self.whiteListedAccs[txMemp.Event.Transaction.From]; ok {
				continue
			}

			gasPrice := big.NewInt(int64(txMemp.Event.Transaction.GasPriceGwei + 100))
			nonce, err := self.client.NonceAt(ctx, self.account.Address)
			if err != nil {
				return nil, nil, errors.Wrap(err, "getting nonce for miner address")
			}

			if self.txPending != nil {
				self.txPendingCncl()
				lasGasPrice := self.txPending.GasPrice()
				nonce = self.txPending.Nonce()
				gasPrice = lasGasPrice.Add(lasGasPrice, lasGasPrice.Div(lasGasPrice, big.NewInt(10))) // Add 10% more gas then the pending tx.
			}

			// Transact only when profit is 0 or more.
			for {
				profitPercent, err := self.profitChecker.Current(big.NewInt(5), gasPrice)
				if err != nil {
					level.Error(self.logger).Log("msg", "getting profit percent", "err", err)
					<-ticker.C
					continue
				}
				if profitPercent < 0 {
					level.Debug(self.logger).Log("msg", "mempool front running profit check below zero", "profitPercent", profitPercent)
					<-ticker.C
					continue
				}
				break
			}

			ctxTx, ctxTxCncl := context.WithCancel(context.Background())
			tx, err := self.transact(ctxTx, gasPrice, nonce, solution, reqIds, reqVals)
			if err != nil {
				ctxTxCncl()
				return nil, nil, err
			}
			self.txPendingCncl = ctxTxCncl
			self.txPending = tx

		// No other tx has been submitted, but profit is too high to miss so submit anyway.
		case gasPrice := <-self.waitProfitThreshold(ctx):
			if self.txPending != nil {
				level.Info(self.logger).Log("msg", "profit threshold reached, but there is already a pending transaction")
				continue
			}

			nonce, err := self.client.NonceAt(ctx, self.account.Address)
			if err != nil {
				return nil, nil, errors.Wrap(err, "getting nonce for miner address")
			}

			ctxTx, ctxTxCncl := context.WithCancel(context.Background())
			tx, err := self.transact(ctxTx, gasPrice, nonce, solution, reqIds, reqVals)
			if err != nil {
				ctxTxCncl()
				return nil, nil, err
			}
			self.txPendingCncl = ctxTxCncl
			self.txPending = tx

			// Continue monitoring the mempool and if someone else tries to front run us.
			continue

		}
	}
	// Later will also add logic to cancel a tx when it will cause a loss or when the other wallet cancels his tx.
	// I have noticed that the other wallet sometimes submits another transaction to cancel it.
}

func (self *FrontRunner) waitProfitThreshold(ctx context.Context) chan *big.Int {
	ch := make(chan *big.Int)
	go func(ctx context.Context) {
		ticker := time.NewTicker(1 * time.Second)
		for {
			select {
			case <-ctx.Done():
				return
			default:
			}
			_gasPrice, err := self.proxy.Get(db.GasKey)
			if err != nil {
				level.Error(self.logger).Log("msg", "getting gas price", "err", err)
				<-ticker.C
			}
			gasPrice, err := hexutil.DecodeBig(string(_gasPrice))
			if err != nil {
				level.Error(self.logger).Log("msg", "decode gas price", "err", err)
				<-ticker.C
				continue
			}

			profitPercent, err := self.profitChecker.Current(big.NewInt(5), gasPrice)
			if err != nil {
				level.Error(self.logger).Log("msg", "getting profit percent", "err", err)
				<-ticker.C
				continue
			}
			if profitPercent < profitThreshold {
				<-ticker.C
				continue
			}
			select {
			case <-ctx.Done():
				return
			case ch <- gasPrice:
				return
			}
		}
	}(ctx)
	return ch
}

func (self *FrontRunner) transact(
	ctx context.Context,
	gasPrice *big.Int,
	nonce uint64,
	solution string,
	reqIds [5]*big.Int,
	reqVals [5]*big.Int,
) (*types.Transaction, error) {
	balance, err := self.client.BalanceAt(ctx, self.account.Address, nil)
	if err != nil {
		return nil, errors.Wrap(err, "get account balance")
	}

	gasUsage := big.NewInt(gasUsge)
	cost := big.NewInt(0).Mul(gasUsage, gasPrice)
	if balance.Cmp(cost) < 0 {
		return nil, errors.Errorf("insufficient funds to send transaction: %v < %v", balance, cost)
	}

	netID, err := self.client.NetworkID(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "getting network id")
	}
	auth, err := bind.NewKeyedTransactorWithChainID(self.account.PrivateKey, netID)
	if err != nil {
		return nil, errors.Wrap(err, "creating transactor")
	}

	auth.Nonce = big.NewInt(int64(nonce))
	auth.Value = big.NewInt(0)      // in weiF
	auth.GasLimit = uint64(3000000) // in units
	auth.GasPrice = gasPrice

	return self.contractInstance.SubmitMiningSolution(
		auth,
		solution,
		reqIds,
		reqVals,
	)
}

func (self *FrontRunner) waitMined(ctx context.Context, tx *types.Transaction) {
	txMined := &txMined{}
	receipt, err := bind.WaitMined(ctx, self.client, tx)
	if err != nil {
		txMined.Err = err
		self.txMined <- txMined
	}
	if receipt.Status != 1 {
		txMined.Err = errors.Errorf("unsuccessful transaction status:%v", receipt.Status)
		self.txMined <- txMined
	}
	txMined.Tx = tx
	txMined.Receipt = receipt
	self.txMined <- txMined
}

func (self *FrontRunner) waitMempool(ctx context.Context) chan *blocknative.Message {
	ch := make(chan *blocknative.Message)
	go func(mempMon *blocknative.Mempool, ctx context.Context) {
		ticker := time.NewTicker(1 * time.Second)
		for {
			select {
			case <-ctx.Done():
				return
			default:
			}
			tx, err := mempMon.Read()
			if err != nil {
				level.Error(self.logger).Log("msg", "read mempool tx", "err", err)
				err = self.mempMon.Subscribe(ctx, common.HexToAddress(telliotCfg.TellorAddress), "submitMiningSolution")
				if err != nil {
					level.Error(self.logger).Log("msg", "mempool subscribe", "err", err)
					<-ticker.C
					continue
				}
			}
			select {
			case <-ctx.Done():
				return
			case ch <- tx:
				return
			}
		}
	}(self.mempMon, ctx)
	return ch
}

func (self *FrontRunner) Close() {
	self.mempMon.Close()
}
