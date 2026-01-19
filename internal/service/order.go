package service

import (
	"context"

	"github.com/mrPTqp/gofermart/internal/model"
)

type OrderService interface {
    UploadOrder(ctx context.Context, userID int64, orderNum string) error
    GetUserOrders(ctx context.Context, userID int64) ([]model.Order, error)
}