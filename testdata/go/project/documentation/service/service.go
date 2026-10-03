// Package service coordinates application workflows.
package service

// Service coordinates order creation.
//
// It validates requests before persistence.
type Service struct{}

// Repository defines persistence behavior.
type Repository interface {
	// Save persists an entity.
	Save()
}

type (
	// Worker performs background work.
	Worker struct{}

	// Store defines grouped persistence behavior.
	Store interface {
		// Put stores a value.
		Put()
	}
)

// This group comment must not become declaration documentation.
type (
	UndocumentedGrouped struct{}
)

// Run starts the application.
func Run(service Service) {
	service.Create()
}

// Create persists a new order.
func (Service) Create() {
	// Retry behavior is an implementation detail, not declaration documentation.
}

func helper() {}

// PostgresRepository persists entities.
type PostgresRepository struct{}

func (PostgresRepository) Save() {}
