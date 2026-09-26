package app

import (
	"example.com/typeview/repository"
	"example.com/typeview/service"
)

func Run(s service.Service, r repository.Repository) {
	s.Create(r)
}
