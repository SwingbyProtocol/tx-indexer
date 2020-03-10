package types

type Block struct {
	Hash              string `json:"hash"`
	Confirmations     int64  `json:"confirmations"`
	Height            int64  `json:"height"`
	Ntx               int64  `json:"nTx"`
	Txs               []*Tx  `json:"tx"`
	Time              int64  `json:"time"`
	Mediantime        int64  `json:"mediantime"`
	Previousblockhash string `json:"previousblockhash"`
}

type HeightHash struct {
	BlockHash string `json:"blockhash"`
}

type BlockTxs struct {
	Hash   string   `json:"hash"`
	Height int64    `json:"height"`
	Txs    []string `json:"tx"`
}

func (block *Block) GetTxIDs() []string {
	ids := []string{}
	for _, tx := range block.Txs {
		ids = append(ids, tx.Txid)
	}
	return ids
}
