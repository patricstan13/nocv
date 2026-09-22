package orders

import (
	"fmt"

	"example.com/shop/logging"
)

type Order struct{}

type Repository interface {
	Save(Order) error
}

type Service struct {
	repo Repository
}

func NewService() *Service {
	return &Service{}
}

func (Service) Health() error {
	return nil
}

func (s *Service) Validate() {}

func (s *Service) Create(order Order) error {
	s.Health()
	s.Validate()
	logging.Info()
	return s.repo.Save(order)
}

func Process() {
	Validate()
	Validate()
	logging.Info()
}

func Validate() {}

func ExternalOnly(values []string) {
	fmt.Println(len(values))
	_ = append(values, "item")
}

func ConvertOnly(value rune) string {
	return string(value)
}

type Box[T any] struct{}

func (b *Box[T]) Get() {}

func Outer() {
	fn := func() {
		Validate()
	}

	_ = fn
}

type PostgresRepository struct{}

func (*PostgresRepository) Save(Order) error {
	return nil
}

type MemoryRepository struct{}

func (MemoryRepository) Save(Order) error {
	return nil
}

type BrokenRepository struct{}

type BaseRepository struct{}

func (BaseRepository) Save(Order) error {
	return nil
}

type PromotedRepository struct {
	BaseRepository
}

type Empty interface{}

type ExternalStringer struct{}

func (ExternalStringer) String() string {
	return "external"
}

var _ fmt.Stringer = ExternalStringer{}
