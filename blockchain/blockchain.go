package blockchain

import (
	"errors"
	"sync"

	"github.com/btcsuite/btcd/wire"
	log "github.com/sirupsen/logrus"
)

type Blockchain struct {
	mu       *sync.RWMutex
	resolver *Resolver
	// index is stored per block
	index       map[int64]*Index
	txStore     *TxStore
	targetPrune int
	nowHeight   int64
	txChan      chan *wire.MsgTx
	blockChan   chan *wire.MsgBlock
	pushTxChan  chan *Tx
}

func NewBlockchain(conf *BlockchainConfig) *Blockchain {
	bc := &Blockchain{
		mu:          new(sync.RWMutex),
		resolver:    NewResolver(conf.TrustedNode),
		index:       make(map[int64]*Index),
		txStore:     NewTxStore(),
		targetPrune: conf.PruneSize,
		txChan:      make(chan *wire.MsgTx),
		blockChan:   make(chan *wire.MsgBlock),
		pushTxChan:  make(chan *Tx),
	}
	log.Info("Using trusted node ", conf.TrustedNode)
	block, err := bc.GetRemoteBlock()
	if err != nil {
		log.Fatal("latest block is not get")
	}
	bc.AddBlock(block)
	return bc
}

func (b *Blockchain) Start() {
	go func() {
		for {
			msg := <-b.txChan
			tx := MsgTxToTx(msg)
			// Check tx input to update indexer storage
			for _, in := range tx.Vin {
				// Load a tx from storage
				inTx, err := b.txStore.GetTx(in.Txid)
				if err != nil {
					// continue if spent tx is not exist
					//log.Debug(err)
					continue
				}
				targetOutput := inTx.Vout[in.Vout]
				targetOutput.Spent = true
				targetOutput.Txs = append(targetOutput.Txs, tx.Txid)
				// check the sender of tx
				addrs := inTx.GetOutsAddrs()
				for _, addr := range addrs {
					// Update index and the spent tx (spent)
					b.index[b.nowHeight].UpdateTx(addr, inTx.Txid, true)
				}
				// Delete tx that all consumed output
				b.txStore.DeleteAllSpentTx(inTx)

			}
			// Check tx output to update indexer storage
			addrs := tx.GetOutsAddrs()
			for _, addr := range addrs {
				// Update index and the spent tx (unspent)
				b.index[b.nowHeight].UpdateTx(addr, tx.Txid, false)
			}

			// add tx
			b.txStore.AddTx(&tx)
			// push notification to ws handler
			b.pushTxChan <- &tx
		}
	}()

	go func() {
		for {
			msg := <-b.blockChan
			// add block
			block := MsgBlockToBlock(msg)
			log.Info(block.Hash)
			b.AddBlock(&block)
			b.DeleteOldIndex()
		}
	}()
}

func (b *Blockchain) GetIndexTxs(addr string, depth int, spent bool) ([]*Tx, error) {
	if len(b.index) < depth {
		return nil, errors.New("Getindextxs error")
	}
	if depth == 0 {
		// Set max depth if depth is zero
		depth = len(b.index)
	}
	res := []*Tx{}
	for i := b.nowHeight; i > b.nowHeight-int64(depth); i-- {
		txids := b.index[i].GetTxIDs(addr, spent)
		txs, _ := b.txStore.GetTxs(txids)
		for _, tx := range txs {
			res = append(res, tx)
		}
		log.Info("count ", len(b.index), " block ", i)
	}
	return res, nil
}

func (b *Blockchain) AddBlock(block *Block) {
	if b.nowHeight == block.Height {
		return
	}
	b.index[block.Height] = NewIndex()
	b.nowHeight = block.Height
}

func (b *Blockchain) DeleteOldIndex() {
	if len(b.index) > b.targetPrune {
		// Remove index if prune block is come
		b.mu.Lock()
		delete(b.index, b.nowHeight-int64(b.targetPrune))
		b.mu.Unlock()
		log.Info("delete index ", b.nowHeight-int64(b.targetPrune))
	}
}

func (b *Blockchain) GetRemoteBlock() (*Block, error) {
	info := ChainInfo{}
	err := b.resolver.GetRequest("/rest/chaininfo.json", &info)
	if err != nil {
		return nil, err
	}
	block := Block{}
	err = b.resolver.GetRequest("/rest/block/"+info.Bestblockhash+".json", &block)
	if err != nil {
		return nil, err
	}
	return &block, nil
}

func (b *Blockchain) GetTxScore() *TxStore {
	return b.txStore
}

func (b *Blockchain) TxChan() chan *wire.MsgTx {
	return b.txChan
}

func (b *Blockchain) BlockChan() chan *wire.MsgBlock {
	return b.blockChan
}

func (b *Blockchain) PushTxChan() chan *Tx {
	return b.pushTxChan
}
