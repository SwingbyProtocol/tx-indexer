package btc

import (
	"errors"
	"time"

	"github.com/SwingbyProtocol/sc-indexer/resolver"
	log "github.com/sirupsen/logrus"
)

type PoolTx struct {
	Height int64 `json:"height"`
	Time   int64 `json:"time"`
}

func NewMempool(uri string) *Mempool {
	mem := &Mempool{
		Pool:     make(map[string]bool),
		Resolver: resolver.NewResolver(uri),
		TxChan:   make(chan Tx),
	}
	return mem
}

type Mempool struct {
	Pool     map[string]bool
	Tasks    []*Tx
	Resolver *resolver.Resolver
	TxChan   chan Tx
	isWork   bool
}

func (mem *Mempool) StartSync(t time.Duration) {
	go mem.doGetTxIDs(t)
}

func (mem *Mempool) doGetTxIDs(t time.Duration) {
	err := mem.loadTxsIDs()
	if err != nil {
		log.Info(err)
	}
	time.Sleep(t)
	mem.doGetTxIDs(t)
}

func (mem *Mempool) loadTxsIDs() error {
	res := make(map[string]PoolTx)
	err := mem.Resolver.GetRequest("/rest/mempool/contents.json", &res)
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
		mem.Tasks = append(mem.Tasks, &newTx)
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
		}
	}
}

func (mem *Mempool) doGetTx() {
	err := mem.getTx()
	if err != nil {
		mem.isWork = false
		return
	}
	time.Sleep(1 * time.Microsecond)
	mem.doGetTx()
}

func (mem *Mempool) getTx() error {
	lock := GetMu()
	lock.Lock()
	if len(mem.Tasks) == 0 {
		lock.Unlock()
		return errors.New("task zero")
	}
	tx := mem.Tasks[0]
	mem.Tasks = mem.Tasks[1:]
	resolver := mem.Resolver
	lock.Unlock()
	go func() {
		err := tx.AddTxData(resolver)
		if err != nil {
			if err.Error() == "404" {
				return
			}
			lock.Lock()
			mem.Tasks = append(mem.Tasks, tx)
			lock.Unlock()
			return
		}
		mem.TxChan <- *tx
	}()
	return nil
}
