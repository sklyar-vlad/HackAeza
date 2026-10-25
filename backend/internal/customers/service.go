package customers

import (
	"context"

	"github.com/sklyar-vlad/HackAeza/backend/internal/entity"
)

type Query struct {
	Page       int64
	PageSize   int64
	Search     string
	RiskLevels []string
	MinScore   *float64
	MaxScore   *float64
	SortBy     string
	SortOrder  string
}

type TransactionQuery struct {
	Page      int64
	PageSize  int64
	Status    string
	Type      string
	SortOrder string
}

type Pagination struct {
	Page     int64 `json:"page"`
	PageSize int64 `json:"pageSize"`
	Total    int64 `json:"total"`
}

type Page struct {
	Items       []entity.Customer  `json:"items"`
	Pagination  Pagination         `json:"pagination"`
	Calculation entity.Calculation `json:"calculation"`
}

type Detail struct {
	Customer    entity.CustomerDetail `json:"customer"`
	Calculation entity.Calculation    `json:"calculation"`
}

type TransactionPage struct {
	Items       []entity.Transaction `json:"items"`
	Pagination  Pagination           `json:"pagination"`
	Calculation entity.Calculation   `json:"calculation"`
}

type Service struct {
	repository *Repository
}

func NewService(repository *Repository) *Service {
	return &Service{repository: repository}
}

func (s *Service) List(ctx context.Context, query Query) (Page, error) {
	calculation, horizonDays, err := s.repository.Published(ctx)
	if err != nil {
		return Page{}, err
	}
	page := Page{Items: []entity.Customer{}, Pagination: Pagination{Page: query.Page, PageSize: query.PageSize}, Calculation: calculation}
	if calculation.Version == 0 {
		return page, nil
	}
	page.Items, page.Pagination.Total, err = s.repository.List(ctx, calculation, query)
	if err != nil {
		return Page{}, err
	}
	for i := range page.Items {
		withLoss(&page.Items[i].Loss, horizonDays)
	}
	return page, nil
}

func (s *Service) Get(ctx context.Context, id entity.ID) (Detail, error) {
	calculation, horizonDays, err := s.repository.Published(ctx)
	if err != nil {
		return Detail{}, err
	}
	if calculation.Version == 0 {
		return Detail{}, entity.ErrNotFound
	}
	customer, err := s.repository.Get(ctx, calculation, id)
	if err != nil {
		return Detail{}, err
	}
	withLoss(&customer.Loss, horizonDays)
	for i := range customer.Services {
		withLoss(&customer.Services[i].Loss, horizonDays)
	}
	return Detail{Customer: customer, Calculation: calculation}, nil
}

func (s *Service) Transactions(ctx context.Context, id entity.ID, query TransactionQuery) (TransactionPage, error) {
	calculation, _, err := s.repository.Published(ctx)
	if err != nil {
		return TransactionPage{}, err
	}
	if calculation.Version == 0 {
		return TransactionPage{}, entity.ErrNotFound
	}
	items, total, err := s.repository.Transactions(ctx, calculation, id, query)
	if err != nil {
		return TransactionPage{}, err
	}
	return TransactionPage{Items: items, Pagination: Pagination{Page: query.Page, PageSize: query.PageSize, Total: total}, Calculation: calculation}, nil
}

func withLoss(loss *entity.LossEstimate, horizonDays int64) {
	loss.Currency = entity.CurrencyEUR
	loss.HorizonDays = horizonDays
}
