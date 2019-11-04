package btc

import (
	"errors"
	"time"

	"github.com/SwingbyProtocol/tx-indexer/resolver"
	log "github.com/sirupsen/logrus"
)

type PoolTx struct {
	Height int64 `json:"height"`
	Time   int64 `json:"time"`
}

type Mempool struct {
	pool     map[string]bool
	tasks    []*Tx
	resolver *resolver.Resolver
	waitchan chan Tx
	iswork   bool
}

func NewMempool(uri string) *Mempool {
	mem := &Mempool{
		pool:     make(map[string]bool),
		resolver: resolver.NewResolver(uri),
		waitchan: make(chan Tx),
	}
	return mem
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
	go mem.doGetTxIDs(t)
	return
}

func (mem *Mempool) loadTxsIDs() error {
	res := make(map[string]PoolTx)
	err := mem.resolver.GetRequest("/rest/mempool/contents.json", &res)
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
			Receivedtime: tx.Time,
		}
		txs = append(txs, id)
		if mem.pool[id] == true {
			continue
		}
		mem.pool[id] = true
		mem.tasks = append(mem.tasks, &newTx)
	}
	log.Infof(" --- task_count -> %d --- ", len(mem.tasks))
	if mem.iswork == false {
		mem.iswork = true
		go mem.doGetTx()
	}
	mem.removePool(txs)
	return nil
}

func (mem *Mempool) doGetTx() {
	err := mem.getTx()
	if err != nil {
		mem.iswork = false
		return
	}
	time.Sleep(1 * time.Microsecond)
	go mem.doGetTx()
	return
}

func (mem *Mempool) removePool(txs []string) {
	if len(mem.pool) <= 1000 {
		return
	}
	for tx := range mem.pool {
		isMatch := false
		for _, txID := range txs {
			if tx == txID {
				isMatch = true
			}
		}
		if isMatch == false {
			delete(mem.pool, tx)
		}
	}
}

func (mem *Mempool) getTx() error {
	lock := GetMu()
	lock.Lock()
	if len(mem.tasks) == 0 {
		lock.Unlock()
		return errors.New("task zero")
	}
	tx := mem.tasks[0]
	mem.tasks = mem.tasks[1:]
	resolver := mem.resolver
	lock.Unlock()
	go func() {
		err := tx.AddTxData(resolver)
		if err != nil {
			if err.Error() == "404" {
				return
			}
			lock.Lock()
			mem.tasks = append(mem.tasks, tx)
			lock.Unlock()
			return
		}
		mem.waitchan <- *tx
	}()
	return nil
}

func (mem *Mempool) GetTaskCount() int {
	return len(mem.tasks)
}
