package repository

import "example.com/shop/impact/domain"

// PostgresRepository models a pointer-receiver Repository implementation.
type PostgresRepository struct{}

// Save stores a user identifier in PostgreSQL.
func (*PostgresRepository) Save(domain.ID) error {
	return nil
}

// Find returns a predictable PostgreSQL-backed user.
func (*PostgresRepository) Find(id domain.ID) (domain.User, error) {
	return domain.User{ID: id, Name: "Postgres User"}, nil
}
