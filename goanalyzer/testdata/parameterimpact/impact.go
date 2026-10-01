package parameterimpact

import "time"

type ID string
type Alias = string

type Item struct{}

func (*Item) Mark() {}

type Marker interface {
	Mark()
}

type Service struct{}

func (Service) Save(id ID) {}

type Store interface {
	Save(id ID)
}

func UseID(id ID) {}

func UseString(value string) {}

func UseAny(value any) {}

func UseNumber(value int) {}

func Variadic(values ...string) {}

func Unused(value string) {}

func CallsID(id ID) {
	UseID(id)
	UseID("literal")
	func() {
		UseID(id)
	}()
}

func CallsString(value string) {
	UseString(value)
	UseString("literal")
	UseString(Alias(value))
}

func CallsAny(item Item, pointer *Item) {
	UseAny(item)
	UseAny(pointer)
}

func CallsNumber() {
	UseNumber(255)
	UseNumber(256)
}

func CallsVariadic(values []string) {
	Variadic("one", "two")
	Variadic(values...)
}

func CallsMethod(service Service, id ID) {
	service.Save(id)
}

func CallsInterface(store Store, id ID) {
	store.Save(id)
}

var _ time.Time
