package engine

import (
	"code.google.com/p/log4go"
	"github.com/influxdb/influxdb/protocol"
)

type Merger struct {
	name                  string
	s                     []StreamQuery
	size                  int
	h                     *SeriesHeap
	n                     Processor
	lastStreamIdx         int
	initializing          bool
	mergeColumns          bool
	fields                map[string]struct{}
	resultFields          []string
	resultFieldsPerStream map[int][]int
}

func NewCME(name string, s []StreamQuery, h *SeriesHeap, n Processor, mergeColumns bool) *Merger {
	log4go.Debug("%sMerger: created with %d streams", name, len(s))
	return &Merger{
		name:                  name,
		s:                     s,
		h:                     h,
		n:                     n,
		lastStreamIdx:         0,
		mergeColumns:          mergeColumns,
		initializing:          true,
		fields:                make(map[string]struct{}),
		resultFieldsPerStream: make(map[int][]int),
	}
}

func (cme *Merger) Update() (bool, error) {
	if cme.initializing {
		return cme.initialize()
	}
	return cme.tryYieldNextPoint()
}

func (cme *Merger) initialize() (bool, error) {
	for cme.h.Size() != len(cme.s) {
		stream := cme.s[cme.lastStreamIdx]
		if !stream.HasPoint() && !stream.Closed() {
			log4go.Debug("%sMerger: data not ready for stream %d, still initializing", cme.name, cme.lastStreamIdx)
			return true, nil
		}

		if stream.HasPoint() {
			p := stream.Next()
			cme.h.Add(cme.lastStreamIdx, p)
			for _, f := range p.Fields {
				cme.fields[f] = struct{}{}
			}
			cme.lastStreamIdx++
		} else if stream.Closed() {
			s := len(cme.s)
			cme.s[cme.lastStreamIdx] = cme.s[s-1]
			cme.s = cme.s[:s-1]
		}

	}

	if cme.mergeColumns {
		// finished initialization
		cme.resultFields = make([]string, 0, len(cme.fields))
		for f := range cme.fields {
			cme.resultFields = append(cme.resultFields, f)
		}
	}

	log4go.Debug("%sMerger initialization finished", cme.name)
	cme.initializing = false
	cme.size = len(cme.s)
	return cme.yieldNextPoint()
}

func (cme *Merger) tryYieldNextPoint() (bool, error) {
	stream := cme.s[cme.lastStreamIdx]
	// If the stream has new points, added to the heap
	if stream.HasPoint() {
		cme.h.Add(cme.lastStreamIdx, stream.Next())
	} else if stream.Closed() {
		cme.size--
	}

	// If all streams have yielded one point. Then we can get the next
	// point with the smallest (or largest) timestamp and yield it to the
	// next processor.
	if cme.h.Size() != cme.size {
		return true, nil
	}

	return cme.yieldNextPoint()
}

func (cme *Merger) yieldNextPoint() (bool, error) {
	for {
		var s *protocol.Series
		cme.lastStreamIdx, s = cme.h.Next()
		log4go.Debug("cme.lastStreamIdx: %d, s: %s", cme.lastStreamIdx, s)
		cme.fixFields(s)
		log4go.Debug("%sMerger yielding to %s: %s", cme.name, cme.n.Name(), s)
		ok, err := cme.n.Yield(s)
		if !ok || err != nil {
			return ok, err
		}

		stream := cme.s[cme.lastStreamIdx]
		if stream.HasPoint() {
			s := stream.Next()
			log4go.Debug("%sMerger received %s from %d", s, cme.lastStreamIdx)
			cme.h.Add(cme.lastStreamIdx, s)
			continue
		} else if stream.Closed() {
			cme.size--
			if cme.size != 0 {
				continue
			}
		}

		return true, nil
	}
}

func (cme *Merger) fixFields(s *protocol.Series) {
	if !cme.mergeColumns {
		return
	}

	idx := cme.lastStreamIdx
	mapping := cme.resultFieldsPerStream[idx]
	if mapping == nil {
		for _, f := range cme.resultFields {
			index := -1
			for i, sf := range s.Fields {
				if sf == f {
					index = i
					break
				}
			}
			mapping = append(mapping, index)
			cme.resultFieldsPerStream[idx] = mapping
		}
	}

	s.Fields = cme.resultFields
	p := s.Points[0]
	originalValues := p.Values
	p.Values = nil
	for _, i := range mapping {
		if i == -1 {
			p.Values = append(p.Values, nil)
			continue
		}
		p.Values = append(p.Values, originalValues[i])
	}
}
