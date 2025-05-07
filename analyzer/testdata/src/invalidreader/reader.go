package reader

import (
	"data"
)

type Reader struct {
	d *data.BigStruct
}

func (r *Reader) IncDecInt() int {
	r.d.Value1++ // want "IncDec modification of BigStruct is forbidden"
	r.d.Value1-- // want "IncDec modification of BigStruct is forbidden"
	return r.d.Value1
}

func (r *Reader) AssignmentString() string {
	r.d.Value1 += 1      // want "assignment to field of BigStruct is forbidden"
	r.d.Value1 = 2       // want "assignment to field of BigStruct is forbidden"
	r.d.Value2 = "input" // want "assignment to field of BigStruct is forbidden"
	return r.d.Value2
}

/*
func (r *Reader) ModifyThroughPointer() string {
	p := &r.d.Value2
	p = "new string"

	modifyString(p)

	return
}

func modifyString(s *string) {
	*s = "new string"
}
*/
