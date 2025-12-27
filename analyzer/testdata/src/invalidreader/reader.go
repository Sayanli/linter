package reader

import (
	"data"
)

type Reader struct {
	d *data.BigStruct
}

func (r *Reader) IncDecInt() int {
	r.d.Value1++ // want "increment/decrement of protected structure field is forbidden"
	r.d.Value1-- // want "increment/decrement of protected structure field is forbidden"
	return r.d.Value1
}

func (r *Reader) AssignmentString() string {
	r.d.Value1 += 1      // want "direct assignment to protected structure field is forbidden"
	r.d.Value1 = 2       // want "direct assignment to protected structure field is forbidden"
	r.d.Value2 = "input" // want "direct assignment to protected structure field is forbidden"
	return r.d.Value2
}

func (r *Reader) ModifyThroughPointer() string {
	p := &r.d.Value2
	b := p
	*b = "new string" // want "modification through pointer to protected structure field is forbidden"

	p2 := &r.d.Value1
	*p2 = 3 // want "modification through pointer to protected structure field is forbidden"

	return *p
}

func (r *Reader) ModifyByRange() string {
	for i, _ := range r.d.Arr {
		r.d.Arr[i] = 0 // want "modification of protected structure field is forbidden"
	}
	return "modified"
}

func (r *Reader) ModifyMap() string {
	for k, _ := range r.d.Map {
		r.d.Map[k] = 0 // want "modification of protected structure field is forbidden"
	}
	return "modified"
}

func (r *Reader) DirectFunctionCall() {
	ptr := &r.d.Value1
	r.modifyDirectly(ptr) // want "passing pointer to protected structure field to function Reader.modifyDirectly"
}

func (r *Reader) modifyDirectly(p *int) {
	*p = 42 // want "modification through pointer to protected structure field is forbidden"
}

func (r *Reader) MultiLevelFunctionCall() {
	ptr := &r.d.Value2
	r.intermediate(ptr) // want "passing pointer to protected structure field to function Reader.intermediate"
}

func (r *Reader) intermediate(p *string) {
	r.finalModify(p) // want "passing pointer to protected structure field to function Reader.finalModify"
}

func (r *Reader) finalModify(p *string) {
	*p = "modified" // want "modification through pointer to protected structure field is forbidden"
}

func (r *Reader) AliasChainAndFunctions() {
	original := &r.d.Value1
	intermediate := original
	r.useAlias(intermediate) // want "passing pointer to protected structure field to function Reader.useAlias"
}

func (r *Reader) useAlias(p *int) {
	*p = 100 // want "modification through pointer to protected structure field is forbidden"
}
