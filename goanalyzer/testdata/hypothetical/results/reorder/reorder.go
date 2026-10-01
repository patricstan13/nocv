package reorder

import (
	"example.com/hypothetical/model"
	"example.com/hypothetical/repo"
)

func consume(model.User, error) {}

func Example() {
	consume(repo.Reordered())
}
