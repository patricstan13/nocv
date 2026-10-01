package repository

import "example.com/impactdemo/domain"

// MemoryRepository is a value-receiver Repository implementation.
type MemoryRepository struct{}

// Save stores a user identifier in memory.
func (MemoryRepository) Save(domain.ID) error {
	return nil
}

// Find returns a predictable in-memory user.
func (MemoryRepository) Find(id domain.ID) (domain.User, error) {
	return domain.User{ID: id, Name: "Memory User"}, nil
}
