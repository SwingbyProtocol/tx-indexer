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
	index           *Index // index is stored per block
	blocks          map[int64]*Block
	mempool         map[string]*Tx
	targetPrune     uint
	latestBlockHash string
	msgTxChan       chan *wire.MsgTx
	txChan          chan *Tx
	blockChan       chan *wire.MsgBlock
	pushMsgChan     chan *PushMsg
}

func NewBlockchain(conf *BlockchainConfig) *Blockchain {
	bc := &Blockchain{
		mu:          new(sync.RWMutex),
		resolver:    NewResolver(conf.TrustedNode),
		index:       NewIndex(),
		blocks:      make(map[int64]*Block),
		mempool:     make(map[string]*Tx),
		targetPrune: conf.PruneSize,
		msgTxChan:   make(chan *wire.MsgTx),
		txChan:      make(chan *Tx, 10000),
		blockChan:   make(chan *wire.MsgBlock),
		pushMsgChan: make(chan *PushMsg),
	}
	log.Infof("Using trusted node: %s", conf.TrustedNode)

	//bc.index[block.Height] = NewIndex()
	//bc.FinalizeBlock(block)
	return bc
}

func (bc *Blockchain) AddMempoolTx(tx *Tx) {
	bc.mu.Lock()
	defer bc.mu.Unlock()
	bc.mempool[tx.Txid] = tx
}

func (bc *Blockchain) RemoveMempoolTx(tx *Tx) {
	bc.mu.Lock()
	defer bc.mu.Unlock()
	delete(bc.mempool, tx.Txid)
}

func (bc *Blockchain) Start() {
	err := bc.SyncBlocks()
	if err != nil {
		log.Fatal(err)
	}
	latest := bc.GetLatestBlock()
	log.Infof("Get block -> #%d %s", latest.Height, latest.Hash)
	go func() {
		for {
			tx := <-bc.txChan
			storedTx, mempool := bc.GetTx(tx.Txid)
			// Tx is not exist add to mempool
			if storedTx == nil && tx.Confirms == 0 {
				// add new tx
				bc.UpdateIndex(tx)
				// Add tx to mempool
				bc.AddMempoolTx(tx)
				log.Debugf("new tx came %s witness %t", tx.Txid, tx.MsgTx.HasWitness())
			}
			// Tx is on the kv
			if storedTx != nil && !mempool {
				if storedTx.Confirms != 0 {
					continue
				}
				bc.UpdateIndex(tx)
				// Remove Tx from mempool
				bc.RemoveMempoolTx(tx)
				log.Debugf("already tx exist on mempool %s", tx.Txid)
			}
		}
	}()

	go func() {
		for {
			msg := <-bc.msgTxChan
			tx := MsgTxToTx(msg)
			bc.txChan <- &tx
		}
	}()

	go func() {
		for {
			// TODO: Getting block from P2P network
			_ = <-bc.blockChan
			// block := MsgBlockToBlock(msg)

			// Get block data from TrustedPeer for now
			err := bc.SyncBlocks()
			if err != nil {
				log.Info(err)
				continue
			}
			latest := bc.GetLatestBlock()
			log.Infof("Get block -> #%d %s", latest.Height, latest.Hash)
		}
	}()
}

func (bc *Blockchain) GetLatestBlock() *Block {
	bc.mu.RLock()
	defer bc.mu.RUnlock()
	top := int64(0)
	for height := range bc.blocks {
		if height > top {
			top = height
		}
	}
	return bc.blocks[top]
}

func (bc *Blockchain) SyncBlocks() error {
	// Check hash of prev block
	blocks, err := bc.GetRemoteBlocks(1)
	if err != nil {
		return err
	}
	latestHash := bc.latestBlockHash
	if blocks[0].Hash == latestHash {
		return errors.New("height is latest")
	}
	// Latest block hash is checked same as the latest previous hash
	if blocks[0].Previousblockhash != latestHash {
		log.Infof("laest block hash is not match. search more blocks prevHash: %s", blocks[0].Previousblockhash)
		allBlocks, err := bc.GetRemoteBlocks(5)
		if err != nil {
			log.Info(err)
			return err
		}
		blocks = allBlocks
	}
	for _, block := range blocks {
		bc.blocks[block.Height] = block
	}
	nowHash := bc.GetLatestBlock().Hash
	// Add latest block hash
	bc.latestBlockHash = nowHash
	// Add txs to txChan
	for _, block := range blocks {
		for _, tx := range block.Txs {
			tx.AddBlockData(block.Height, block.Time, block.Mediantime)
			tx.Receivedtime = block.Time
			bc.txChan <- tx
		}
	}
	return nil
}

