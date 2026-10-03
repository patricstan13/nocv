package brokenone

type ID string

type Service struct{}

func (Service) Save(ID) {}

var first int = "one"
var second string = 2
