package btc

import (
	"strconv"

	"github.com/SwingbyProtocol/tx-indexer/api"
	"github.com/SwingbyProtocol/tx-indexer/chains/btc/types"
)

type Rest struct {
	resolver *api.Resolver
}

func NewRest(uri string) *Rest {
	rest := &Rest{api.NewResolver(uri, 2)}
	return rest
}

func (r *Rest) GetBlockChainInfo() (*types.ChainInfo, error) {
	info := types.ChainInfo{}
	err := r.resolver.GetRequest("/rest/chaininfo.json", &info)
	if err != nil {
		return nil, err
	}
	return &info, err
}

func (r *Rest) GetBlockHash(blockNum int) (string, error) {
	bh := types.BlockHash{}
	err := r.resolver.GetRequest("/rest/blockhashbyheight/"+strconv.Itoa(blockNum)+".json", &bh)
	if err != nil {
		return "", err
	}
	return bh.Blockhash, nil
}

func (r *Rest) GetBlock(hash string) (*types.Block, error) {
	block := types.Block{}
	err := r.resolver.GetRequest("/rest/block/"+hash+".json", &block)
	if err != nil {
		return nil, err
	}
	return &block, nil
}