func (bc *Blockchain) GetTx(txid string) (*Tx, bool) {
	bc.mu.RLock()
	defer bc.mu.RUnlock()
	for _, block := range bc.blocks {
		for _, tx := range block.Txs {
			if tx.Txid == txid {
				return tx, false
			}
		}
	}
	for key, tx := range bc.mempool {
		if key == txid {
			return tx, true
		}
	}
	return nil, false
}

func (bc *Blockchain) GetTxs(txids []string) []*Tx {
	txs := []*Tx{}
	for _, txid := range txids {
		tx, _ := bc.GetTx(txid)
		if tx == nil {
			continue
		}
		txs = append(txs, tx)
	}
	return txs
}

func (bc *Blockchain) GetIndexTxsWithTW(addr string, start int64, end int64, state int, mempool bool) ([]*Tx, error) {
	if end == 0 {
		end = int64(^uint(0) >> 1)
	}
	// Seek all index
	txidsAll := []string{}

	if bc.index != nil {
		txids := bc.index.GetTxIDs(addr, state)
		for _, tx := range txids {
			txidsAll = append(txidsAll, tx)
		}
	}

	txs := bc.GetTxs(txidsAll)

	res := []*Tx{}
	for _, tx := range txs {
		if tx.MinedTime == 0 && !mempool {
			continue
		}
		if tx.MinedTime >= start && tx.MinedTime <= end {
			res = append(res, tx)
		}
	}
	sortTx(res)
	return res, nil
}

func (bc *Blockchain) UpdateIndex(tx *Tx) {
	// Check tx input to update indexer storage
	for _, in := range tx.Vin {
		// Load a tx from storage
		inTx, _ := bc.GetTx(in.Txid)
		if inTx == nil {
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
			bc.index.Update(addrs[0], tx.Txid, Send)
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
		bc.index.Update(addr, tx.Txid, Received)
		// Publish tx to notification handler
		bc.pushMsgChan <- &PushMsg{Tx: tx, Addr: addr, State: Received}
	}
}

func (bc *Blockchain) GetRemoteBlocks(depth int) ([]*Block, error) {
	info := ChainInfo{}
	err := bc.resolver.GetRequest("/rest/chaininfo.json", &info)
	if err != nil {
		return nil, err
	}
	newBlocks, err := bc.GetDepthBlocks(info.Bestblockhash, depth, []*Block{})
	if err != nil {
		return nil, err
	}
	return newBlocks, nil
}

func (bc *Blockchain) GetDepthBlocks(blockHash string, depth int, blocks []*Block) ([]*Block, error) {
	block := Block{}
	err := bc.resolver.GetRequest("/rest/block/"+blockHash+".json", &block)
	if err != nil {
		return nil, err
	}
	if block.Height == 0 {
		return nil, errors.New("block height is zero")
	}
	blocks = append(blocks, &block)
	if depth != 0 {
		depth = depth - 1
	}
	if depth == 0 {
		return blocks, nil
	}
	return bc.GetDepthBlocks(block.Previousblockhash, depth, blocks)
}

func (bc *Blockchain) TxChan() chan *wire.MsgTx {
	return bc.msgTxChan
}

func (bc *Blockchain) BlockChan() chan *wire.MsgBlock {
	return bc.blockChan
}

func (bc *Blockchain) PushMsgChan() chan *PushMsg {
	return bc.pushMsgChan
}

/*
func (bc *Blockchain) FinalizeBlock(block *Block) {
	bc.mu.Lock()
	newHeight := bc.commitHeight + 1
	bc.index[newHeight] = NewIndex()
	// Update curernt block height
	bc.commitHeight = newHeight
	log.Infof("now -> #%d", bc.commitHeight)
	bc.mu.Unlock()
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

// RemoveOldIndex invoked when prune block is came
func (bc *Blockchain) RemoveOldIndex() {
	if len(bc.index) <= int(bc.targetPrune)+1 {
		return
	}
	target := bc.commitHeight - int64(bc.targetPrune) - 1
	bc.mu.Lock()
	delete(bc.index, target)
	bc.mu.Unlock()
	log.Infof("Remove index #%d", target)
}
*/
