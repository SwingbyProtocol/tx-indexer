package blockchain

import (
	"errors"

	"github.com/btcsuite/btcd/wire"
	log "github.com/sirupsen/logrus"
)

type Blockchain struct {
	resolver   *Resolver
	index      map[int64]*Index
	txStore    *TxStore
	nowHeight  int64
	txChan     chan *wire.MsgTx
	blockChan  chan *wire.MsgBlock
	pushTxChan chan *Tx
}

func NewBlockchain(conf *BlockchainConfig) *Blockchain {
	bc := &Blockchain{
		resolver:   NewResolver(conf.TrustedNode),
		index:      make(map[int64]*Index),
		txStore:    NewTxStore(),
		txChan:     make(chan *wire.MsgTx),
		blockChan:  make(chan *wire.MsgBlock),
		pushTxChan: make(chan *Tx),
	}
	log.Info("Using trusted node ", conf.TrustedNode)
	block, err := bc.GetRemoteBlock()
	if err != nil {
		log.Fatal("latest block is not get")
	}
	bc.AddBlock(block)
	return bc
}

func (b *Blockchain) Start() {
	go func() {
		for {
			msg := <-b.txChan
			tx := MsgTxToTx(msg)
			// Check the input to remark spent tx
			for _, in := range tx.Vin {
				inTx, err := b.txStore.GetTx(in.Txid)
				if err != nil {
					//log.Debug(err)
					continue
				}
				targetOutput := inTx.Vout[in.Vout]
				targetOutput.Txs = append(targetOutput.Txs, tx.Txid)
				// check the sender of tx
				addrs := inTx.GetOutsAddrs()
				for _, addr := range addrs {
					b.index[b.nowHeight].UpdateTx(addr, inTx.Txid, true)
				}
			}
			// Check the outputs to remark spent tx
			addrs := tx.GetOutsAddrs()
			for _, addr := range addrs {
				b.index[b.nowHeight].UpdateTx(addr, tx.Txid, false)
			}

			// add tx
			b.txStore.AddTx(&tx)
			// push notification to ws handler
			b.pushTxChan <- &tx
		}
	}()

	go func() {
		for {
			msg := <-b.blockChan
			// add block
			block := MsgBlockToBlock(msg)
			log.Info(block.Hash)
			b.AddBlock(&block)
		}
	}()
}

func (b *Blockchain) GetIndexTxs(addr string, times int, spent bool) ([]*Tx, error) {
	if len(b.index) < times {
		return nil, errors.New("Getindextxs error")
	}
	for i := b.nowHeight; i > b.nowHeight-int64(times); i-- {
		txids := b.index[i].GetTxIDs(addr, spent)
		log.Info(txids)
	}
	return nil, nil
}

func (b *Blockchain) AddBlock(block *Block) {
	if b.nowHeight == block.Height {
		return
	}
	b.index[block.Height] = NewIndex()
	b.nowHeight = block.Height
}

func (b *Blockchain) GetRemoteBlock() (*Block, error) {
	info := ChainInfo{}
	err := b.resolver.GetRequest("/rest/chaininfo.json", &info)
	if err != nil {
		return nil, err
	}
	block := Block{}
	err = b.resolver.GetRequest("/rest/block/"+info.Bestblockhash+".json", &block)
	if err != nil {
		return nil, err
	}
	return &block, nil
}

func (b *Blockchain) GetTxScore() *TxStore {
	return b.txStore
}

func (b *Blockchain) TxChan() chan *wire.MsgTx {
	return b.txChan
}

func (b *Blockchain) BlockChan() chan *wire.MsgBlock {
	return b.blockChan
}

func (b *Blockchain) PushTxChan() chan *Tx {
	return b.pushTxChan
}
