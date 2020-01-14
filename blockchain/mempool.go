package blockchain

import (
	"encoding/json"
	"io/ioutil"
	"os"
	"sync"

	"github.com/SwingbyProtocol/tx-indexer/types"
)

type Mempool struct {
	mu    *sync.RWMutex
	store map[string]*types.Tx
}

func NewMempool() *Mempool {
	mpool := &Mempool{
		mu:    new(sync.RWMutex),
		store: make(map[string]*types.Tx),
	}
	return mpool
}

func (m *Mempool) AddTx(tx *types.Tx) {
	m.mu.RLock()
	m.store[tx.Txid] = tx
	m.mu.Unlock()
}

func (m *Mempool) GetTx(txid string) *types.Tx {
	m.mu.RLock()
	tx := m.store[txid]
	m.mu.RUnlock()
	return tx
}

func (m *Mempool) RemoveTx(txid string) {
	m.mu.Lock()
	delete(m.store, txid)
	m.mu.Unlock()
}

func (m *Mempool) Backup() error {
	m.mu.RLock()
	defer m.mu.RUnlock()
	str, err := json.Marshal(m.store)
	if err != nil {
		return err
	}
	f, err := os.Create("./data/mempool.backup")
	if err != nil {
		err = os.MkdirAll("./data", 0755)
		if err != nil {
			return err
		}
		return m.Backup()
	}
	_, err = f.Write(str)
	if err != nil {
		return err
	}
	return nil
}

func (m *Mempool) Load() error {
	data, err := ioutil.ReadFile("./data/mempool.backup")
	if err != nil {
		return err
	}
	err = json.Unmarshal(data, &m.store)
	if err != nil {
		return err
	}
	return nil
}
