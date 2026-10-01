package failure

import (
	"example.com/hypothetical/model"
	"example.com/hypothetical/repo"
)

func consumeUser(model.User) {}

func consumePair(model.User, error) {}

func Inferred() {
	u, _ := repo.Find()
	consumeUser(u)
}

func Typed() {
	var u model.User
	var err error
	u, err = repo.Find()
	_, _ = u, err
}

func Forward() (model.User, error) {
	return repo.Find()
}

func TupleArgument() {
	consumePair(repo.Find())
}
