// Package service coordinates user operations.
package service

import (
	"example.com/shop/impact/domain"
	"example.com/shop/impact/repository"
	"example.com/shop/impact/util"
)

// Service coordinates validation, auditing, and persistence. Embedding the
// value AuditBase exposes Audit only through *Service.
type Service struct {
	BaseService
	AuditBase
	Repository repository.Repository
}

// ExtendedService exercises recursive method promotion.
type ExtendedService struct {
	Service
}

// CreateUser validates and stores a normalized user.
func (s *Service) CreateUser(id domain.ID, name string) error {
	if err := s.Validate(id); err != nil {
		return err
	}
	if err := s.Audit(id); err != nil {
		return err
	}
	user := domain.User{ID: id, Name: util.NormalizeName(name)}
	return s.Repository.Save(user.ID)
}

// FindUser retrieves one user.
func (s *Service) FindUser(id domain.ID) (domain.User, error) {
	return s.Repository.Find(id)
}

// SaveWith demonstrates concrete-to-interface assignability at call sites.
func SaveWith(storage repository.Repository, id domain.ID) error {
	return storage.Save(id)
}
