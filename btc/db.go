package btc

import (
	"encoding/json"

	log "github.com/sirupsen/logrus"
	"github.com/syndtr/goleveldb/leveldb"
)

type Database struct {
	db *leveldb.DB
}

func NewDB(path string) *Database {
	db, err := leveldb.OpenFile("./db", nil)
	if err != nil {
		log.Fatal(err)
	}
	newDB := &Database{
		db: db,
	}
	return newDB
}

func (d *Database) LoadTxs(txIDs []string) []*Tx {
	txRes := []*Tx{}
	c := make(chan Tx)
	for _, txID := range txIDs {
		go d.LoadTx(txID, c)
	}
	for i := 0; i < len(txIDs); i++ {
		tx := <-c
		txRes = append(txRes, &tx)
	}
	return txRes
}
func (d *Database) LoadTx(txID string, c chan Tx) {
	tx := Tx{}
	value, err := d.db.Get([]byte("tx_"+txID), nil)
	if err != nil {
		tx.Txid = txID
		c <- tx
		return
	}
	json.Unmarshal(value, &tx)
	c <- tx
}

func (d *Database) LoadIndex(addr string) (*Index, error) {
	index := Index{}
	value, err := d.db.Get([]byte("index_"+addr), nil)
	if err != nil {
		return nil, err
	}
	json.Unmarshal(value, &index)
	return &index, nil
}

func (d *Database) storeIndex(addr string, index *Index) error {
	bytes, err := json.Marshal(index)
	if err != nil {
		return err
	}
	err = d.db.Put([]byte("index_"+addr), bytes, nil)
	if err != nil {
		return err
	}
	return nil
}

func (d *Database) storePool(key string) error {
	bytes, err := json.Marshal(true)
	if err != nil {
		return err
	}
	err = d.db.Put([]byte("pool_"+key), bytes, nil)
	if err != nil {
		return err
	}
	return nil
}

func (d *Database) loadSpent(key string) ([]string, error) {
	txs := []string{}
	value, err := d.db.Get([]byte("spent_"+key), nil)
	if err != nil {
		return nil, err
	}
	json.Unmarshal(value, &txs)
	return txs, nil
}

func (d *Database) storeSpent(key string, data []string) error {
	bytes, err := json.Marshal(data)
	if err != nil {
		return err
	}
	err = d.db.Put([]byte("spent_"+key), bytes, nil)
	if err != nil {
		return err
	}
	return nil
}

func (d *Database) storeTx(key string, tx *Tx) error {
	t, err := json.Marshal(tx)
	if err != nil {
		return err
	}
	err = d.db.Put([]byte("tx_"+key), []byte(t), nil)
	if err != nil {
		return err
	}
	return nil
}

func (d *Database) removePool(txID string) error {
	err := d.db.Delete([]byte("pool_"+txID), nil)
	if err != nil {
		return err
	}
	return nil
}
