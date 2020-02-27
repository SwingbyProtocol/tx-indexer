package blockchain

import (
	"encoding/json"

	"github.com/syndtr/goleveldb/leveldb"
)

type Index struct {
	db *leveldb.DB
}

type Store struct {
	Txs map[string]int
}

func NewIndex() *Index {
	index := &Index{}
	return index
}

func (in *Index) Open() error {
	db, err := leveldb.OpenFile("./data/leveldb", nil)
	if err != nil {
		return err
	}
	in.db = db
	return nil
}

func (in *Index) Get(addr string, prefix string) ([]string, error) {
	data := []string{}
	base, err := in.db.Get([]byte(prefix+"_"+addr), nil)
	if err != nil {
		return nil, err
	}
	err = json.Unmarshal(base, &data)
	if err != nil {
		return nil, err
	}
	return data, nil
}

func (in *Index) Add(addr string, txid string, prefix string) error {
	data := []string{}
	base, err := in.db.Get([]byte(prefix+"_"+addr), nil)
	if err != nil {
		return err
	}
	err = json.Unmarshal(base, &data)
	if err != nil {
		return err
	}
	data = append(data, txid)
	bytes, err := json.Marshal(data)
	if err != nil {
		return err
	}
	err = in.db.Put([]byte(prefix+"_"+addr), bytes, nil)
	if err != nil {
		return err
	}
	return nil
}

func (in *Index) Remove(addr string, prefix string) error {
	err := in.db.Delete([]byte(prefix+"_"+addr), nil)
	if err != nil {
		return err
	}
	return nil
}
