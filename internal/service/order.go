// internal/service/order.go
package service

import (
	"context"
	"errors"

	"github.com/mrPTqp/gofermart/internal/model"
	"github.com/mrPTqp/gofermart/internal/repository"
)

type OrderService interface {
    UploadOrder(ctx context.Context, userID int64, orderNum string) error
    GetUserOrders(ctx context.Context, userID int64) ([]model.Order, error)
}

type OrderServiceImpl struct {
	repo repository.OrderRepository
}

func NewOrderServiceImpl(repo repository.OrderRepository) *OrderServiceImpl {
	return &OrderServiceImpl{repo: repo}
}

func (s *OrderServiceImpl) UploadOrder(ctx context.Context, userID int64, orderNum string) error {
	existingOrder, err := s.repo.GetByNumber(ctx, orderNum)
	if errors.Is(err, repository.ErrNotFound) {
		return s.repo.Create(ctx, orderNum, userID)
	}
	if err != nil {
		return err
	}

	if existingOrder.UserID == userID {
		return errors.New("already uploaded")
	}

	return errors.New("another user")
}

func (s *OrderServiceImpl) GetUserOrders(ctx context.Context, userID int64) ([]model.Order, error) {
	orders, err := s.repo.GetByUser(ctx, userID)
	if err != nil {
		return nil, err
	}
	return orders, nil
}
