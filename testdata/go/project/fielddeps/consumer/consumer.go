package consumer

import "example.com/shop/fielddeps/model"

type Direct struct {
	Value model.B
}

type Containers struct {
	Pointer *model.B
	Slice   []model.B
	Array   [2]model.B
	Map     map[string]model.B
	Channel chan model.B
	Primary model.B
	Backup  *model.B
}

type Nested struct {
	Box      model.Box[model.B]
	Function func(model.B) model.C
	Builtin  func(string) error
}

type Node struct {
	Next *Node
}
