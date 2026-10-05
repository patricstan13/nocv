package interfaces

import (
	"example.com/shop/hypothetical/results/model"
	"example.com/shop/hypothetical/results/repo"
)

var _ model.Validatable = repo.Single()

var _ model.Touchable = repo.Pointer()
