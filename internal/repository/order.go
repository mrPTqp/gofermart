package repository

import (
	"context"

	"github.com/mrPTqp/gofermart/internal/model"
)

type OrderProcessor func(order model.Order) error

type OrderRepository interface {
	Create(ctx context.Context, number string, userID int64) error
	GetByUser(ctx context.Context, userID int64) ([]model.Order, error)
	GetByNumber(ctx context.Context, number string) (*model.Order, error)
	UpdateStatus(ctx context.Context, number, status string) error
	StreamOrdersForProcessing(ctx context.Context, processor OrderProcessor) error
	Update(ctx context.Context, order *model.Order) error
}
