package service

import (
	"example.com/hypothetical/model"
	"example.com/hypothetical/repo"
)

func Get() (model.User, error) {
	return repo.Find()
}
