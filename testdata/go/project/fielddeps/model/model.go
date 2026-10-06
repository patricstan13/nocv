package model

type B struct{}

type C interface {
	Marker()
}

type Box[T any] struct {
	Value T
}

type Embedded struct {
	B
}
