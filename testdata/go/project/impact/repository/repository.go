// Package repository defines and implements user persistence contracts.
package repository

import "example.com/shop/impact/domain"

// Repository stores and retrieves users.
type Repository interface {
	Save(domain.ID) error
	Find(domain.ID) (domain.User, error)
}
