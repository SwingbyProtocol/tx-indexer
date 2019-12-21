package blockchain

import (
	"errors"
	"sync"
)

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

func (ts *TxStore) AddTx(tx *Tx) error {
	if ts.GetTx(tx.GetHash()) != nil {
		return errors.New("tx is already joind")
	}
	ts.mu.Lock()
	ts.txs[tx.GetHash()] = tx
	ts.mu.Unlock()
	return nil
}

func (ts *TxStore) GetTx(id string) *Tx {
	ts.mu.RLock()
	tx := ts.txs[id]
	ts.mu.RUnlock()
	return tx
}
