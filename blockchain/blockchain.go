package blockchain

import (
	"errors"
	"strconv"
	"sync"

	"github.com/btcsuite/btcd/wire"
	log "github.com/sirupsen/logrus"
)

type ChainInfo struct {
	Chain         string `json:"chain"`
	Blocks        int64  `json:"blocks"`
	Headers       int64  `json:"headers"`
	Bestblockhash string `json:"bestblockhash"`
}

type BlockchainConfig struct {
	// TrustedNode is ip addr for connect to rest api
	TrustedNode string
	// PruneSize is holding height
	PruneSize uint
}

type Blockchain struct {
	mu       *sync.RWMutex
	resolver *Resolver
	// index is stored per block
	index       map[int64]*Index
	txStore     *TxStore
	targetPrune uint
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

func (bc *Blockchain) Start() {
	go func() {
		for {
			msg := <-bc.txChan
			tx := MsgTxToTx(msg)
			bc.UpdateIndex(&tx)
			// store the tx
			bc.txStore.AddTx(&tx)
			// push notification to ws handler
			bc.pushTxChan <- &tx
		}
	}()

	go func() {
		for {
			// TODO: Getting block from P2P network
			_ = <-bc.blockChan
			// block := MsgBlockToBlock(msg)
			// Get block data from TrustedPeer for now
			block, err := bc.GetRemoteBlock()
			if err != nil {
				log.Info(err)
				continue
			}
			if bc.nowHeight == block.Height {
				continue
			}
			log.Info("New Block comming")
			// Add/Update txs from block
			bc.UpdateBlockTxs(block)
			// Add block
			bc.AddBlock(block)
			// Delte old index
			bc.DeleteOldIndex()
		}
	}()
}

func (bc *Blockchain) GetIndexTxs(addr string, depth int, spent bool) ([]*Tx, error) {
	if len(bc.index) < depth {
		return nil, errors.New("Getindextxs error")
	}
	if depth == 0 {
		// Set max depth if depth is zero
		depth = len(bc.index)
	}
	res := []*Tx{}
	for i := bc.nowHeight; i > bc.nowHeight-int64(depth); i-- {
		txids := bc.index[i].GetTxIDs(addr, spent)
		txs, _ := bc.txStore.GetTxs(txids)
		for _, tx := range txs {
			res = append(res, tx)
		}
		log.Info("count ", len(bc.index), " block ", i)
	}
	return res, nil
}

func (bc *Blockchain) UpdateIndex(tx *Tx) {
	// Check tx input to update indexer storage
	for _, in := range tx.Vin {
		// Load a tx from storage
		inTx, err := bc.txStore.GetTx(in.Txid)
		if err != nil {
			// continue if spent tx is not exist
			//log.Debug(err)
			continue
		}
		targetOutput := inTx.Vout[in.Vout]
		targetOutput.Spent = true
		targetOutput.Txs = append(targetOutput.Txs, tx.Txid)
		// check the sender of tx
		addrs := targetOutput.Scriptpubkey.Addresses
		if len(addrs) == 1 {
			log.Info(tx.Txid)
			bc.index[bc.nowHeight].UpdateTx(addrs[0], tx.Txid, true)
		}
		// Update index and the spent tx (spent)

		// Delete tx that all consumed output
		bc.txStore.DeleteAllSpentTx(inTx)

	}
	for _, out := range tx.Vout {
		valueStr := strconv.FormatFloat(out.Value.(float64), 'f', -1, 64)
		out.Value = valueStr
	}
	// Check tx output to update indexer storage
	addrs := tx.GetOutsAddrs()
	for _, addr := range addrs {
		// Update index and the spent tx (unspent)
		bc.index[bc.nowHeight].UpdateTx(addr, tx.Txid, false)
	}
}

func (bc *Blockchain) DeleteOldIndex() {
	if len(bc.index) > int(bc.targetPrune) {
		// Remove index if prune block is come
		bc.mu.Lock()
		delete(bc.index, bc.nowHeight-int64(bc.targetPrune))
		bc.mu.Unlock()
		log.Info("delete index ", bc.nowHeight-int64(bc.targetPrune))
	}
}

func (bc *Blockchain) UpdateBlockTxs(block *Block) {
	for _, tx := range block.Txs {
		// Adding tx data from block
		tx.AddBlockData(block.Height, block.Time, block.Mediantime)
		storedtx, err := bc.txStore.GetTx(tx.GetTxID())
		if err != nil {
			tx.Receivedtime = block.Time
			// Update tx
			bc.UpdateIndex(tx)
			// Store new founded Tx
			bc.txStore.AddTx(tx)
			continue
		}
		storedtx.AddBlockData(block.Height, block.Time, block.Mediantime)
		bc.txStore.UpdateTx(storedtx)
	}
}

func (bc *Blockchain) AddBlock(block *Block) {
	bc.index[block.Height] = NewIndex()
	bc.nowHeight = block.Height
}

func (bc *Blockchain) GetRemoteBlock() (*Block, error) {
	info := ChainInfo{}
	err := bc.resolver.GetRequest("/rest/chaininfo.json", &info)
	if err != nil {
		return nil, err
	}
	block := Block{}
	err = bc.resolver.GetRequest("/rest/block/"+info.Bestblockhash+".json", &block)
	if err != nil {
		return nil, err
	}
	if block.Height == 0 {
		return nil, errors.New("block height is zero")
	}
	return &block, nil
}

func (bc *Blockchain) GetTxScore() *TxStore {
	return bc.txStore
}

func (bc *Blockchain) TxChan() chan *wire.MsgTx {
	return bc.txChan
}

func (bc *Blockchain) BlockChan() chan *wire.MsgBlock {
	return bc.blockChan
}

func (bc *Blockchain) PushTxChan() chan *Tx {
	return bc.pushTxChan
}
