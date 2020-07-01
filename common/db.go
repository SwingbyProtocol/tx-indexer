package common

import (
	"encoding/json"
	"time"

	"github.com/syndtr/goleveldb/leveldb"
)

var pruneTime = 1 * time.Hour

type Db struct {
	db *leveldb.DB
}

func NewDB() *Db {
	return &Db{}
}

func (d *Db) Start(path string) error {
	db, err := leveldb.OpenFile(path, nil)
	if err != nil {
		return err
	}
	d.db = db
	return nil
}

func (d *Db) GetIdxs(address string, isReceived bool) ([]Index, error) {
	idxs := []Index{}
	way := "from_"
	if isReceived {
		way = "to_"
	}
	value, err := d.db.Get([]byte(way+address), nil)
	if err != nil {
		return idxs, err
	}
	json.Unmarshal(value, &idxs)
	return idxs, nil
}

func (d *Db) StoreIdx(id string, tx *Transaction, isReceived bool) error {
	newIdx := []Index{}
	way := "from_"
	addr := tx.From
	if isReceived {
		way = "to_"
		addr = tx.To
	}
	idxs, _ := d.GetIdxs(addr, isReceived)
	for _, idx := range idxs {
		// Prune logic
		if time.Unix(idx.Timestamp, 0).Add(pruneTime).Unix() < time.Now().Unix() {
			continue
		}
		newIdx = append(newIdx, idx)
	}
	new := Index{id, tx.Height, tx.Timestamp.Unix()}
	newIdx = append(newIdx, new)
	data, err := json.Marshal(newIdx)
	if err != nil {
		return err
	}
	err = d.db.Put([]byte(way+addr), data, nil)
	if err != nil {
		return err
	}
	return nil
}

func (d *Db) GetTx(key string) (*Transaction, error) {
	txj := TxJSON{}
	value, err := d.db.Get([]byte(key), nil)
	if err != nil {
		return nil, err
	}
	json.Unmarshal(value, &txj)
	tx, err := txj.ToCommTx()
	if err != nil {
		return nil, err
	}
	return &tx, nil
}

func (d *Db) StoreTx(key string, tx *Transaction) error {
	data, err := tx.MarshalJSON()
	if err != nil {
		return err
	}
	err = d.db.Put([]byte(key), data, nil)
	if err != nil {
		return err
	}
	return nil
}

func (d *Db) GetSelfTxIds() ([]string, error) {
	txkeys := []string{}
	value, err := d.db.Get([]byte("selftxs"), nil)
	if err != nil {
		return []string{}, err
	}
	json.Unmarshal(value, &txkeys)
	return txkeys, nil
}

func (d *Db) StoreSelfTxIds(txid string) error {
	txkeys, _ := d.GetSelfTxIds()
	data, err := json.Marshal(txkeys)
	if err != nil {
		return err
	}
	err = d.db.Put([]byte("selftxs"), data, nil)
	if err != nil {
		return err
	}
	return nil
}
