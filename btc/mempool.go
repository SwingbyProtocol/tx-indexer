package btc

import (
	"errors"
	"time"

	"github.com/SwingbyProtocol/sc-indexer/resolver"
	log "github.com/sirupsen/logrus"
)

type Mempool struct {
	Pool     map[string]bool
	URI      string
	Tasks    []Tx
	Resolver *resolver.Resolver
	TxChan   chan Tx
	isWork   bool
}

type PoolTx struct {
	Height int64 `json:"height"`
	Time   int64 `json:"time"`
}

func NewMempool(uri string) *Mempool {
	mem := &Mempool{
		Pool:     make(map[string]bool),
		Resolver: resolver.NewResolver(),
		URI:      uri,
		TxChan:   make(chan Tx),
	}
	return mem
}

func (mem *Mempool) StartSync(t time.Duration) {
	go mem.doGetTxIDs(t)
}

func (mem *Mempool) doGetTxIDs(t time.Duration) {
	err := mem.LoadTxsIDs()
	if err != nil {
		log.Info(err)
	}
	time.Sleep(t)
	mem.doGetTxIDs(t)
}

func (mem *Mempool) LoadTxsIDs() error {
	res := make(map[string]PoolTx)
	err := mem.Resolver.GetRequest(mem.URI, "/rest/mempool/contents.json", &res)
	if err != nil {
		return err
	}
	height := int64(0)
	for _, tx := range res {
		if tx.Height > height {
			height = tx.Height
		}
	}
	txs := []string{}
	for id, tx := range res {
		if tx.Height != height {
			continue
		}
		newTx := Tx{
			Txid:         id,
			ReceivedTime: tx.Time,
		}
		txs = append(txs, id)
		if mem.Pool[id] == true {
			continue
		}
		mem.Pool[id] = true
		mem.Tasks = append(mem.Tasks, newTx)
	}
	log.Infof("mempool task -> %7d", len(mem.Tasks))
	if mem.isWork == false {
		mem.isWork = true
		go mem.doGetTx()
	}
	mem.removePool(txs)
	return nil
}

func (mem *Mempool) removePool(txs []string) {
	if len(mem.Pool) <= 1000 {
		return
	}
	for tx := range mem.Pool {
		isMatch := false
		for _, txID := range txs {
			if tx == txID {
				isMatch = true
			}
		}
		if isMatch == false {
			delete(mem.Pool, tx)
			//go removePool(node.db, key)
		}
	}
}

func (mem *Mempool) doGetTx() {
	err := mem.getTx()
	if err != nil {
		if err.Error() == "task zero" {
			mem.isWork = false
			return
		}
	}
	time.Sleep(10 * time.Microsecond)
	mem.doGetTx()
}

func (mem *Mempool) getTx() error {
	if len(mem.Tasks) == 0 {
		return errors.New("task zero")
	}
	tx := mem.Tasks[0]
	mem.Tasks = mem.Tasks[1:]
	go func() {
		GetMu().Lock()
		err := tx.getTxData(mem.Resolver, mem.URI)
		if err != nil {
			mem.Tasks = append(mem.Tasks, tx)
			GetMu().Unlock()
			return
		}
		GetMu().Unlock()
		mem.TxChan <- tx
	}()
	return nil
}
