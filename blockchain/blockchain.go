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

type PushMsg struct {
	Tx    *Tx
	Addr  string
	State int
}

type BlockchainConfig struct {
	// TrustedNode is ip addr for connect to rest api
	TrustedNode string
	// PruneSize is holding height
	PruneSize uint
}

type Blockchain struct {
	mu              *sync.RWMutex
	resolver        *Resolver
	index           map[int64]*Index // index is stored per block
	minedtime       map[int64]int64  // store the minedtime per block
	txStore         *TxStore
	targetPrune     uint
	targetHeight    int64
	latestBlockHash string
	txChan          chan *wire.MsgTx
	blockChan       chan *wire.MsgBlock
	pushMsgChan     chan *PushMsg
}

func NewBlockchain(conf *BlockchainConfig) *Blockchain {
	bc := &Blockchain{
		mu:          new(sync.RWMutex),
		resolver:    NewResolver(conf.TrustedNode),
		index:       make(map[int64]*Index),
		minedtime:   make(map[int64]int64),
		txStore:     NewTxStore(),
		targetPrune: conf.PruneSize,
		txChan:      make(chan *wire.MsgTx),
		blockChan:   make(chan *wire.MsgBlock),
		pushMsgChan: make(chan *PushMsg),
	}
	log.Info("Using trusted node ", conf.TrustedNode)
	block, err := bc.GetRemoteBlock()
	if err != nil {
		log.Fatal(err)
	}
	//bc.index[block.Height] = NewIndex()
	bc.FinalizeBlock(block)
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
				//log.Info(err)
				continue
			}
			log.Infof("Get block -> #%d %s", block.Height, bc.latestBlockHash)
			// Add/Update txs from block
			bc.UpdateBlockTxs(block)
			// Finalize block
			bc.FinalizeBlock(block)
			// Delte old index
			bc.DeleteOldIndex()
		}
	}()
}

func (bc *Blockchain) GetIndexTxs(addr string, depth int, state int) ([]*Tx, error) {
	res, err := bc.GetIndexTxsRange(addr, bc.targetHeight, depth, state, false)
	if err != nil {
		return nil, err
	}
	return res, nil
}

func (bc *Blockchain) GetIndexTxsWithTW(addr string, start int64, end int64, state int, mempool bool) ([]*Tx, error) {
	heights := []int64{}
	res := []*Tx{}
	if end == 0 {
		end = int64(^uint(0) >> 1)
	}
	for height, time := range bc.minedtime {
		if time >= start && time <= end {
			heights = append(heights, height)
		}
	}
	if mempool {
		heights = append(heights, bc.targetHeight)
	}
	isError := false
	for _, height := range heights {
		txs, err := bc.GetIndexTxsBlock(addr, height, state, mempool)
		if err != nil {
			isError = true
			continue
		}
		for _, tx := range txs {
			res = append(res, tx)
		}
	}
	if isError {
		return nil, errors.New("block tx get error")
	}
	log.Info(heights)
	sortTx(res)
	return res, nil
}

func (bc *Blockchain) GetIndexTxsBlock(addr string, height int64, state int, mempool bool) ([]*Tx, error) {
	res := []*Tx{}
	if bc.index[height] == nil {
		return nil, errors.New("error ")
	}
	txids := bc.index[height].GetTxIDs(addr, state)
	minedtime := bc.minedtime[height]
	if mempool {
		minedtime = 0
	}
	// Getting txs
	txs, err := bc.txStore.GetTxs(txids, minedtime)
	if err != nil {
		return nil, err
	}
	for _, tx := range txs {
		res = append(res, tx)
	}
	return res, nil
}

func (bc *Blockchain) GetIndexTxsRange(addr string, end int64, depth int, state int, mined bool) ([]*Tx, error) {
	if depth == 0 {
		// Set max depth if depth is zero
		depth = len(bc.index)
	}
	res := []*Tx{}
	// Start with hightest block
	for i := end; i > end-int64(depth); i-- {
		if bc.index[i] == nil {
			continue
		}
		txids := bc.index[i].GetTxIDs(addr, state)
		// Getting txs with sorted
		txs, _ := bc.txStore.GetTxs(txids, bc.minedtime[i])
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
			//log.Info(tx.Txid, " ", addrs[0])
			// Update index and the spent tx (spent)
			bc.index[bc.targetHeight].Update(addrs[0], tx.Txid, Send)
			// Publish tx to notification handler
			bc.pushMsgChan <- &PushMsg{Tx: inTx, Addr: addrs[0], State: Send}
		}
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
		bc.index[bc.targetHeight].Update(addr, tx.Txid, Received)
		// Publish tx to notification handler
		bc.pushMsgChan <- &PushMsg{Tx: tx, Addr: addr, State: Received}
	}
}

func (bc *Blockchain) DeleteOldIndex() {
	if len(bc.index) > int(bc.targetPrune) {
		// Remove index if prune block is come
		bc.mu.Lock()
		delete(bc.index, bc.targetHeight-int64(bc.targetPrune))
		delete(bc.minedtime, bc.targetHeight-int64(bc.targetPrune))
		bc.mu.Unlock()
		log.Info("delete index and minedtime ", bc.targetHeight-int64(bc.targetPrune))
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
			//log.Info("new tx")
			continue
		}
		storedtx.AddBlockData(block.Height, block.Time, block.Mediantime)
		bc.txStore.UpdateTx(storedtx)
	}
}

func (bc *Blockchain) FinalizeBlock(block *Block) {
	// Check the old block
	if bc.index[bc.targetHeight] != nil {
		// Update prev block's mined time
		bc.minedtime[bc.targetHeight] = block.Time
	}
	newHeight := bc.targetHeight + 1
	bc.index[newHeight] = NewIndex()
	// Update curernt block height
	bc.targetHeight = newHeight
	log.Info("now -> ", bc.targetHeight, " ", bc.index, bc.minedtime)
}

func (bc *Blockchain) GetRemoteBlock() (*Block, error) {
	bc.mu.Lock()
	defer bc.mu.Unlock()

	info := ChainInfo{}
	err := bc.resolver.GetRequest("/rest/chaininfo.json", &info)
	if err != nil {
		return nil, err
	}
	if bc.targetHeight == info.Blocks+1 {
		return nil, errors.New("target height is latest")
	}

	if bc.latestBlockHash == "" {
		// Initialize targetHeight
		bc.targetHeight = info.Blocks
	}
	diff := info.Blocks - bc.targetHeight
	if diff < 0 {
		diff = 0
	}
	depthBlock, err := bc.GetDepthBlock(info.Bestblockhash, int(diff))
	if err != nil {
		return nil, err
	}
	if bc.latestBlockHash != "" && depthBlock.Previousblockhash != bc.latestBlockHash {
		return nil, errors.New("Previousblockhash is not correct")
	}
	bc.latestBlockHash = depthBlock.Hash
	return depthBlock, nil
}

func (bc *Blockchain) GetDepthBlock(blockHash string, depth int) (*Block, error) {
	block := Block{}
	err := bc.resolver.GetRequest("/rest/block/"+blockHash+".json", &block)
	if err != nil {
		return nil, err
	}
	if block.Height == 0 {
		return nil, errors.New("block height is zero")
	}
	if depth != 0 {
		depth = depth - 1
	}
	if depth == 0 {
		return &block, nil
	}
	return bc.GetDepthBlock(block.Previousblockhash, depth)
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

func (bc *Blockchain) PushMsgChan() chan *PushMsg {
	return bc.pushMsgChan
}
