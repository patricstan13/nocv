package service

import "example.com/closure/repo"

func Get() int {
	return repo.Find()
}
