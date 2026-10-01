// Package api provides the application's high-level entry points.
package api

import (
	"example.com/impactdemo/domain"
	"example.com/impactdemo/repository"
	"example.com/impactdemo/service"
	"example.com/impactdemo/util"
)

// Handler coordinates user-facing operations.
type Handler struct {
	Service *service.Service
}

// CreateUser normalizes input before calling the service layer.
func (h *Handler) CreateUser(id domain.ID, name string) error {
	return h.Service.CreateUser(id, util.NormalizeName(name))
}

// CreateGuest supplies an untyped string constant where domain.ID is expected.
func (h *Handler) CreateGuest() error {
	return h.CreateUser("guest", "Guest User")
}

// FindUser retrieves a user through the service layer.
func (h *Handler) FindUser(id domain.ID) (domain.User, error) {
	return h.Service.FindUser(id)
}

// ValidateGuest calls the method promoted from BaseService.
func (h *Handler) ValidateGuest() error {
	return h.Service.Validate("guest")
}

// FormatTags exercises ordinary and ellipsis calls to util.JoinTags.
func (h *Handler) FormatTags(tags []string) string {
	_ = util.JoinTags("new", "user")
	return util.JoinTags(tags...)
}

// CheckRepositories passes both value- and pointer-receiver implementations
// through the Repository interface.
func CheckRepositories(id domain.ID) error {
	memory := repository.MemoryRepository{}
	if err := service.SaveWith(memory, id); err != nil {
		return err
	}
	postgres := &repository.PostgresRepository{}
	return service.SaveWith(postgres, id)
}
