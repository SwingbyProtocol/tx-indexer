package blockchain

import (
	"encoding/json"
	"errors"
	"io/ioutil"
	"os"
	"sync"
)

const (
	Received = iota
	Send
	Both
)

type Index struct {
	mu *sync.RWMutex
	Kv map[string]*Store
}

type Store struct {
	Txs map[string]int
}

func NewIndex() *Index {
	index := &Index{
		mu: new(sync.RWMutex),
		Kv: make(map[string]*Store),
	}
	return index
}

func (in *Index) Update(addr string, txid string, state int) {
	in.mu.Lock()
	if in.Kv[addr] == nil {
		in.Kv[addr] = &Store{Txs: make(map[string]int)}
	}
	if in.Kv[addr].Txs[txid] == Send && state == Received {
		in.Kv[addr].Txs[txid] = Both
	} else {
		in.Kv[addr].Txs[txid] = state
	}
	in.mu.Unlock()
}

func (in *Index) Remove(addr string, txid string) error {
	in.mu.Lock()
	defer in.mu.Unlock()
	if in.Kv[addr] == nil {
		return errors.New("tx is not exit")
	}
	delete(in.Kv[addr].Txs, txid)
	return nil
}

func (in *Index) GetTxIDs(addr string, state int) []string {
	in.mu.RLock()
	defer in.mu.RUnlock()
	txids := []string{}
	if in.Kv[addr] == nil {
		return txids
	}
	for i, status := range in.Kv[addr].Txs {
		if status == state || status == Both {
			txids = append(txids, i)
		}
	}
	return txids
}

func (in *Index) Backup() error {
	in.mu.RLock()
	defer in.mu.RUnlock()
	str, err := json.Marshal(in.Kv)
	if err != nil {
		return err
	}
	f, err := os.Create("./data/index.backup")
	if err != nil {
		err = os.MkdirAll("./data", 0755)
		if err != nil {
			return err
		}
		return in.Backup()
	}
	_, err = f.Write(str)
	if err != nil {
		return err
	}
	return nil
}

func (in *Index) Load() error {
	data, err := ioutil.ReadFile("./data/index.backup")
	if err != nil {
		return err
	}
	err = json.Unmarshal(data, &in.Kv)
	if err != nil {
		return err
	}
	return nil
}
