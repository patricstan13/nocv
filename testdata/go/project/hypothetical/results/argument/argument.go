package argument

import (
	"example.com/shop/hypothetical/results/model"
	"example.com/shop/hypothetical/results/repo"
)

func consumeUser(model.User) {}

func consumeAny(any) {}

func Example() {
	consumeUser(repo.Single())
	consumeAny(repo.Single())
}
