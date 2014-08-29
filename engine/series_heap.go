package engine

import (
	"sort"

	"github.com/influxdb/influxdb/common/heap"
	"github.com/influxdb/influxdb/protocol"
)

type Value struct {
	streamId int
	s        *protocol.Series
}

type ValueSlice []Value

func (vs ValueSlice) Len() int {
	return len(vs)
}

func (vs ValueSlice) Less(i, j int) bool {
	return vs[i].s.Points[0].GetTimestamp() < vs[j].s.Points[0].GetTimestamp()
}

func (vs ValueSlice) Swap(i, j int) {
	vs[i], vs[j] = vs[j], vs[i]
}

type SeriesHeap struct {
	Ascending bool
	values    []Value
}

func (sh *SeriesHeap) Size() int {
	return len(sh.values)
}

func (sh *SeriesHeap) Add(streamId int, s *protocol.Series) {
	sh.values = append(sh.values, Value{
		s:        s,
		streamId: streamId,
	})
	l := sh.Size()
	if sh.Ascending {
		heap.BubbleUp(ValueSlice(sh.values), l-1)
	} else {
		heap.BubbleUp(sort.Reverse(ValueSlice(sh.values)), l-1)
	}
}

func (sh *SeriesHeap) Next() (int, *protocol.Series) {
	idx := 0
	s := sh.Size()
	v := sh.values[idx]
	sh.values, sh.values[0] = sh.values[:s-1], sh.values[s-1]
	if sh.Ascending {
		heap.BubbleDown(ValueSlice(sh.values), 0)
	} else {
		heap.BubbleDown(sort.Reverse(ValueSlice(sh.values)), 0)
	}
	return v.streamId, v.s
}
