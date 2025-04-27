package reader

import (
	"data"
)

type Reader struct {
	d *data.BigStruct
}

func (r *Reader) ReadInt() int {
	return r.d.Value1
}

func (r *Reader) ReadString() string {
	return r.d.Value2
}
