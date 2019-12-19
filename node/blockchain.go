package node

import (
	"github.com/SwingbyProtocol/tx-indexer/common"
	"time"
)

type ChainInfo struct {
	Chain         string `json:"chain"`
	Blocks        int64  `json:"blocks"`
	Headers       int64  `json:"headers"`
	Bestblockhash string `json:"bestblockhash"`
}

type BlockChain struct {
	resolver       common.Resolver
	Latestblock    int64
	Blocktimes     []int64
	Blocks         []*Block
	Nextblockcount int64
}

func NewBlockchain() *BlockChain {
	bc := &BlockChain{}
	return bc
}

func (b *BlockChain) AddBlock(t time.Duration) {

}

func (b *BlockChain) doLoadNewBlocks(t time.Duration) {
}

func (b *BlockChain) LoadNewBlocks() error {
	info := ChainInfo{}
	err := b.resolver.GetRequest("/rest/chaininfo.json", &info)
	if err != nil {
		return err
	}
	block := Block{}
	err = b.resolver.GetRequest("/rest/block/"+info.Bestblockhash+".json", &block)
	if err != nil {
		return err
	}
	return nil
}

/*


func (b *BlockChain) getBlock() error {

	err := b.resolver.GetRequest("/rest/block/"+task.BlockHash+".json", &block)
	if err != nil {
		b.AddTaskWithError(task)
		return err
	}
	if block.Height == 0 {
		b.AddTaskWithError(task)
		return errors.New("Block height is zero " + task.BlockHash)
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
		task := Task{block.Previousblockhash, 0}
		b.tasks = append(b.tasks, &task)
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

func (b *BlockChain) AddTaskWithError(task *Task) {
	task.Errors++
	log.Info("task errors: ", task.Errors)
	if task.Errors <= 8 {
		b.tasks = append(b.tasks, task)
	}
}

*/
