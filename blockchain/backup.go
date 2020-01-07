package blockchain

import (
	"encoding/json"
	"io/ioutil"
	"os"
)

func (bc *Blockchain) Backup() error {
	err := bc.index.Backup()
	if err != nil {
		return err
	}
	err = bc.backupBlocks()
	if err != nil {
		return err
	}
	err = bc.backupMempool()
	if err != nil {
		return err
	}
	return nil
}

func (bc *Blockchain) Load() error {
	err := bc.index.Load()
	if err != nil {
		return err
	}
	err = bc.loadBlocks()
	if err != nil {
		return err
	}
	err = bc.loadMempool()
	if err != nil {
		return err
	}
	return nil
}

func (bc *Blockchain) backupBlocks() error {
	bc.mu.RLock()
	defer bc.mu.RUnlock()
	data, err := json.Marshal(bc.Blocks)
	if err != nil {
		return err
	}
	f, err := os.Create("./data/blocks.backup")
	if err != nil {
		err = os.MkdirAll("./data", 0755)
		if err != nil {
			return err
		}
		return bc.backupBlocks()
	}
	_, err = f.Write(data)
	if err != nil {
		return err
	}
	return nil
}

func (bc *Blockchain) loadBlocks() error {
	data, err := ioutil.ReadFile("./data/blocks.backup")
	if err != nil {
		return err
	}
	err = json.Unmarshal(data, &bc.Blocks)
	if err != nil {
		return err
	}
	return nil
}

func (bc *Blockchain) backupMempool() error {
	bc.mu.RLock()
	defer bc.mu.RUnlock()
	data, err := json.Marshal(bc.Mempool)
	if err != nil {
		return err
	}
	f, err := os.Create("./data/mempool.backup")
	if err != nil {
		err = os.MkdirAll("./data", 0755)
		if err != nil {
			return err
		}
		return bc.backupBlocks()
	}
	_, err = f.Write(data)
	if err != nil {
		return err
	}
	return nil
}

func (bc *Blockchain) loadMempool() error {
	data, err := ioutil.ReadFile("./data/mempool.backup")
	if err != nil {
		return err
	}
	err = json.Unmarshal(data, &bc.Mempool)
	if err != nil {
		return err
	}
	return nil
}
