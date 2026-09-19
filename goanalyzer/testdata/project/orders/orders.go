package orders

type Order struct{}

type Repository interface {
	Save(Order) error
}

type Service struct{}

func NewService() *Service {
	return &Service{}
}

func (Service) Health() error {
	return nil
}

func (*Service) Create() error {
	return nil
}

type Box[T any] struct{}

func (b *Box[T]) Get() {}
