// internal/service/accrual.go
package service

import (
	"context"
)

type AccrualService interface {
	ProcessOrders(ctx context.Context)
}
