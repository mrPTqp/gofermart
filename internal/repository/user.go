package repository

import (
	"context"

	"github.com/mrPTqp/gofermart/internal/model"
)

type UserRepository interface {
	Create(ctx context.Context, login string, passwordHash string) error
	FindByLogin(ctx context.Context, login string) (*model.User, error)
}
