package reader

import (
	"data"
)

type Reader struct {
	d *data.BigStruct
}

func (r *Reader) IncDecInt() int {
	r.d.Value1++ // want "increment/decrement of BigStruct field is forbidden"
	r.d.Value1-- // want "increment/decrement of BigStruct field is forbidden"
	return r.d.Value1
}

func (r *Reader) AssignmentString() string {
	r.d.Value1 += 1      // want "assignment to field of BigStruct is forbidden"
	r.d.Value1 = 2       // want "assignment to field of BigStruct is forbidden"
	r.d.Value2 = "input" // want "assignment to field of BigStruct is forbidden"
	return r.d.Value2
}

func (r *Reader) ModifyThroughPointer() string {
	p := &r.d.Value2
	b := p
	*b = "new string" // want "modification through pointer to BigStruct field is forbidden"

	p2 := &r.d.Value1
	*p2 = 3 // want "modification through pointer to BigStruct field is forbidden"

	return *p
}

func (r *Reader) ModifyByRange() string {
	for i, _ := range r.d.Arr {
		r.d.Arr[i] = 0 // want "modification to BigStruct field is forbidden"
	}
	return "modified"
}

func (r *Reader) ModifyMap() string {
	for k, _ := range r.d.Map {
		r.d.Map[k] = 0 // want "modification to BigStruct field is forbidden"
	}
	return "modified"
}
