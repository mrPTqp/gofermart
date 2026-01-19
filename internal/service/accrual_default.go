// internal/service/accrual.go
package service

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/mrPTqp/gofermart/internal/model"
	"github.com/mrPTqp/gofermart/internal/floatutils"
	"github.com/mrPTqp/gofermart/internal/repository"
	"go.uber.org/zap"
)

type AccrualServiceDefault struct {
	client           *http.Client
	accrualSystemURL *url.URL
	orderRepository  repository.OrderRepository
	accountService   AccountService
	logger           *zap.Logger
}

type AccrualResponse struct {
	Order   string   `json:"order"`
	Status  string   `json:"status"`
	Accrual *float64 `json:"accrual,omitempty"`
}

func NewAccrualService(
	accrualSystemAddress string,
	orderRepository repository.OrderRepository,
	balanceService AccountService,
	logger *zap.Logger,
) (*AccrualServiceDefault, error) {
	accrualURL, err := url.Parse(accrualSystemAddress)
	if err != nil {
		return nil, fmt.Errorf("invalid accrual system address: %w", err)
	}

	return &AccrualServiceDefault{
		client:           &http.Client{Timeout: 10 * time.Second},
		accrualSystemURL: accrualURL,
		orderRepository:  orderRepository,
		accountService:   balanceService,
		logger:           logger,
	}, nil
}

func (s *AccrualServiceDefault) ProcessOrders(ctx context.Context) {
	s.logger.Debug("Starting order processing cycle")

	orders, err := s.orderRepository.GetOrdersForProcessing(ctx)
	if err != nil {
		s.logger.Error("Failed to get orders for processing", zap.Error(err))
		return
	}

	s.logger.Debug("Fetched orders for processing",
		zap.Int("count", len(orders)),
		zap.Strings("order_numbers", func() []string {
			var nums []string
			for _, o := range orders {
				nums = append(nums, o.Number)
			}
			return nums
		}()),
	)

	for _, order := range orders {
		s.logger.Debug("Processing order", zap.String("order_number", order.Number))
		if err := s.processOrder(ctx, order); err != nil {
			s.logger.Error("Failed to process order", zap.String("order_number", order.Number), zap.Error(err))
		}
	}
}

func (s *AccrualServiceDefault) processOrder(ctx context.Context, order model.Order) error {
	accrualURL := s.accrualSystemURL.ResolveReference(&url.URL{
		Path: fmt.Sprintf("/api/orders/%s", order.Number),
	})

	s.logger.Debug("Sending request to accrual system",
		zap.String("method", "GET"),
		zap.String("url", accrualURL.String()),
		zap.String("order_number", order.Number),
	)

	resp, err := s.client.Get(accrualURL.String())
	if err != nil {
		return fmt.Errorf("failed to query accrual system: %w", err)
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusOK:
		var accrualResp AccrualResponse
		if err := json.NewDecoder(resp.Body).Decode(&accrualResp); err != nil {
			return fmt.Errorf("failed to decode accrual response: %w", err)
		}

		s.logger.Debug("Received valid response from accrual system",
			zap.String("order", accrualResp.Order),
			zap.String("status", accrualResp.Status),
			zap.Float64p("accrual", accrualResp.Accrual),
		)

		var newStatus model.OrderStatus
		switch accrualResp.Status {
		case "REGISTERED":
			newStatus = model.OrderStatusNew
		case "PROCESSING":
			newStatus = model.OrderStatusProcessing
		case "INVALID":
			newStatus = model.OrderStatusInvalid
		case "PROCESSED":
			newStatus = model.OrderStatusProcessed
		default:
			s.logger.Warn("Unknown status from accrual system",
				zap.String("status", accrualResp.Status),
				zap.String("order_number", order.Number))
			return nil
		}

		updatedOrder := order
		updatedOrder.StatusCode = newStatus
		updatedOrder.UpdatedAt = time.Now()

		if accrualResp.Accrual != nil {
			roundedAccrual := floatutils.Round(*accrualResp.Accrual, 2)
			s.logger.Debug("Rounded accrual value",
				zap.Float64("original", *accrualResp.Accrual),
				zap.Float64("rounded", roundedAccrual))
			updatedOrder.Accrual = &roundedAccrual
		} else {
			updatedOrder.Accrual = nil
		}

		if err := s.orderRepository.Update(ctx, &updatedOrder); err != nil {
			return fmt.Errorf("failed to update order in DB: %w", err)
		}

		if updatedOrder.StatusCode == model.OrderStatusProcessed && updatedOrder.Accrual != nil {
			s.logger.Info("Processing balance accrual for order",
				zap.Int64("user_id", order.UserID),
				zap.String("order_number", order.Number),
				zap.Float64("accrual", *updatedOrder.Accrual),
			)

			if err := s.accountService.IncreaseBalance(ctx, order.UserID, order.Number, *updatedOrder.Accrual); err != nil {
				s.logger.Error("Failed to increase user balance",
					zap.Int64("user_id", order.UserID),
					zap.String("order_number", order.Number),
					zap.Float64("accrual", *updatedOrder.Accrual),
					zap.Error(err))
			}
		}

	case http.StatusNoContent:
		s.logger.Debug("Order not found in accrual system, will retry later",
			zap.String("order_number", order.Number))

	case http.StatusTooManyRequests:
		retryAfter := resp.Header.Get("Retry-After")
		s.logger.Warn("Too many requests to accrual system",
			zap.String("retry_after", retryAfter),
			zap.String("order_number", order.Number))
		return nil

	case http.StatusInternalServerError:
		s.logger.Warn("Internal server error from accrual system",
			zap.Int("status_code", resp.StatusCode),
			zap.String("order_number", order.Number))

	default:
		s.logger.Warn("Unexpected HTTP status from accrual system",
			zap.Int("status_code", resp.StatusCode),
			zap.String("order_number", order.Number))
	}

	return nil
}
