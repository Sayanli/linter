package data

type BigStruct struct {
	Value1 int
	Value2 string
}

// New создает новый экземпляр BigStruct
func New() *BigStruct {
	return &BigStruct{
		Value1: 0,
		Value2: "",
	}
}
