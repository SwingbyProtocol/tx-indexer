package btc

import (
	"errors"
	"time"

	"github.com/SwingbyProtocol/sc-indexer/resolver"
	log "github.com/sirupsen/logrus"
)

type ChainInfo struct {
	Chain         string `json:"chain"`
	Blocks        int64  `json:"blocks"`
	Headers       int64  `json:"headers"`
	BestBlockHash string `json:"bestblockhash"`
}

type BlockChain struct {
	Mempool        *Mempool
	Resolver       *resolver.Resolver
	LatestBlock    int64
	NextBlockCount int64
	BlockChan      chan Block
	Tasks          []string
	isSync         bool
}

func NewBlockchain(uri string) *BlockChain {
	bc := &BlockChain{
		Resolver:  resolver.NewResolver(uri),
		Mempool:   NewMempool(uri),
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
	err := b.loadNewBlocks()
	if err != nil {
		log.Info(err)
	}
	time.Sleep(t)
	b.doLoadNewBlocks(t)
}

func (b *BlockChain) doLoadBlock(t time.Duration) {
	err := b.getBlock()
	if err != nil {
		log.Info(err)
	}
	time.Sleep(t)
	b.doLoadBlock(t)
}

func (b *BlockChain) loadNewBlocks() error {
	info := ChainInfo{}
	err := b.Resolver.GetRequest("/rest/chaininfo.json", &info)
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

func (b *BlockChain) getBlock() error {
	if len(b.Tasks) == 0 {
		return nil
	}
	task := b.Tasks[0]
	b.Tasks = b.Tasks[1:]
	block := Block{}
	err := b.Resolver.GetRequest("/rest/block/"+task+".json", &block)
	if err != nil {
		b.Tasks = append(b.Tasks, task)
		return err
	}
	if block.Height == 0 {
		b.Tasks = append(b.Tasks, task)
		return errors.New("block height is zero")
	}
	log.Info("get txs from block -> ", block.Height)
	b.BlockChan <- block
	if b.NextBlockCount <= 0 {
		return nil
	}
	b.NextBlockCount--
	if b.NextBlockCount > 0 {
		b.Tasks = append(b.Tasks, block.Previousblockhash)
	}
	return nil
}
