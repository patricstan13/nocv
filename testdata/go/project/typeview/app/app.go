package app

import (
	"example.com/shop/typeview/repository"
	"example.com/shop/typeview/service"
)

func Run(s service.Service, r repository.Repository) {
	s.Create(r)
}
