package orders

import (
	"context"
	"fmt"
	"io"
	"net/http"

	"example.com/shop/logging"
)

type Order struct{}

type Repository interface {
	Save(Order) error
}

type Processor interface {
	Process(Order, Repository) Repository
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

func (s *Service) Transform(order Order, repo Repository) Repository {
	return repo
}

func RunCreate(service *Service) {
	_ = service.Create(Order{})
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

func HandleOrder(order Order, repo Repository) *Order {
	return &order
}

func LoadPair() (Order, Repository, error) {
	return Order{}, nil, nil
}

func CompareOrders(a Order, b Order) {}

func PointerOrder(order *Order) *Order {
	return order
}

func ExternalSignature(ctx context.Context) http.Request {
	return http.Request{}
}

func ContainerSignature(
	orders []Order,
	byID map[string]Order,
	stream chan Order,
) ([]Order, map[string]Order, chan Order) {
	return orders, byID, stream
}

func VariadicOrders(orders ...Order) {}

type UserID string

func FindUser(id UserID) {}

type Purchase = Order

func AliasOrder(order Purchase) Purchase {
	return order
}

func GenericBox(box Box[int]) Box[int] {
	return box
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

type EmbeddedBase struct{}

type EmbeddedChild struct {
	EmbeddedBase
}

type PointerEmbeddedChild struct {
	*EmbeddedBase
}

type NamedFieldChild struct {
	base EmbeddedBase
}

type Holder struct {
	Box[int]
}

type Reader interface {
	Read() error
}

type Writer interface {
	Write() error
}

type ReadWriter interface {
	Reader
	Writer
}

type Number interface {
	~int | ~float64
}

type HTTPWrapper struct {
	http.Client
}

type ExternalReader interface {
	io.Reader
}
