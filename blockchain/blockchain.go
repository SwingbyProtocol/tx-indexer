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
	log.Infof("Using trusted node: %s", conf.TrustedNode)
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
			_, err := bc.txStore.GetTx(msg.TxHash().String())
			if err != nil {
				tx := MsgTxToTx(msg)
				bc.UpdateIndex(&tx)
				// store the tx
				bc.txStore.AddTx(&tx)
				log.Debugf("new tx came %s witness %t", tx.Txid, msg.HasWitness())
			}
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
				log.Error(err)
				continue
			}
			log.Infof("Get block -> #%d %s", block.Height, bc.latestBlockHash)
			// Add/Update txs from block
			bc.UpdateBlockTxs(block)
			// Finalize block
			bc.FinalizeBlock(block)
			// Remove old index
			bc.RemoveOldIndex()
			// Remove old minedtime
			bc.RemoveMinedtiime()
		}
	}()
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
	errMsg := ""
	for _, height := range heights {
		txs, err := bc.GetIndexTxsBlock(addr, height, state, mempool)
		if err != nil {
			errMsg = err.Error()
			continue
		}
		for _, tx := range txs {
			res = append(res, tx)
		}
	}
	if errMsg != "" {
		return nil, errors.New(errMsg)
	}
	log.Info(heights)
	sortTx(res)
	return res, nil
}

func (bc *Blockchain) GetIndexTxsBlock(addr string, height int64, state int, mempool bool) ([]*Tx, error) {
	res := []*Tx{}
	index := bc.index[height]
	if index == nil {
		return nil, errors.New("error index is nil")
	}
	txids := index.GetTxIDs(addr, state)
	minedtime := bc.getMinedtime(height)
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

func (bc *Blockchain) getMinedtime(height int64) int64 {
	bc.mu.RLock()
	minedtime := bc.minedtime[height]
	bc.mu.RUnlock()
	return minedtime
}

func (bc *Blockchain) updateMinedtime(height int64, time int64) {
	bc.mu.Lock()
	bc.minedtime[height] = time
	bc.mu.Unlock()
}

func (bc *Blockchain) UpdateIndex(tx *Tx) {
	// Check tx input to update indexer storage
	for _, in := range tx.Vin {
		// Load a tx from storage
		inTx, err := bc.txStore.GetTx(in.Txid)
		if err != nil {
			// continue if spent tx is not exist
			//log.Debug(err)
			in.Value = "not exist"
			in.Addresses = []string{"not exist"}
			continue
		}
		targetOutput := inTx.Vout[in.Vout]
		targetOutput.Spent = true
		targetOutput.Txs = append(targetOutput.Txs, tx.Txid)
		// Add vaue of spent to in
		in.Value = targetOutput.Value
		// check the sender of tx
		addrs := targetOutput.Scriptpubkey.Addresses
		if len(addrs) == 1 {
			// Add address of spent to in
			in.Addresses = addrs
			// Update index and the spent tx (spent)
			bc.index[bc.targetHeight].Update(addrs[0], tx.Txid, Send)
			// Publish tx to notification handler
			bc.pushMsgChan <- &PushMsg{Tx: tx, Addr: addrs[0], State: Send}
		}
		// Remove tx that all consumed output
		//bc.txStore.RemoveAllSpentTx(inTx)
	}
	for _, out := range tx.Vout {
		valueStr := strconv.FormatFloat(out.Value.(float64), 'f', -1, 64)
		out.Value = valueStr
		if len(out.Scriptpubkey.Addresses) != 0 {
			out.Addresses = out.Scriptpubkey.Addresses
		} else {
			out.Addresses = []string{}
		}
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

// RemoveOldIndex invoked when prune block is came
func (bc *Blockchain) RemoveOldIndex() {
	if len(bc.index) <= int(bc.targetPrune)+1 {
		return
	}
	target := bc.targetHeight - int64(bc.targetPrune) - 1
	bc.mu.Lock()
	delete(bc.index, target)
	bc.mu.Unlock()
	log.Infof("Remove index #%d", target)
}

func (bc *Blockchain) RemoveMinedtiime() {
	if len(bc.minedtime) <= int(bc.targetPrune) {
		return
	}
	target := bc.targetHeight - int64(bc.targetPrune) - 1
	bc.mu.Lock()
	delete(bc.minedtime, target)
	bc.mu.Unlock()
	log.Infof("Remove minedtime #%d", target)
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
	bc.mu.Lock()
	// Check the old block
	if bc.index[bc.targetHeight] != nil {
		// Update prev block's mined time
		bc.minedtime[bc.targetHeight] = block.Time
	}
	newHeight := bc.targetHeight + 1
	bc.index[newHeight] = NewIndex()
	// Update curernt block height
	bc.targetHeight = newHeight
	log.Infof("now -> #%d", bc.targetHeight)
	//log.Info(bc.index, " ", bc.minedtime)
	bc.mu.Unlock()
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

func (bc *Blockchain) TxScore() *TxStore {
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
