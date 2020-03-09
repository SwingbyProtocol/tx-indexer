package blockchain

import (
	"errors"
	"sort"
	"strconv"
	"sync"
	"time"

	"github.com/SwingbyProtocol/tx-indexer/api"
	"github.com/SwingbyProtocol/tx-indexer/types"
	"github.com/SwingbyProtocol/tx-indexer/utils"
	"github.com/btcsuite/btcd/wire"
	log "github.com/sirupsen/logrus"
	"github.com/syndtr/goleveldb/leveldb"
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
	txmap           map[string]string
	db              *leveldb.DB
	Mempool         map[string]*types.Tx
	targetPrune     uint
	latestBlockHash string
	txChan          chan *types.Tx
	blockChan       chan *wire.MsgBlock
	pushMsgChan     chan *types.PushMsg
}

type HeighHash struct {
	BlockHash string `json:"blockhash"`
}

func NewBlockchain(conf *BlockchainConfig) *Blockchain {
	bc := &Blockchain{
		mu:          new(sync.RWMutex),
		resolver:    api.NewResolver(conf.TrustedNode),
		index:       NewIndex(),
		Blocks:      make(map[int64]*types.Block),
		txmap:       make(map[string]string),
		Mempool:     make(map[string]*types.Tx),
		targetPrune: conf.PruneSize,
		txChan:      make(chan *types.Tx, 100000000),
		blockChan:   make(chan *wire.MsgBlock),
		pushMsgChan: make(chan *types.PushMsg),
	}
	db, err := leveldb.OpenFile("./data/leveldb", nil)
	if err != nil {
		log.Fatal(err)
	}
	bc.db = db
	log.Infof("Start block syncing with pruneSize: %d", conf.PruneSize)
	log.Infof("Using trusted node: %s", conf.TrustedNode)
	return bc
}

func (bc *Blockchain) AddTxMap(height int64) error {
	bh := HeighHash{}
	err := bc.resolver.GetRequest("/rest/blockhashbyheight/"+strconv.Itoa(int(height))+".json", &bh)
	if err != nil {
		return err
	}
	block := types.BlockTxs{}
	err = bc.resolver.GetRequest("/rest/block/notxdetails/"+bh.BlockHash+".json", &block)
	if err != nil {
		log.Info("error ", height)
		return err
	}
	if block.Height == 0 {
		log.Info("error ", height)
		return errors.New("block is zero")
	}
	for _, txid := range block.Txs {
		//bc.mu.Lock()
		//bc.txmap[txid] = block.Hash
		//bc.mu.Unlock()
		bc.StoreData(txid, block.Hash)
		log.Info("stored ", txid, " ", block.Height)
	}
	return nil
}

func (bc *Blockchain) GetData(key string) (string, error) {
	txhash, err := bc.db.Get([]byte(key), nil)
	if err != nil {
		return "", err
	}
	return string(txhash), nil
}

func (bc *Blockchain) GetMempool() []*types.Tx {
	bc.mu.RLock()
	defer bc.mu.RUnlock()
	txs := []*types.Tx{}
	for _, tx := range bc.Mempool {
		txs = append(txs, tx)
	}
	return txs
}

func (bc *Blockchain) StoreData(key string, data string) error {
	_, err := bc.GetData(data)
	if err != nil {
		bc.db.Put([]byte(key), []byte(data), nil)
		return nil
	}
	return errors.New("tx map is already exist")
}

func (bc *Blockchain) WatchTx() {
	for {
		tx := <-bc.txChan
		log.Infof("remain tx %d", len(bc.txChan))
		storedTx, mempool := bc.GetTx(tx.Txid)
		// Tx is not exist, add to mempool from tx
		if storedTx == nil && !mempool {
			// add new tx
			bc.UpdateIndex(tx)
			// Add tx to mempool
			bc.AddMempoolTx(tx)
			log.Infof("new tx came add to mempool %s", tx.Txid)
			continue
		}
		// Tx is on the mempool and kv
		if storedTx != nil && mempool && tx.Height != 0 {
			bc.UpdateIndex(tx)
			// Remove Tx from mempool
			go func() {
				time.Sleep(30 * time.Minute)
				bc.RemoveMempoolTx(tx)
			}()
			log.Infof("already tx exist on mempool future remove (mined) %s", tx.Txid)
			continue
		}
		// Tx is on the kv
		if storedTx != nil && !mempool {
			bc.UpdateIndex(tx)
			// Remove Tx from mempool
			//bc.RemoveMempoolTx(tx)
			log.Infof("already tx exist on kv updated not mempool %s", tx.Txid)
			continue
		}
		//log.Info("remaining sync count -> ", len(bc.txChan))
	}
}

