package valid

import (
	"example.com/hypothetical/model"
	"example.com/hypothetical/repo"
)

func consumeAny(any) {}

func Example() {
	u, err := repo.Find()
	_ = err
	consumeAny(u)
	_ = u.Name
	u.Validate()
	var _ model.Validatable = u
}

func Discard() {
	repo.Find()
}
