package api

import (
	"example.com/hypothetical/cascade/service"
	"example.com/hypothetical/model"
)

func Get() (model.User, error) {
	return service.Get()
}
