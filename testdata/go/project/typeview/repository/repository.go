package repository

import "example.com/shop/typeview/store"

type Repository struct{}

func (Repository) Save() {}

func (Repository) Update(s store.Store) {
	s.Put()
}
