// internal/service/order.go
package service

import (
	"context"
	"errors"

	"github.com/mrPTqp/gofermart/internal/model"
	"github.com/mrPTqp/gofermart/internal/repository"
)

type OrderServiceDefault struct {
	repo repository.OrderRepository
}

func NewOrderService(repo repository.OrderRepository) *OrderServiceDefault {
	return &OrderServiceDefault{repo: repo}
}

func (s *OrderServiceDefault) UploadOrder(ctx context.Context, userID int64, orderNum string) error {
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

func (s *OrderServiceDefault) GetUserOrders(ctx context.Context, userID int64) ([]model.Order, error) {
	orders, err := s.repo.GetByUser(ctx, userID)
	if err != nil {
		return nil, err
	}
	return orders, nil
}
