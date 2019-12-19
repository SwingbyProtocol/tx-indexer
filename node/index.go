package node

/*
type Index struct {
	lists   []*IndexScore
	counter map[string]int
	stamps  map[string][]*Stamp
}

type Stamp struct {
	Txid string
	Time int64
	Vout []*Link
}

type IndexScore struct {
	Address string
	Time    int64
	Txid    string
}

type Link struct {
	Txs     []string
	Address string
}

func NewIndex() *Index {
	index := &Index{
		counter: make(map[string]int),
		stamps:  make(map[string][]*Stamp),
	}
	return index
}

func (i *Index) AddIn(tx *Tx) {
	stamp := &Stamp{tx.Txid, tx.Receivedtime, nil}
	for _, vout := range tx.Vout {
		if len(vout.Scriptpubkey.Addresses) != 1 {
			continue
		}
		addr := vout.Scriptpubkey.Addresses[0]
		stamp.Vout = append(stamp.Vout, &Link{Address: addr})
	}
	addresses := tx.GetOutputsAddresses()
	lock := GetMu()
	for _, addr := range addresses {
		lock.Lock()
		i.stamps[addr] = append(i.stamps[addr], stamp)
		// Insertion Sort
		sortStamp(i.stamps[addr])
		i.UpdateScore(addr, tx.Receivedtime, tx.Txid)
		// Insertion Sort
		sortScores(i.lists)
		lock.Unlock()
	}
}


func (i *Index) AddVouts(addr string, storage *Storage) error {
	index := i.GetStamps(addr)
	if index == nil {
		return errors.New("index is not exist")
	}
	for _, in := range i.stamps[addr] {
		for i, out := range in.Vout {
			if len(out.Txs) != 0 {
				continue
			}
			if addr != out.Address {
				continue
			}
			key := in.Txid + "_" + strconv.Itoa(i)
			spents, err := storage.GetSpents(key)
			if err != nil {
				continue
			}
			out.Txs = spents
		}
	}
	return nil
}

func (i *Index) RemoveIndexWithTxBefore(blockchian *BlockChain, storage *Storage) error {
	time, err := blockchian.GetPruneBlockTime()
	if err != nil {
		return err
	}
	i.removeIndexWIthAllSpentTxBefore(time, storage)
	return nil
}

func (i *Index) GetSpents(addr string, storage *Storage) ([]*Tx, error) {
	res := []*Tx{}
	ins := i.GetStamps(addr)
	if ins == nil {
		return nil, errors.New("index is not exist")
	}
	for _, in := range ins {
		for _, link := range in.Vout {
			for _, txID := range link.Txs {
				tx := storage.txs[txID]
				res = append(res, tx)
			}
		}
	}
	return res, nil
}

func (i *Index) GetIns(addr string, storage *Storage) ([]*Tx, error) {
	res := []*Tx{}
	ins := i.GetStamps(addr)
	if ins == nil {
		return nil, errors.New("index is not exist")
	}
	for _, in := range ins {
		tx, err := storage.GetTx(in.Txid)
		if err != nil {
			continue
		}
		res = append(res, tx)
	}
	return res, nil
}

func (i *Index) GetStamps(addr string) []*Stamp {
	lock := GetMu()
	lock.RLock()
	stamps := i.stamps[addr]
	lock.RUnlock()
	return stamps
}

func (i *Index) UpdateScore(addr string, time int64, txid string) {
	newScore := &IndexScore{Address: addr, Time: time, Txid: txid}
	i.lists = append(i.lists, newScore)
	i.counter[addr]++
}

func (i *Index) removeIndexWIthAllSpentTxBefore(prunetime int64, storage *Storage) {
	indexTotal := 0
	txTotal := 0
	spentTotal := 0
	for {
		score, err := i.checkTimeWithPop(prunetime)
		if err != nil {
			log.Infof(" Removed Index -> %7d Spent -> %7d Tx -> %7d", txTotal, indexTotal, spentTotal)
			return
		}
		count := i.removeCountWithBeforeNum(score.Address)
		if count != 1 {
			continue
		}
		// count == 1
		ins, err := i.GetIns(score.Address, storage)
		for _, in := range ins {
			isAllSpent := in.CheckAllSpent(storage)
			if isAllSpent == false {
				continue
			}
			//log.Info("all spent ", in.Txid, " ", count)
			for i := range in.Vout {
				key := in.Txid + "_" + strconv.Itoa(i)
				storage.DeleteSpent(key)
				spentTotal++
			}
			storage.DeleteTx(in.Txid)
			txTotal++
		}
		indexTotal++
		i.removeIndex(score.Address)
	}
}

func (i *Index) removeCountWithBeforeNum(addr string) int {
	lock := GetMu()
	lock.Lock()
	now := i.counter[addr]
	if now == 0 {
		lock.Unlock()
		return 0
	}
	i.counter[addr]--
	lock.Unlock()
	return now
}

func (i *Index) removeIndex(addr string) {
	lock := GetMu()
	lock.Lock()
	delete(i.stamps, addr)
	lock.Unlock()
}

func (i *Index) removeStamp(addr string, txid string) {
	stamps := i.GetStamps(addr)
	if len(stamps) == 0 {
		return
	}
	for index, stamp := range stamps {
		if stamp.Txid == txid {
			lock := GetMu()
			lock.Lock()
			i.stamps[addr] = append(i.stamps[addr][:index], i.stamps[addr][index+1:]...)
			lock.Unlock()
			log.Info("remove tx from ", addr, " ", txid)
		}
	}
}

func (i *Index) checkTimeWithPop(prunetime int64) (*IndexScore, error) {
	if len(i.lists) == 0 {
		return nil, errors.New("list is zero")
	}
	data := i.lists[0]
	if data.Time > prunetime {
		return nil, errors.New("time is wrong")
	}
	i.lists = i.lists[1:]
	return data, nil
}

func sortStamp(stamps []*Stamp) {
	sort.SliceStable(stamps, func(i, j int) bool { return stamps[i].Time < stamps[j].Time })
}

func sortScores(scores []*IndexScore) {
	sort.SliceStable(scores, func(i, j int) bool { return scores[i].Time < scores[j].Time })
}

*/
