package btc

import (
	"encoding/json"

	log "github.com/sirupsen/logrus"
	"github.com/syndtr/goleveldb/leveldb"
)

func loadData(node *Node) error {
	iter := node.db.NewIterator(nil, nil)
	log.Info("loading leveldb....")
	for iter.Next() {
		key := iter.Key()
		value := iter.Value()
		if string(key[:4]) == "pool" {
			node.Pool[string(key[5:])] = true
		}
		if string(key[:5]) == "index" {
			node.Index[string(key[6:])] = string(value)
		}
		if string(key[:5]) == "spent" {
			node.Spent[string(key[6:])] = string(value)
		}
	}
	iter.Release()
	err := iter.Error()
	if err != nil {
		return err
	}
	return nil
}

func loadTxs(db *leveldb.DB, txIDs []string) []*Tx {
	txRes := []*Tx{}
	c := make(chan Tx)
	for _, txID := range txIDs {
		go loadTx(db, "tx_"+txID, c)
	}
	for i := 0; i < len(txIDs); i++ {
		tx := <-c
		txRes = append(txRes, &tx)
	}
	return txRes
}
func loadTx(db *leveldb.DB, key string, c chan Tx) {
	tx := Tx{}
	value, err := db.Get([]byte(key), nil)
	if err != nil {
		c <- tx
		return
	}
	json.Unmarshal(value, &tx)
	c <- tx
}

func storeIndex(db *leveldb.DB, addr string, data string) error {
	err := db.Put([]byte("index_"+addr), []byte(data), nil)
	if err != nil {
		return err
	}
	return nil
}

func storePool(db *leveldb.DB, key string) error {
	s, err := json.Marshal(true)
	if err != nil {
		return err
	}
	err = db.Put([]byte("pool_"+key), []byte(s), nil)
	if err != nil {
		return err
	}
	return nil
}

func storeSpent(db *leveldb.DB, key string, data string) error {
	err := db.Put([]byte("spent_"+key), []byte(data), nil)
	if err != nil {
		return err
	}
	return nil
}

func storeTx(db *leveldb.DB, key string, tx *Tx) error {
	t, err := json.Marshal(tx)
	if err != nil {
		return err
	}
	err = db.Put([]byte("tx_"+key), []byte(t), nil)
	if err != nil {
		return err
	}
	return nil
}

func removePool(db *leveldb.DB, txID string) error {
	err := db.Delete([]byte("pool_"+txID), nil)
	if err != nil {
		return err
	}
	return nil
}
