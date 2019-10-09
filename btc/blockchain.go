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
	Bestblockhash string `json:"bestblockhash"`
}

type BlockChain struct {
	mempool        *Mempool
	resolver       *resolver.Resolver
	latestblock    int64
	pruneblocks    int
	blocktimes     []int64
	nextblockcount int64
	waitchan       chan Block
	tasks          []string
}

func NewBlockchain(uri string, pruneblocks int) *BlockChain {
	bc := &BlockChain{
		resolver:    resolver.NewResolver(uri),
		mempool:     NewMempool(uri),
		pruneblocks: pruneblocks,
		waitchan:    make(chan Block),
	}
	return bc
}

func (b *BlockChain) StartSync(t time.Duration) {
	go b.doLoadNewBlocks(t)
	go b.doLoadBlock(3 * time.Second)
}

func (b *BlockChain) StartMemSync(t time.Duration) {
	b.mempool.StartSync(t)
}

func (b *BlockChain) doLoadNewBlocks(t time.Duration) {
	err := b.loadNewBlocks()
	if err != nil {
		log.Info(err)
	}
	time.Sleep(t)
	go b.doLoadNewBlocks(t)
	return
}

func (b *BlockChain) doLoadBlock(t time.Duration) {
	err := b.getBlock()
	if err != nil {
		log.Info(err)
	}
	time.Sleep(t)
	go b.doLoadBlock(t)
	return
}

func (b *BlockChain) loadNewBlocks() error {
	info := ChainInfo{}
	err := b.resolver.GetRequest("/rest/chaininfo.json", &info)
	if err != nil {
		return err
	}

	if b.latestblock == 0 {
		b.latestblock = info.Blocks - 1
	}
	if b.latestblock >= info.Blocks {
		return nil
	}
	if b.latestblock < info.Blocks {
		b.nextblockcount = info.Blocks - b.latestblock
		b.latestblock = info.Blocks
	}
	log.Infof("Task Block# %d Push", b.latestblock)
	b.tasks = append(b.tasks, info.Bestblockhash)
	return nil
}

func (b *BlockChain) getBlock() error {
	if len(b.tasks) == 0 {
		return nil
	}
	task := b.tasks[0]
	b.tasks = b.tasks[1:]
	block := Block{}
	err := b.resolver.GetRequest("/rest/block/"+task+".json", &block)
	if err != nil {
		b.tasks = append(b.tasks, task)
		return err
	}
	if block.Height == 0 {
		b.tasks = append(b.tasks, task)
		return errors.New("block height is zero " + task)
	}
	b.blocktimes = append(b.blocktimes, block.Time)
	if len(b.blocktimes) > b.pruneblocks+1 {
		b.blocktimes = b.blocktimes[1:]
	}
	log.Infof("Task Block# %d Get", block.Height)
	b.waitchan <- block
	if b.nextblockcount <= 0 {
		return nil
	}
	b.nextblockcount--
	if b.nextblockcount > 0 {
		b.tasks = append(b.tasks, block.Previousblockhash)
	}
	return nil
}

func (b *BlockChain) GetLatestBlock() int64 {
	return b.latestblock
}

func (b *BlockChain) GetPruneBlockTime() (int64, error) {
	if len(b.blocktimes) == b.pruneblocks+1 {
		return b.blocktimes[0], nil
	}
	return 0, errors.New("prune block is not reached")
}
