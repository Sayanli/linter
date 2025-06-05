package data

type BigStruct struct {
	Arr    []int
	Map    map[int]int
	Value1 int
	Value2 string
}

// New создает новый экземпляр BigStruct
func New() *BigStruct {
	return &BigStruct{
		Arr:    []int{},
		Map:    make(map[int]int),
		Value1: 0,
		Value2: "",
	}
}
