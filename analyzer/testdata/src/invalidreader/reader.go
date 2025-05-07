package reader

import (
	"data"
)

type Reader struct {
	d *data.BigStruct
}

func (r *Reader) ReadInt() int {
	r.d.Value1++ // want "modification of BigStruct is forbidden"
	return r.d.Value1
}

// TODO добавь декремент
func (r *Reader) DecInt() int {
	r.d.Value1-- // want "modification of BigStruct is forbidden"
	return r.d.Value1
}

func (r *Reader) ReadString() string {
	r.d.Value2 = "input" // want "assignment to field of BigStruct is forbidden"
	return r.d.Value2
}
