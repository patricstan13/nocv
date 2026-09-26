package service

import "example.com/typeview/repository"

type Service struct{}

func (Service) Create(r repository.Repository) {
	r.Save()
}

func (s Service) Internal() {
	s.validate()
}

func (Service) validate() {
	repository.Repository{}.Save()
}
