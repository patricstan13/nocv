package api

import (
	"example.com/shop/hypothetical/results/cascade/service"
	"example.com/shop/hypothetical/results/model"
)

func Get() (model.User, error) {
	return service.Get()
}
