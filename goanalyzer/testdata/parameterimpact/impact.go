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

func (Service) Health() {}

type Store interface {
	Save(id ID)
}

type Saver interface {
	Save(id ID)
}

type Healthy interface {
	Health()
}

type ExtendedStore interface {
	Store
	Health()
}

type StringSaver interface {
	Save(value string)
}

type PointerStore struct{}

func (*PointerStore) Save(id ID) {}

type StringStore struct{}

func (StringStore) Save(value string) {}

type PromotionBase struct{}

func (PromotionBase) Change(id ID) {}

type PromotionWrapper struct {
	PromotionBase
}

type PromotionOuter struct {
	PromotionWrapper
}

type PromotionShadow struct {
	PromotionBase
}

func (PromotionShadow) Change(id ID) {}

type PromotionRival struct{}

func (PromotionRival) Change(id ID) {}

type PromotionAmbiguous struct {
	PromotionBase
	PromotionRival
}

type PointerBase struct{}

func (*PointerBase) Touch(id ID) {}

type ValueEmbed struct {
	PointerBase
}

type PointerEmbed struct {
	*PointerBase
}

type RecursivePointerMid struct {
	PointerBase
}

type RecursivePointerOuter struct {
	RecursivePointerMid
}

type VariadicBase struct{}

func (VariadicBase) Collect(values ...string) {}

type VariadicEmbed struct {
	VariadicBase
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