func (bc *Blockchain) WatchBlock() {
	for {
		// TODO: Getting block from P2P network
		_ = <-bc.blockChan
		// block := MsgBlockToBlock(msg)

		// Get block data from TrustedPeer for now
		err := bc.syncBlocks(8)
		if err != nil {
			log.Debug(err)
			continue
		}
		latest := bc.GetLatestBlock()
		log.Infof("Now block -> #%d %s", latest.Height, latest.Hash)
	}
}

func (bc *Blockchain) Start() {
	// Once sync blocks
	err := bc.syncBlocks(10)
	if err != nil {
		log.Fatal(err)
	}

	go bc.WatchBlock()
	go bc.WatchTx()

	latest := bc.GetLatestBlock()

	//jobs := make(chan int64, 500)

	//for i := 0; i < 160; i++ {
	//	go bc.SyncWork(i, jobs)
	//}
	//for i := 0; i < 4; i++ {
	//		jobs <- latest.Height - int64(i)
	//	}

	//close(jobs)

	log.Infof("Now block -> #%d %s", latest.Height, latest.Hash)

}

func (bc *Blockchain) SyncWork(id int, jobs chan int64) {
	for height := range jobs {
		err := bc.AddTxMap(height)
		if err != nil {
			log.Info(err)
			jobs <- height
		}
	}
}

func (bc *Blockchain) UpdateIndex(tx *types.Tx) {
	// Check tx input to update indexer storage
	for _, in := range tx.Vin {
		// Load a tx from storage
		inTx, _ := bc.GetTx(in.Txid)
		if inTx == nil {
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
	// Check tx output to update indexer storage
	addrs := tx.GetOutsAddrs()
	for _, addr := range addrs {
		// Update index and the spent tx (unspent)
		bc.index.Update(addr, tx.Txid, Received)
		// Publish tx to notification handler
		bc.pushMsgChan <- &types.PushMsg{Tx: tx, Addr: addr, State: Received}
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

func (bc *Blockchain) GetLatestBlock() *types.Block {
	bc.mu.RLock()
	defer bc.mu.RUnlock()
	top := int64(0)
	for height := range bc.Blocks {
		if height > top {
			top = height
		}
	}
	latest := bc.Blocks[top]
	return latest
}

func (bc *Blockchain) syncBlocks(depth int) error {
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
		allBlocks, err := bc.GetRemoteBlocks(depth)
		if err != nil {
			log.Warn(err)
			return err
		}
		blocks = allBlocks
	}
	for _, block := range blocks {
		for _, tx := range block.Txs {
			for _, vout := range tx.Vout {
				vout.Value = utils.ValueSat(vout.Value)
				vout.Addresses = vout.Scriptpubkey.Addresses
				if vout.Addresses == nil {
					vout.Addresses = []string{"OP_RETURN"}
				}
				vout.Txs = []string{}
			}
		}
	}
	for _, block := range blocks {
		bc.mu.Lock()
		bc.Blocks[block.Height] = block
		bc.mu.Unlock()
		log.Infof("Stored new block -> %d", block.Height)
	}

	nowHash := bc.GetLatestBlock().Hash
	// Add latest block hash
	bc.latestBlockHash = nowHash
	// Add txs to txChan
	for i := len(blocks) - 1; i >= 0; i-- {
		for _, tx := range blocks[i].Txs {
			tx.AddBlockData(blocks[i].Height, blocks[i].Time, blocks[i].Mediantime)
			tx.Receivedtime = blocks[i].Time
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

func (bc *Blockchain) GetTxs(txids []string, mem bool) []*types.Tx {
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
	bc.Mempool[tx.Txid] = tx
	bc.mu.Unlock()
	go func() {
		time.Sleep(40 * time.Minute)
		bc.mu.Lock()
		delete(bc.Mempool, tx.Txid)
		bc.mu.Unlock()
	}()
}

func (bc *Blockchain) RemoveMempoolTx(tx *types.Tx) {
	bc.mu.Lock()
	delete(bc.Mempool, tx.Txid)
	bc.mu.Unlock()
}

func (bc *Blockchain) GetIndexTxsWithTW(addr string, start int64, end int64, state int, mempool bool) []*types.Tx {
	if end == 0 {
		end = int64(^uint(0) >> 1)
	}
	txids := bc.index.GetTxIDs(addr, state)
	txs := bc.GetTxs(txids, mempool)
	res := []*types.Tx{}
	for _, tx := range txs {
		if tx.MinedTime == 0 && !mempool {
			continue
		}
		if mempool {
			if tx.MinedTime != 0 {
				continue
			}
			res = append(res, tx)
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
	uri := "/rest/block/" + blockHash + ".json"
	log.Info(uri)
	err := bc.resolver.GetRequest(uri, &block)
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
