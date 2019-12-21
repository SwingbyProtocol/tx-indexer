package blockchain

type Tx struct {
	Txid         string  `json:"txid"`
	Hash         string  `json:"hash"`
	Confirms     int64   `json:"confirms"`
	Receivedtime int64   `json:"receivedtime"`
	MinedTime    int64   `json:"minedtime"`
	Mediantime   int64   `json:"mediantime"`
	Version      int     `json:"version"`
	Weight       int     `json:"weight"`
	Locktime     int     `json:"locktime"`
	Vin          []*Vin  `json:"vin"`
	Vout         []*Vout `json:"vout"`
	//Hex      string  `json:"hex"`
}

type Vin struct {
	Txid     string `json:"txid"`
	Vout     int    `json:"vout"`
	Sequence int64  `json:"sequence"`
}

type Vout struct {
	Value        interface{}   `json:"value"`
	Spent        bool          `json:"spent"`
	Txs          []string      `json:"txs"`
	N            int           `json:"n"`
	Scriptpubkey *ScriptPubkey `json:"scriptPubkey"`
}

type ScriptPubkey struct {
	Asm       string   `json:"asm"`
	Hex       string   `json:"hex"`
	Reqsigs   int      `json:"reqSigs"`
	Keytype   string   `json:"type"`
	Addresses []string `json:"addresses"`
}

func (tx *Tx) GetHash() string {
	return tx.Hash
}

func (tx *Tx) AddBlockData(height int64, time int64, medianTime int64) *Tx {
	tx.Confirms = height
	tx.MinedTime = time
	tx.Mediantime = medianTime
	return tx
}

/*

func (tx *Tx) EnableTxSpent(addr string, storage *Storage) {
	for i, vout := range tx.Vout {
		key := tx.Txid + "_" + strconv.Itoa(i)
		spents, err := storage.GetSpents(key)
		if err != nil {
			continue
		}
		if len(vout.Scriptpubkey.Addresses) != 1 {
			continue
		}
		address := vout.Scriptpubkey.Addresses[0]
		if addr != address {
			continue
		}
		vout.Spent = true
		vout.Txs = spents
	}
}

func (tx *Tx) CheckAllSpent(storage *Storage) bool {
	isAllSpent := true
	for i, vout := range tx.Vout {
		if len(vout.Scriptpubkey.Addresses) != 1 {
			continue
		}
		key := tx.Txid + "_" + strconv.Itoa(i)
		_, err := storage.GetSpents(key)
		if err != nil {
			isAllSpent = false
			continue
		}
	}
	return isAllSpent
}

func (tx *Tx) GetOutputsAddresses() []string {
	addresses := []string{}
	for _, vout := range tx.Vout {
		if len(vout.Scriptpubkey.Addresses) != 1 {
			log.Debug("debug : len(vout.ScriptPubkey.Addresses) != 1")
			continue
		}
		addr := vout.Scriptpubkey.Addresses[0]
		if checkExist(addr, addresses) == true {
			continue
		}
		addresses = append(addresses, addr)
	}
	return addresses
}

*/
