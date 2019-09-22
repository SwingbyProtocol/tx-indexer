package btc

import (
	"errors"
	"time"

	"github.com/SwingbyProtocol/sc-indexer/resolver"
	log "github.com/sirupsen/logrus"
)

type BlockChain struct {
	Blocks         []*Block
	Mempool        *Mempool
	URI            string
	Resolver       *resolver.Resolver
	LatestBlock    int64
	NextBlockCount int64
	BlockChan      chan Block
	Tasks          []string
	isSync         bool
}

type ChainInfo struct {
	Chain         string `json:"chain"`
	Blocks        int64  `json:"blocks"`
	Headers       int64  `json:"headers"`
	BestBlockHash string `json:"bestblockhash"`
}

func NewBlockchain(uri string) *BlockChain {
	bc := &BlockChain{
		Resolver:  resolver.NewResolver(),
		Mempool:   NewMempool(uri),
		URI:       uri,
		BlockChan: make(chan Block),
	}
	return bc
}

func (b *BlockChain) StartSync(t time.Duration) {
	go b.doLoadNewBlocks(t)
	go b.doLoadBlock(3 * time.Second)
}

func (b *BlockChain) StartMemSync(t time.Duration) {
	b.Mempool.StartSync(t)
}

func (b *BlockChain) doLoadNewBlocks(t time.Duration) {
	err := b.LoadNewBlocks()
	if err != nil {
		log.Info(err)
	}
	time.Sleep(t)
	b.doLoadNewBlocks(t)
}

func (b *BlockChain) doLoadBlock(t time.Duration) {
	err := b.GetBlock()
	if err != nil {
		log.Info(err)
	}
	time.Sleep(t)
	b.doLoadBlock(t)
}

func (b *BlockChain) LoadNewBlocks() error {
	info := ChainInfo{}
	err := b.Resolver.GetRequest(b.URI, "/rest/chaininfo.json", &info)
	if err != nil {
		return err
	}
	if b.LatestBlock == 0 {
		b.LatestBlock = info.Blocks - 1
	}
	if b.LatestBlock == info.Blocks {
		return nil
	}
	if b.LatestBlock < info.Blocks {
		b.NextBlockCount = info.Blocks - b.LatestBlock
		b.LatestBlock = info.Blocks
	}
	log.Info("push block task -> ", b.LatestBlock)
	b.Tasks = append(b.Tasks, info.BestBlockHash)
	return nil
}

func (b *BlockChain) GetBlock() error {
	if len(b.Tasks) == 0 {
		return nil
	}
	task := b.Tasks[0]
	b.Tasks = b.Tasks[1:]
	block := Block{}
	err := b.Resolver.GetRequest(b.URI, "/rest/block/"+task+".json", &block)
	if err != nil {
		b.Tasks = append(b.Tasks, task)
		return err
	}
	if block.Height == 0 {
		b.Tasks = append(b.Tasks, task)
		return errors.New("block height is zero")
	}
	b.BlockChan <- block
	log.Info("get txs from block -> ", block.Height)
	if b.NextBlockCount <= 0 {
		return nil
	}
	b.NextBlockCount--
	if b.NextBlockCount > 0 {
		b.Tasks = append(b.Tasks, block.Previousblockhash)
	}
	b.Blocks = append(b.Blocks, &block)
	return nil
}
