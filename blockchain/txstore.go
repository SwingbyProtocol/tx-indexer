package blockchain

import (
	"errors"
	"sort"
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
	_, err := ts.GetTx(tx.GetTxID())
	if err == nil {
		return errors.New("tx is already joind")
	}
	ts.mu.Lock()
	ts.txs[tx.GetTxID()] = tx
	ts.mu.Unlock()
	return nil
}

func (ts *TxStore) UpdateTx(tx *Tx) {
	ts.mu.Lock()
	ts.txs[tx.GetTxID()] = tx
	ts.mu.Unlock()
}

func (ts *TxStore) DeleteAllSpentTx(tx *Tx) error {
	allspent := true
	for _, vout := range tx.Vout {
		if !vout.Spent {
			allspent = false
		}
	}
	if allspent {
		ts.mu.Lock()
		delete(ts.txs, tx.GetTxID())
		ts.mu.Unlock()
		return nil
	}
	return errors.New("remove error")
}

func (ts *TxStore) GetTx(txid string) (*Tx, error) {
	if txid == "" {
		return nil, errors.New("id is null")
	}
	ts.mu.RLock()
	tx := ts.txs[txid]
	ts.mu.RUnlock()
	if tx == nil {
		return nil, errors.New("tx is not exist")
	}
	return tx, nil
}

func (ts *TxStore) GetTxs(txids []string) ([]*Tx, error) {
	txs := []*Tx{}
	var result error
	for _, txid := range txids {
		tx, err := ts.GetTx(txid)
		if err != nil {
			result = err
		}
		txs = append(txs, tx)
	}
	if result != nil {
		return nil, result
	}
	sortTx(txs)
	return txs, result
}

func sortTx(txs []*Tx) {
	sort.SliceStable(txs, func(i, j int) bool { return txs[i].Receivedtime > txs[j].Receivedtime })
}