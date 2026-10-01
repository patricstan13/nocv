package interfaces

import (
	"example.com/hypothetical/model"
	"example.com/hypothetical/repo"
)

var _ model.Validatable = repo.Single()

var _ model.Touchable = repo.Pointer()
