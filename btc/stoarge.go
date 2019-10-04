package btc

import (
	"errors"
	"strconv"
)

type Storage struct {
	txs   map[string]*Tx
	spent map[string][]string
}

func NewStorage() *Storage {
	return &Storage{
		txs:   make(map[string]*Tx),
		spent: make(map[string][]string),
	}
}

func (s *Storage) GetTx(txid string) (*Tx, error) {
	lock := GetMu()
	lock.RLock()
	tx, ok := s.txs[txid]
	lock.RUnlock()
	if ok == false {
		return nil, errors.New("tx is not exist")
	}
	return tx, nil
}

func (s *Storage) GetSpents(key string) ([]string, error) {
	lock := GetMu()
	lock.RLock()
	spents, ok := s.spent[key]
	lock.RUnlock()
	if ok == false {
		return nil, errors.New("spent is not exist")
	}
	if len(spents) == 0 {
		return nil, errors.New("spent tx count is zero")
	}
	return spents, nil
}

func (s *Storage) AddSpent(key string, txid string) error {
	lock := GetMu()
	lock.RLock()
	spents, _ := s.spent[key]
	lock.RUnlock()
	if checkExist(key, spents) == true {
		return errors.New("already exist")
	}
	lock.Lock()
	s.spent[key] = append(s.spent[key], txid)
	lock.Unlock()
	return nil
}

func (s *Storage) DeleteSpent(key string) {
	lock := GetMu()
	lock.Lock()
	delete(s.spent, key)
	lock.Unlock()
}

func (s *Storage) AddTx(tx *Tx) error {
	for _, vin := range tx.Vin {
		key := vin.Txid + "_" + strconv.Itoa(vin.Vout)
		err := s.AddSpent(key, tx.Txid)
		if err != nil {
			return err
		}
	}
	for _, vout := range tx.Vout {
		_, ok := vout.Value.(float64)
		if ok == true {
			vout.Value = strconv.FormatFloat(vout.Value.(float64), 'f', -1, 64)
		}
		vout.Txs = []string{}
	}
	s.UpdateTx(tx)
	return nil
}

func (s *Storage) DeleteTx(txid string) {
	lock := GetMu()
	lock.Lock()
	delete(s.txs, txid)
	lock.Unlock()
}

func (s *Storage) UpdateTx(tx *Tx) {
	lock := GetMu()
	lock.Lock()
	s.txs[tx.Txid] = tx
	lock.Unlock()
}
