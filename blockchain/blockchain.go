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
	// Finalizer is peer address to finalize block and txs
	Finalizer string
	// PruneSize is holding height
}

type Blockchain struct {
	mu          *sync.RWMutex
	resolver    *api.Resolver
	index       *Index
	cache       *Cache
	txMap       *TxMap
	mempool     *Mempool
	TxChan      chan *wire.MsgTx
	BChan       chan *wire.MsgBlock
	PushMsgChan chan *types.PushMsg
}

type HeighHash struct {
	BlockHash string `json:"blockhash"`
}

func NewBlockchain(conf *BlockchainConfig) *Blockchain {
	bc := &Blockchain{
		mu:          new(sync.RWMutex),
		index:       NewIndex(),
		cache:       NewCache(conf.Finalizer),
		txMap:       NewTxMap(conf.Finalizer),
		mempool:     NewMempool(),
		TxChan:      make(chan *wire.MsgTx, 100000),
		BChan:       make(chan *wire.MsgBlock),
		PushMsgChan: make(chan *types.PushMsg),
	}
	log.Infof("Using Finalizer node: %s", conf.Finalizer)
	return bc
}

func (bc *Blockchain) GetLatestBlock() int64 {
	return bc.cache.GetLatestBlock()
}

func (bc *Blockchain) WatchTx() {
	for {
		tx := <-bc.TxChan
		log.Info(tx.TxHash().String())
		/*
			storedTx, mempool := bc.GetTx(tx.Txid)
			// Tx is not exist, add to mempool from tx
			if storedTx == nil && !mempool {
				// add new tx
				bc.UpdateIndex(tx)
				// Add tx to mempool
				bc.AddMempoolTx(tx)
				log.Debugf("new tx came add to mempool %s witness %t", tx.Txid, tx.HasWitness())
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
		*/
	}
}

func (bc *Blockchain) WatchBlock() {
	for {
		// TODO: Getting block from P2P network
		_ = <-bc.BChan
		// block := MsgBlockToBlock(msg)

		// Get block data from TrustedPeer for now

	}
}

func (bc *Blockchain) Start() {
	// load data from files
	err := bc.txMap.Load()
	if err != nil {
		log.Error(err)
		log.Info("Skip load process...")
	}
	go bc.WatchBlock()
	go bc.WatchTx()
	go bc.txMap.Start()
	// Once sync blocks
}

func (bc *Blockchain) UpdateIndex(tx *types.Tx) {
	// Check tx input to update indexer storage
	for _, in := range tx.Vin {
		// Load a tx from storage
		inTx, _ := bc.GetTx(in.Txid)
		if inTx == nil {
			// continue if spent tx is not exist
			// check coinbase
			if in.Txid == "" {
				in.Value = "coinbase"
				in.Addresses = []string{"coinbase"}
				continue
			}
			targetHash := "ss"
			if targetHash == "" {
				in.Value = "not exist"
				in.Addresses = []string{"not exist"}
				continue
			}
			getBlock, err := bc.NewBlock(targetHash)
			if err != nil {
				in.Value = "not exist"
				in.Addresses = []string{"not exist"}
				continue
			}
			for _, btx := range getBlock.Txs {
				if btx.Txid == in.Txid {
					vout := btx.Vout[in.Vout]
					in.Value = strconv.FormatFloat(vout.Value.(float64), 'f', -1, 64)
					in.Addresses = vout.Scriptpubkey.Addresses
				}
			}
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
			bc.PushMsgChan <- &types.PushMsg{Tx: tx, Addr: addrs[0], State: Send}
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
		bc.PushMsgChan <- &types.PushMsg{Tx: tx, Addr: addr, State: Received}
	}
}

func (bc *Blockchain) NewBlock(hash string) (*types.Block, error) {
	block := types.Block{}
	err := bc.resolver.GetRequest("/rest/block/"+hash+".json", &block)
	if err != nil {
		return nil, err
	}
	return &block, nil
}

func (bc *Blockchain) GetTx(txid string) (*types.Tx, bool) {
	mTx := bc.mempool.GetTx(txid)
	if mTx != nil {
		return mTx, true
	}
	tHash := bc.txMap.GetHash(txid)
	if tHash == "" {
		return nil, false
	}
	bTx := bc.cache.GetTx(tHash, txid)
	if bTx == nil {
		return nil, false
	}
	return bTx, false
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

func (bc *Blockchain) GetUTXOData(txhash string, vout int) ([]*types.UTXO, error) {
	utxos := types.UTXOs{}
	err := bc.resolver.GetRequest("/rest/getutxos/checkmempool/"+txhash+"-"+strconv.Itoa(vout)+".json", &utxos)
	if err != nil {
		return nil, err
	}
	if len(utxos.Utxos) == 0 {
		return nil, errors.New("utxo is not exist")
	}
	return utxos.Utxos, nil
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

func sortTx(txs []*types.Tx) {
	sort.SliceStable(txs, func(i, j int) bool { return txs[i].Receivedtime > txs[j].Receivedtime })
}
