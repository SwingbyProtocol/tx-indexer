package btc

import (
	"errors"
	"sort"
	"strconv"

	log "github.com/sirupsen/logrus"
)

type Storage struct {
	txs      map[string]*Tx
	txscores []*TxScore
	spent    map[string][]string
}

type TxScore struct {
	Txid string
	Time int64
}

func NewStorage() *Storage {
	return &Storage{
		txs:   make(map[string]*Tx),
		spent: make(map[string][]string),
	}
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
	s.addNewScore(tx.Txid, tx.Receivedtime)
	s.sortScores()
	s.UpdateTx(tx)
	return nil
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

func (s *Storage) UpdateTx(tx *Tx) {
	lock := GetMu()
	lock.Lock()
	s.txs[tx.Txid] = tx
	lock.Unlock()
}

func (s *Storage) DeleteTx(txid string) {
	lock := GetMu()
	lock.Lock()
	delete(s.txs, txid)
	lock.Unlock()
}

func (s *Storage) AddSpent(key string, txid string) error {
	lock := GetMu()
	lock.RLock()
	spents, _ := s.spent[key]
	lock.RUnlock()
	if checkExist(txid, spents) == true {
		return errors.New("already exist")
	}
	lock.Lock()
	s.spent[key] = append(s.spent[key], txid)
	lock.Unlock()
	return nil
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

func (s *Storage) DeleteSpent(key string) {
	lock := GetMu()
	lock.Lock()
	delete(s.spent, key)
	lock.Unlock()
}

func (s *Storage) RemoveTxsWIthPruneTime(prunetime int64, index *Index) {
	txTotal := 0
	for {
		txscore, err := s.checkTimeWithPop(prunetime)
		if err != nil {
			log.Infof(" Removed txs -> %7d", txTotal)
			return
		}
		tx, err := s.GetTx(txscore.Txid)
		if err != nil {
			log.Info(err)
			continue
		}
		addresses := tx.GetOutputsAddresses()
		log.Info(addresses)

		for _, addr := range addresses {
			index.removeStamp(addr, tx.Txid)
		}
		delete(s.txs, txscore.Txid)
		txTotal++
	}
}

func (s *Storage) checkTimeWithPop(prunetime int64) (*TxScore, error) {
	if len(s.txscores) == 0 {
		return nil, errors.New("list is zero")
	}
	data := s.txscores[0]
	if data.Time > prunetime {
		return nil, errors.New("time is wrong")
	}
	s.txscores = s.txscores[1:]
	return data, nil
}

func (s *Storage) addNewScore(txid string, time int64) {
	newScore := &TxScore{Txid: txid, Time: time}
	s.txscores = append(s.txscores, newScore)
}

func (s *Storage) sortScores() {
	sort.SliceStable(s.txscores, func(i, j int) bool { return s.txscores[i].Time < s.txscores[j].Time })
}
