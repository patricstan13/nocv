package reorder

import (
	"example.com/shop/hypothetical/results/model"
	"example.com/shop/hypothetical/results/repo"
)

func consume(model.User, error) {}

func Example() {
	consume(repo.Reordered())
}
