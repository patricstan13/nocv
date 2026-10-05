package service

import (
	"errors"

	"example.com/shop/impact/domain"
)

// BaseService provides validation shared by services.
type BaseService struct{}

// Validate rejects an empty user identifier.
func (BaseService) Validate(id domain.ID) error {
	if id == "" {
		return errors.New("user ID is required")
	}
	return nil
}

// AuditBase provides pointer-receiver audit behavior.
type AuditBase struct{}

// Audit records access to one user identifier.
func (*AuditBase) Audit(domain.ID) error {
	return nil
}

// CustomService shadows BaseService.Validate deliberately.
type CustomService struct {
	BaseService
}

// Validate is CustomService's own validation rule.
func (CustomService) Validate(id domain.ID) error {
	if id == "custom" {
		return nil
	}
	return errors.New("custom user ID is required")
}
