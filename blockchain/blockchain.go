package blockchain

import (
	"errors"
	"sort"
	"strconv"
	"sync"

	"github.com/SwingbyProtocol/tx-indexer/api"
	"github.com/SwingbyProtocol/tx-indexer/types"
	"github.com/btcsuite/btcd/wire"
	log "github.com/sirupsen/logrus"
)

type BlockchainConfig struct {
	// TrustedNode is ip addr for connect to rest api
	TrustedNode string
	// PruneSize is holding height
	PruneSize uint
}

type Blockchain struct {
	mu              *sync.RWMutex
	resolver        *api.Resolver
	index           *Index // index is stored per block
	Blocks          map[int64]*types.Block
	Mempool         map[string]*types.Tx
	targetPrune     uint
	latestBlockHash string
	txChan          chan *types.Tx
	blockChan       chan *wire.MsgBlock
	pushMsgChan     chan *types.PushMsg
}

func NewBlockchain(conf *BlockchainConfig) *Blockchain {
	bc := &Blockchain{
		mu:          new(sync.RWMutex),
		resolver:    api.NewResolver(conf.TrustedNode),
		index:       NewIndex(),
		Blocks:      make(map[int64]*types.Block),
		Mempool:     make(map[string]*types.Tx),
		targetPrune: conf.PruneSize,
		txChan:      make(chan *types.Tx, 70000),
		blockChan:   make(chan *wire.MsgBlock),
		pushMsgChan: make(chan *types.PushMsg),
	}
	log.Infof("Start block syncing with pruneSize: %d", conf.PruneSize)
	log.Infof("Using trusted node: %s", conf.TrustedNode)
	return bc
}

func (bc *Blockchain) WatchTx() {
	for {
		tx := <-bc.txChan
		storedTx, mempool := bc.GetTx(tx.Txid)
		// Tx is not exist, add to mempool from tx
		if storedTx == nil && !mempool {
			// add new tx
			bc.UpdateIndex(tx)
			// Add tx to mempool
			bc.AddMempoolTx(tx)
			log.Debugf("new tx came add to mempool %s witness %t", tx.Txid, tx.MsgTx.HasWitness())
			continue
		}
		// Tx is on the mempool and kv
		if storedTx != nil && mempool && tx.Confirms != 0 {
			bc.UpdateIndex(tx)
			// Remove Tx from mempool
			bc.RemoveMempoolTx(tx)
			log.Debugf("already tx exist on mempool removed %s", tx.Txid)
			continue
		}
		// Tx is on the kv
		if storedTx != nil && !mempool {
			bc.UpdateIndex(tx)
			log.Debugf("already tx exist on kv updated %s", tx.Txid)
			continue
		}
	}
}

func (bc *Blockchain) WatchBlock() {
	for {
		// TODO: Getting block from P2P network
		_ = <-bc.blockChan
		// block := MsgBlockToBlock(msg)

		// Get block data from TrustedPeer for now
		err := bc.syncBlocks()
		if err != nil {
			log.Debug(err)
			continue
		}
		latest := bc.GetLatestBlock()
		log.Infof("Now block -> #%d %s", latest.Height, latest.Hash)
	}
}

func (bc *Blockchain) UpdateIndex(tx *types.Tx) {
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
			bc.pushMsgChan <- &types.PushMsg{Tx: tx, Addr: addrs[0], State: Send}
		}
		// Remove tx that all consumed output
		//bc.txStore.RemoveAllSpentTx(inTx)
	}
	for _, out := range tx.Vout {
		valueStr := strconv.FormatFloat(out.Value.(float64), 'f', -1, 64)
		out.Value = valueStr
		out.Txs = []string{}
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
		bc.pushMsgChan <- &types.PushMsg{Tx: tx, Addr: addr, State: Received}
	}
}

func (bc *Blockchain) Start() {
	// load data from files
	err := bc.Load()
	if err != nil {
		log.Error(err)
		log.Info("Skip load process...")
	}
	// Once sync blocks
	err = bc.syncBlocks()
	if err != nil {
		log.Fatal(err)
	}
	latest := bc.GetLatestBlock()
	log.Infof("Now block -> #%d %s", latest.Height, latest.Hash)
	go bc.WatchBlock()
	go bc.WatchTx()
}

