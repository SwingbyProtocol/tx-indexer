package bnb

import (
	"net/url"
	"sync"
	"time"

	"github.com/SwingbyProtocol/tx-indexer/common"
	"github.com/SwingbyProtocol/tx-indexer/utils"
	"github.com/ant0ine/go-json-rest/rest"
	"github.com/binance-chain/go-sdk/common/types"
	eth_common "github.com/ethereum/go-ethereum/common"
	log "github.com/sirupsen/logrus"
)

type Keeper struct {
	mu        *sync.RWMutex
	client    *Client
	ticker    *time.Ticker
	tokenAddr eth_common.Address
	watchAddr types.AccAddress
	tesnet    bool
	Txs       *State
}

type State struct {
	InTxsMempool  []common.Transaction `json:"inTxsMempool"`
	InTxs         []common.Transaction `json:"inTxs"`
	OutTxsMempool []common.Transaction `json:"outTxsMempool"`
	OutTxs        []common.Transaction `json:"outTxs"`
}

func NewKeeper(urlStr string, apiUri string, isTestnet bool) *Keeper {
	bnbRPCURI, err := url.ParseRequestURI(urlStr)
	if err != nil {
		panic(err)
	}
	bnbHTTPURI, err := url.ParseRequestURI(apiUri)
	if err != nil {
		panic(err)
	}
	bnbNetwork := types.ProdNetwork
	if isTestnet {
		bnbNetwork = types.TestNetwork
	}
	c := NewClient(bnbRPCURI, bnbHTTPURI, bnbNetwork)
	k := &Keeper{
		mu:     new(sync.RWMutex),
		client: c,
		tesnet: isTestnet,
		Txs:    &State{},
	}
	return k
}

func (k *Keeper) StartReScanAddr(addr string, timestamp int64) error {
	// TODO: set rescan
	return nil
}

func (k *Keeper) SetTokenAndAddr(token string, addr string) error {
	k.mu.Lock()
	//	k.tokenAddr = eth_common.HexToAddress(token)
	//	k.watchAddr = eth_common.HexToAddress(addr)
	types.Network = types.TestNetwork
	accAddr, err := types.AccAddressFromBech32(addr)
	if err != nil {
		log.Info(err)
	}
	k.watchAddr = accAddr
	k.mu.Unlock()
	return nil
}

func (k *Keeper) GetAddr() types.AccAddress {
	k.mu.RLock()
	defer k.mu.RUnlock()
	return k.watchAddr
}

func (k *Keeper) GetTxs(w rest.ResponseWriter, r *rest.Request) {
	k.mu.RLock()
	defer k.mu.RUnlock()
	w.WriteJson(k.Txs)
}

func (k *Keeper) Start() {
	if k.GetAddr().String() == new(eth_common.Address).String() {
		log.Fatal("Error: eth address is not set")
	}
	k.ticker = time.NewTicker(15 * time.Second)
	k.processKeep()
	go func() {
		for {
			select {
			case <-k.ticker.C:
				k.processKeep()
			}
		}
	}()
}

func (k *Keeper) BroadcastTx(w rest.ResponseWriter, r *rest.Request) {
	/*
		req := common.BroadcastParams{}
		res := common.BroadcastResponse{
			Result: false,
		}
		err := r.DecodeJsonPayload(&req)
		if err != nil {
			res.Msg = err.Error()
			w.WriteHeader(400)
			w.WriteJson(res)
			return
		}
		var tx *types.Transaction
		rawtx, err := hexutil.Decode(req.HEX)
		rlp.DecodeBytes(rawtx, &tx)
		err = k.client.RpcClient().SendToken(context.Background(), tx)
		if err != nil {
			res.Msg = err.Error()
			w.WriteHeader(400)
			w.WriteJson(res)
			return
		}
		res.TxHash = tx.Hash().String()
		res.Result = true
		res.Msg = "success"
		w.WriteJson(res)
	*/
}

func (k *Keeper) processKeep() error {
	fromTime := time.Now().Add(-78 * time.Hour)
	toTime := time.Now()
	bnbBal, err := k.client.RpcClient().GetBalance(k.watchAddr, "BNB")
	if err != nil {
		log.Errorf("Error while calling bnbClient.RpcClient().GetBalance(bnbAccAddr, BNB): %s", err)
		bnbBal = &types.TokenBalance{
			Symbol: "BNB",
		}
	}
	bnbSwapCoinBal, err := k.client.RpcClient().GetBalance(k.watchAddr, common.BNB.String())
	if err != nil {
		log.Errorf("Error while calling bnbClient.RpcClient().GetBalance(bnbAccAddr, side1Params.Currency): %s", err)
		bnbSwapCoinBal = &types.TokenBalance{
			Symbol: common.BNB.String(),
		}
	}
	log.Infof("BNB: Native token balance: %d", bnbBal.Free)
	log.Infof("BNB: Spendable token balance: %d", bnbSwapCoinBal.Free)

	//bnbSwapCoinEffBal := bnbSwapCoinBal.Free.ToInt64()
	//if bnbBal.Free.ToInt64() < side1Params.Fees.FixedOutFee {
	// TODO: a bit of a hacky way to do it
	//	bnbSwapCoinEffBal = 0 // no BNB; refund incoming swaps on the other chain
	//}
	// .. incoming BNB txs
	bnbInTxs, err := k.client.GetTransactions(common.TxQueryParams{
		Address:  k.watchAddr.String(),
		Type:     TxTypeReceived,
		TimeFrom: fromTime.Unix(),
		TimeTo:   toTime.Unix(), // snap to epoch interval
	})
	if err != nil {
		return err
	}
	// ... outgoing BNB txs
	bnbOutTxs, err := k.client.GetTransactions(common.TxQueryParams{
		Address:  k.watchAddr.String(),
		Type:     TxTypeSend,
		TimeFrom: fromTime.Unix() - (60 * 60),
		TimeTo:   0, // 0 = now. always find an outbound TX even when extremely recent (anti dupe tx)
	})
	if err != nil {
		return err
	}

	//utils.SortTx(inTxsMempool)
	utils.SortTx(bnbInTxs)
	//utils.SortTx(outTxsMempool)
	utils.SortTx(bnbOutTxs)

	k.mu.Lock()
	//k.Txs.InTxsMempool = bnbInTxs
	k.Txs.InTxs = bnbInTxs
	//	k.Txs.OutTxsMempool = bnbOutTxs
	k.Txs.OutTxs = bnbOutTxs
	k.mu.Unlock()
	return nil
}

func (k *Keeper) Stop() {
	k.ticker.Stop()
}
