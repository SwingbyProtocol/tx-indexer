package common

import (
	"encoding/json"
	"time"

	"github.com/syndtr/goleveldb/leveldb"
)

type Db struct {
	db        *leveldb.DB
	pruneTime time.Duration
}

func NewDB() *Db {
	return &Db{}
}

func (d *Db) Start(path string, pruneHours int64) error {
	db, err := leveldb.OpenFile(path, nil)
	if err != nil {
		return err
	}
	d.db = db
	d.pruneTime = time.Duration(pruneHours) * time.Hour
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
		if time.Unix(idx.Timestamp, 0).Add(d.pruneTime).Unix() < time.Now().Unix() {
			// Remove tx
			d.db.Delete([]byte(idx.ID), nil)
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
	// Check and Remove mempool tx for sender
	senderTxsRaw, _ := d.GetMempoolTxs(tx.From)
	senderTxs := []Transaction{}
	for _, mempoolTx := range senderTxsRaw {
		if mempoolTx.Serialize() == tx.Serialize() {
			continue
		}
		senderTxs = append(senderTxs, mempoolTx)
	}
	d.StoreMempoolTxs(tx.From, senderTxs)
	receiverTxsRaw, _ := d.GetMempoolTxs(tx.To)
	receiverTxs := []Transaction{}
	for _, mempoolTx := range receiverTxsRaw {
		if mempoolTx.Serialize() == tx.Serialize() {
			continue
		}
		receiverTxs = append(receiverTxs, mempoolTx)
	}
	d.StoreMempoolTxs(tx.To, receiverTxs)
	return nil
}

func (d *Db) GetMempoolTxs(addr string) ([]Transaction, error) {
	txs := []Transaction{}
	txJsons := []TxJSON{}
	value, err := d.db.Get([]byte("mempool_"+addr), nil)
	if err != nil {
		return txs, err
	}
	json.Unmarshal(value, &txJsons)
	for _, j := range txJsons {
		tx, _ := j.ToCommTx()
		txs = append(txs, tx)
	}
	return txs, nil
}

func (d *Db) AddMempoolTxs(addr string, tx Transaction) error {
	txs, _ := d.GetMempoolTxs(addr)
	isNew := true
	for _, loadTx := range txs {
		if loadTx.Serialize() == tx.Serialize() {
			isNew = false
		}
	}
	if isNew {
		txs = append(txs, tx)
	}
	err := d.StoreMempoolTxs(addr, txs)
	if err != nil {
		return err
	}
	return nil
}

func (d *Db) StoreMempoolTxs(addr string, txs []Transaction) error {
	data, err := json.Marshal(txs)
	if err != nil {
		return err
	}
	err = d.db.Put([]byte("mempool_"+addr), data, nil)
	if err != nil {
		return err
	}
	return nil
}

func (d *Db) GetMemoTxs(memo string) ([]string, error) {
	txkeys := []string{}
	value, err := d.db.Get([]byte("memotxs_"+memo), nil)
	if err != nil {
		return []string{}, err
	}
	json.Unmarshal(value, &txkeys)
	return txkeys, nil
}

func (d *Db) StoreMemoTxs(memo string, txkey string) error {
	txkeys, _ := d.GetMemoTxs(memo)
	txkeys = append(txkeys, txkey)
	data, err := json.Marshal(txkeys)
	if err != nil {
		return err
	}
	err = d.db.Put([]byte("memotxs_"+memo), data, nil)
	if err != nil {
		return err
	}
	return nil
}
