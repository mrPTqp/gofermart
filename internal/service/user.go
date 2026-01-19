// internal/service/user.go
package service

import (
	"context"
)


type UserService interface {
    Register(ctx context.Context, login, password string) (int64, string, error)
    Login(ctx context.Context, login, password string) (int64, string, error)
}