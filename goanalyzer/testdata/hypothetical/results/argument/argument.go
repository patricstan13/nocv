package argument

import (
	"example.com/hypothetical/model"
	"example.com/hypothetical/repo"
)

func consumeUser(model.User) {}

func consumeAny(any) {}

func Example() {
	consumeUser(repo.Single())
	consumeAny(repo.Single())
}
