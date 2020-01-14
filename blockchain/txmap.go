package blockchain

import (
	"encoding/json"
	"errors"
	"io/ioutil"
	"os"
	"strconv"
	"sync"

	"github.com/SwingbyProtocol/tx-indexer/api"
	"github.com/SwingbyProtocol/tx-indexer/types"
	log "github.com/sirupsen/logrus"
)

type TxMap struct {
	mu       *sync.RWMutex
	resolver *api.Resolver
	store    map[string]string
}

func NewTxMap(finalizer string) *TxMap {
	txMap := &TxMap{
		mu:       new(sync.RWMutex),
		resolver: api.NewResolver(finalizer, 200),
		store:    make(map[string]string),
	}
	return txMap
}

func (t *TxMap) GetHash(txid string) string {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.store[txid]
}

func (t *TxMap) Start() error {
	info := types.ChainInfo{}
	err := t.resolver.GetRequest("/rest/chaininfo.json", &info)
	if err != nil {
		log.Fatal(err)
	}
	heightChan := make(chan int64, 1300)
	done := make(chan bool)
	limit := 0
	go func() {
		for {
			height := <-heightChan
			wg := new(sync.WaitGroup)
			wg.Add(1)
			go func() {
				err := t.AddTx(height, wg)
				if err != nil {
					done <- true
					return
				}
			}()
			if limit >= 100 {
				wg.Wait()
				limit = 0
			}
			limit++
		}
	}()
	go func() {
		for {
			<-done
			log.Info("block tx map loaded")
			break
		}
	}()
	for i := info.Blocks; i >= 0; i-- {
		heightChan <- i
	}
	return nil
}

func (t *TxMap) AddTx(height int64, wg *sync.WaitGroup) error {
	defer wg.Done()
	bh := HeighHash{}
	err := t.resolver.GetRequest("/rest/blockhashbyheight/"+strconv.Itoa(int(height))+".json", &bh)
	if err != nil {
		return err
	}
	block := types.BlockTxs{}
	err = t.resolver.GetRequest("/rest/block/notxdetails/"+bh.BlockHash+".json", &block)
	if err != nil {
		return err
	}
	if block.Height == 0 {
		return errors.New("block is zero")
	}
	for _, txid := range block.Txs {
		t.mu.Lock()
		t.store[txid] = block.Hash
		t.mu.Unlock()
		log.Info("stored ", txid, " ", block.Height)
	}
	return nil
}

func (t *TxMap) Backup() error {
	t.mu.RLock()
	defer t.mu.RUnlock()
	str, err := json.Marshal(t.store)
	if err != nil {
		return err
	}
	f, err := os.Create("./data/txmap.backup")
	if err != nil {
		err = os.MkdirAll("./data", 0755)
		if err != nil {
			return err
		}
		return t.Backup()
	}
	_, err = f.Write(str)
	if err != nil {
		return err
	}
	return nil
}

func (t *TxMap) Load() error {
	data, err := ioutil.ReadFile("./data/index.backup")
	if err != nil {
		return err
	}
	err = json.Unmarshal(data, &t.store)
	if err != nil {
		return err
	}
	return nil
}
