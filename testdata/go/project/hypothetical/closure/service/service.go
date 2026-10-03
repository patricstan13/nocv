package service

import "example.com/shop/hypothetical/closure/repo"

func Get() int {
	return repo.Find()
}
