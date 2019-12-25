package blockchain

import (
	"errors"
	"sync"
)

type Index struct {
	ranks map[string]int
	mu    *sync.RWMutex
	kv    map[string]*Store
}

type Store struct {
	txs map[string]bool
}

type Rank struct {
	addr  string
	count int
}

func NewIndex() *Index {
	index := &Index{
		mu: new(sync.RWMutex),
		kv: make(map[string]*Store),
	}
	return index
}

func (in *Index) UpdateTx(addr string, txid string, spent bool) {
	in.mu.Lock()
	if in.kv[addr] == nil {
		in.kv[addr] = &Store{txs: make(map[string]bool)}
	}
	in.kv[addr].txs[txid] = spent
	in.mu.Unlock()
}

func (in *Index) DeleteTx(addr string, txid string) error {
	in.mu.Lock()
	defer in.mu.Unlock()
	if in.kv[addr] == nil {
		return errors.New("tx is not exit")
	}
	delete(in.kv[addr].txs, txid)
	return nil
}

func (in *Index) GetTxIDs(addr string, spent bool) []string {
	txids := []string{}
	if in.kv[addr] == nil {
		return txids
	}
	for i, status := range in.kv[addr].txs {
		if status == spent {
			txids = append(txids, i)
		}
	}
	return txids
}

func insertionSort(obj []*Rank) {
	for j := 1; j < len(obj); j++ {
		key := obj[j]
		i := j - 1
		for i >= 0 && obj[i].count > key.count {
			obj[i+1] = obj[i]
			i = i - 1
		}
		obj[i+1] = key
	}
}
