package blockchain

import (
	"errors"
	"sync"
)

const (
	Received = iota
	Send
	Both
)

type Index struct {
	mu *sync.RWMutex
	kv map[string]*Store
}

type Store struct {
	txs map[string]int
}

func NewIndex() *Index {
	index := &Index{
		mu: new(sync.RWMutex),
		kv: make(map[string]*Store),
	}
	return index
}

func (in *Index) Update(addr string, txid string, state int) {
	in.mu.Lock()
	if in.kv[addr] == nil {
		in.kv[addr] = &Store{txs: make(map[string]int)}
	}
	if in.kv[addr].txs[txid] == Send && state == Received {
		in.kv[addr].txs[txid] = Both
	} else {
		in.kv[addr].txs[txid] = state
	}
	in.mu.Unlock()
}

func (in *Index) Remove(addr string, txid string) error {
	in.mu.Lock()
	defer in.mu.Unlock()
	if in.kv[addr] == nil {
		return errors.New("tx is not exit")
	}
	delete(in.kv[addr].txs, txid)
	return nil
}

func (in *Index) GetTxIDs(addr string, state int) []string {
	in.mu.RLock()
	defer in.mu.RUnlock()
	txids := []string{}
	if in.kv[addr] == nil {
		return txids
	}
	for i, status := range in.kv[addr].txs {
		if status == state || status == Both {
			txids = append(txids, i)
		}
	}
	return txids
}
