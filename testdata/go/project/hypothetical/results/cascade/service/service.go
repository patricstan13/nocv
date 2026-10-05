package service

import (
	"example.com/shop/hypothetical/results/model"
	"example.com/shop/hypothetical/results/repo"
)

func Get() (model.User, error) {
	return repo.Find()
}
