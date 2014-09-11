package datastore

import "github.com/influxdb/influxdb/protocol"

type PointIteratorStream struct {
	pi     *PointIterator
	name   string
	fields []string
}

func (pis PointIteratorStream) HasPoint() bool {
	return pis.pi.Valid()
}
func (pis PointIteratorStream) Next() *protocol.Series {
	p := pis.pi.Point()
	s := &protocol.Series{
		Name:   &pis.name,
		Fields: pis.fields,
		Points: []*protocol.Point{p},
	}
	pis.pi.Next()
	return s
}

func (pis PointIteratorStream) Closed() bool {
	return !pis.pi.Valid()
}
