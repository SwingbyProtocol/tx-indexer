package common

type Index struct {
	ID        string
	Height    int64
	Timestamp int64
}

type Idxs []Index

func (idxs Idxs) GetRangeTxs(fromNum int, toNum int) []Index {
	newIdxs := []Index{}
	for _, idx := range idxs {
		if int64(fromNum) > idx.Height {
			continue
		}
		if int64(toNum) < idx.Height {
			continue
		}
		newIdxs = append(newIdxs, idx)
	}
	return newIdxs
}
