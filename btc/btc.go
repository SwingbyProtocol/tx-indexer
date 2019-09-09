package btc

import (
	"net/http"
	"sync"
	"time"

	"github.com/SwingbyProtocol/sc-indexer/resolver"
	"github.com/ant0ine/go-json-rest/rest"
	log "github.com/sirupsen/logrus"
)

const (
	chainInfo = "/rest/chaininfo.json"
)

var lock = sync.RWMutex{}

type BTCNode struct {
	BestBlockHash string
	Chain         string
	Blocks        int64
	Headers       int64
	Resolver      *resolver.Resolver
	URI           string
	Errors        int64
}

type ChainInfo struct {
	Chain         string `json:"chain"`
	Blocks        int64  `json:"blocks"`
	Headers       int64  `json:"headers"`
	BestBlockHash string `json:"bestblockhash"`
}

type Block struct {
	Hash          string `json:"hash"`
	Confirmations int64  `json:"confirmations"`
	Height        int64  `json:"height"`
	NTx           int64  `json:"nTx"`
	Txs           []*Tx  `json:"tx"`
}

func NewBTCNode(uri string) *BTCNode {
	node := &BTCNode{
		URI:      uri,
		Resolver: resolver.NewResolver(),
	}
	return node
}

func (b *BTCNode) Start() {
	ticker := time.NewTicker(13 * time.Second)
	for {
		select {
		case <-ticker.C:
			go b.GetBestBlockHash()
		}
	}
}

func (b *BTCNode) GetBestBlockHash() error {
	res := ChainInfo{}
	err := b.Resolver.GetRequest(b.URI, chainInfo, &res)
	if err != nil {
		b.Errors++
		return err
	}
	b.BestBlockHash = res.BestBlockHash
	b.Blocks = res.Blocks
	b.Chain = res.Chain
	log.Info(b)
	go b.GetBlock(b.BestBlockHash)
	return nil
}

func (b *BTCNode) GetBlock(bestblockhash string) error {
	res := Block{}
	err := b.Resolver.GetRequest(b.URI, "/rest/block/"+bestblockhash+".json", &res)
	if err != nil {
		b.Errors++
		return err
	}
	for _, tx := range res.Txs {
		for _, vout := range tx.Vout {
			for _, addr := range vout.ScriptPubkey.Addresses {
				log.Info(vout.Value, " ", *addr)
			}
		}
	}
	return nil
}

func (b *BTCNode) GetBTCTxs(w rest.ResponseWriter, r *rest.Request) {
	code := r.PathParam("code")
	b.GetBestBlockHash()
	log.Info(code)
	lock.Lock()
	lock.Unlock()
	w.WriteHeader(http.StatusOK)
	//w.WriteJson(&b.Txs)
}
