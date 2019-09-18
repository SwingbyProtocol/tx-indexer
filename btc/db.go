package btc

import (
	"encoding/json"

	log "github.com/sirupsen/logrus"
)

func (b *BTCNode) loadData() error {
	iter := b.db.NewIterator(nil, nil)
	log.Info("loading leveldb....")
	for iter.Next() {
		key := iter.Key()
		value := iter.Value()
		if string(key[:4]) == "pool" {
			b.Pool[string(key[5:])] = true
		}
		if string(key[:5]) == "index" {
			b.Index[string(key[6:])] = string(value)
		}
		if string(key[:5]) == "spent" {
			b.Spent[string(key[6:])] = string(value)
		}
	}
	iter.Release()
	err := iter.Error()
	if err != nil {
		return err
	}
	return nil
}

func (b *BTCNode) storeIndex(addr string, data string) error {
	err := b.db.Put([]byte("index_"+addr), []byte(data), nil)
	if err != nil {
		return err
	}
	return nil
}

func (b *BTCNode) storePool(key string) error {
	s, err := json.Marshal(true)
	if err != nil {
		return err
	}
	err = b.db.Put([]byte("pool_"+key), []byte(s), nil)
	if err != nil {
		return err
	}
	return nil
}

func (b *BTCNode) storeSpent(key string, data string) error {
	err := b.db.Put([]byte("spent_"+key), []byte(data), nil)
	if err != nil {
		return err
	}
	return nil
}

func (b *BTCNode) storeTx(key string, tx *Tx) error {
	t, err := json.Marshal(tx)
	if err != nil {
		return err
	}
	err = b.db.Put([]byte("tx_"+key), []byte(t), nil)
	if err != nil {
		return err
	}
	return nil
}

func (b *BTCNode) removePool(txID string) error {
	err := b.db.Delete([]byte("pool_"+txID), nil)
	if err != nil {
		return err
	}
	return nil
}
