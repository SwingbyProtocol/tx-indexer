package proxy

import (
	"sync"
	"time"

	"github.com/SwingbyProtocol/tx-indexer/api"
	"github.com/SwingbyProtocol/tx-indexer/types"
)

type Proxy struct {
	mu    *sync.RWMutex
	nodes map[string]bool
}

func NewProxy() *Proxy {
	proxy := &Proxy{
		mu:    new(sync.RWMutex),
		nodes: make(map[string]bool),
	}
	return proxy
}

func (p *Proxy) Add(addr string) {
	p.mu.Lock()
	p.nodes[addr] = true
	p.mu.Unlock()
}

func (p *Proxy) ScanAll() {
	p.mu.RLock()
	nodes := p.nodes
	p.mu.RUnlock()
	wg := new(sync.WaitGroup)
	for addr := range nodes {
		wg.Add(1)
		go func(addr string) {
			resolver := api.NewResolver("http://"+addr, 100)
			resolver.SetTimeout(3 * time.Second)
			info := types.ChainInfo{}
			err := resolver.GetRequest("/rest/chaininfo.json", &info)
			if err != nil {
				p.mu.Lock()
				delete(p.nodes, addr)
				p.mu.Unlock()
				wg.Done()
				return
			}
			p.mu.Lock()
			p.nodes[addr] = true
			p.mu.Unlock()
			wg.Done()
		}(addr)
	}
	wg.Wait()
}
