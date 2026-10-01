package broken

import (
	"example.com/hypothetical/model"
	"example.com/hypothetical/repo"
)

func Shift(value int) {}

var existing int = "existing"

func consumeUser(model.User) {}

func Example() {
	u, _ := repo.Find()
	consumeUser(u)
}
