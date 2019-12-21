package blockchain

import "sync"

type TxStore struct {
	mu  *sync.RWMutex
	txs map[string]*Tx
}

func NewTxStore() *TxStore {
	store := &TxStore{
		mu:  new(sync.RWMutex),
		txs: make(map[string]*Tx),
	}
	return store
}

func (t *TxStore) AddTx(tx *Tx) {
	t.txs[tx.GetHash()] = tx
}

func (t *TxStore) GetTx(id string) *Tx {
	t.mu.RLock()
	tx := t.txs[id]
	t.mu.RUnlock()
	return tx
}