func (bc *Blockchain) GetLatestBlock() *types.Block {
	bc.mu.RLock()
	defer bc.mu.RUnlock()
	top := int64(0)
	for height := range bc.Blocks {
		if height > top {
			top = height
		}
	}
	return bc.Blocks[top]
}

func (bc *Blockchain) syncBlocks() error {
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
		log.Warnf("Latest block may be orphan, hash does not match. search more blocks.. prevhash: %s", blocks[0].Previousblockhash)
		allBlocks, err := bc.GetRemoteBlocks(8)
		if err != nil {
			log.Warn(err)
			return err
		}
		blocks = allBlocks
	}
	for _, block := range blocks {
		bc.Blocks[block.Height] = block
		log.Infof("Stored new block -> %d", block.Height)
	}
	nowHash := bc.GetLatestBlock().Hash
	// Add latest block hash
	bc.latestBlockHash = nowHash
	// Add txs to txChan
	for _, block := range blocks {
		for _, tx := range block.Txs {
			tx.AddBlockData(block.Height, block.Time, block.Mediantime)
			tx.Receivedtime = block.Time
			// Add tx to chan
			bc.txChan <- tx
		}
	}
	return nil
}

func (bc *Blockchain) GetTx(txid string) (*types.Tx, bool) {
	bc.mu.RLock()
	defer bc.mu.RUnlock()
	for key, tx := range bc.Mempool {
		if key == txid {
			return tx, true
		}
	}
	for _, block := range bc.Blocks {
		for _, tx := range block.Txs {
			if tx.Txid == txid {
				return tx, false
			}
		}
	}
	return nil, false
}

func (bc *Blockchain) GetTxs(txids []string) []*types.Tx {
	txs := []*types.Tx{}
	for _, txid := range txids {
		tx, _ := bc.GetTx(txid)
		if tx == nil {
			continue
		}
		txs = append(txs, tx)
	}
	return txs
}

func (bc *Blockchain) AddMempoolTx(tx *types.Tx) {
	bc.mu.Lock()
	defer bc.mu.Unlock()
	bc.Mempool[tx.Txid] = tx
}

func (bc *Blockchain) RemoveMempoolTx(tx *types.Tx) {
	bc.mu.Lock()
	defer bc.mu.Unlock()
	delete(bc.Mempool, tx.Txid)
}

func (bc *Blockchain) GetIndexTxsWithTW(addr string, start int64, end int64, state int, mempool bool) []*types.Tx {
	if end == 0 {
		end = int64(^uint(0) >> 1)
	}
	txids := bc.index.GetTxIDs(addr, state)
	txs := bc.GetTxs(txids)
	res := []*types.Tx{}
	for _, tx := range txs {
		if tx.MinedTime == 0 && !mempool {
			continue
		}
		if tx.MinedTime >= start && tx.MinedTime <= end {
			res = append(res, tx)
		}
	}
	sortTx(res)
	return res
}

func (bc *Blockchain) GetRemoteBlocks(depth int) ([]*types.Block, error) {
	info := types.ChainInfo{}
	err := bc.resolver.GetRequest("/rest/chaininfo.json", &info)
	if err != nil {
		return nil, err
	}
	newBlocks, err := bc.GetDepthBlocks(info.Bestblockhash, depth, []*types.Block{})
	if err != nil {
		return nil, err
	}
	return newBlocks, nil
}

func (bc *Blockchain) GetDepthBlocks(blockHash string, depth int, blocks []*types.Block) ([]*types.Block, error) {
	block := types.Block{}
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

func (bc *Blockchain) TxChan() chan *types.Tx {
	return bc.txChan
}

func (bc *Blockchain) BlockChan() chan *wire.MsgBlock {
	return bc.blockChan
}

func (bc *Blockchain) PushMsgChan() chan *types.PushMsg {
	return bc.pushMsgChan
}

func sortTx(txs []*types.Tx) {
	sort.SliceStable(txs, func(i, j int) bool { return txs[i].Receivedtime > txs[j].Receivedtime })
}
