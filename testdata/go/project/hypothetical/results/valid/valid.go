package valid

import (
	"example.com/shop/hypothetical/results/model"
	"example.com/shop/hypothetical/results/repo"
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
