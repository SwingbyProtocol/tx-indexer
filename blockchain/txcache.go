package blockchain

import (
	"errors"
	"sync"
	"time"

	"github.com/SwingbyProtocol/tx-indexer/api"
	"github.com/SwingbyProtocol/tx-indexer/types"
)

type TxCache struct {
	mu       *sync.RWMutex
	resolver *api.Resolver
	store    map[string]*types.Block
}

func NewCache(finalizer string) *TxCache {
	cache := &TxCache{
		mu:       new(sync.RWMutex),
		resolver: api.NewResolver(finalizer, 200),
		store:    make(map[string]*types.Block),
	}
	return cache
}

func (c *TxCache) GetTx(blockHash string, txid string) *types.Tx {
	c.mu.RLock()
	defer c.mu.RUnlock()
	for _, tx := range c.store[blockHash].Txs {
		if tx.Txid == txid {
			return tx
		}
	}
	return nil
}

func (c *TxCache) StoreBlock(blockHash string) error {
	block := types.Block{}
	err := c.resolver.GetRequest("/rest/block/"+blockHash+".json", &block)
	if err != nil {
		return err
	}
	if block.Height == 0 {
		return errors.New("block height is zero")
	}
	c.store[blockHash] = &block
	go func() {
		time.Sleep(2 * time.Hour)
		delete(c.store, blockHash)
	}()
	return nil
}

func (c *TxCache) GetLatestBlock() int64 {
	return 333
}
