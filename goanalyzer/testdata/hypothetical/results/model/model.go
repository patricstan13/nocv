package model

type User struct {
	Name string
}

func (User) Validate() {}

func (*User) Touch() {}

type Validatable interface {
	Validate()
}

type Touchable interface {
	Touch()
}
