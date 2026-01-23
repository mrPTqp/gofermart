package service

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/mrPTqp/gofermart/internal/floatutils"
	"github.com/mrPTqp/gofermart/internal/model"
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
	accountService AccountService,
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
		accountService:   accountService,
		logger:           logger,
	}, nil
}

func (s *AccrualServiceDefault) ProcessOrders(ctx context.Context) {
	s.logger.Debug("Starting parallel order processing with streaming")

	const numWorkers = 10
	jobs := make(chan model.Order, 100)
	var wg sync.WaitGroup

	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			for {
				select {
				case <-ctx.Done():
					s.logger.Debug("Worker shutting down", zap.Int("worker_id", workerID))
					return
				case order, ok := <-jobs:
					if !ok {
						return
					}
					if err := s.processOrder(ctx, order); err != nil {
						s.logger.Error("Failed to process order",
							zap.Int("worker_id", workerID),
							zap.String("order_number", order.Number),
							zap.Error(err))
					}
				}
			}
		}(i)
	}

	go func() {
		defer close(jobs)

		err := s.orderRepository.StreamOrdersForProcessing(ctx, func(order model.Order) error {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case jobs <- order:
				s.logger.Debug("Queued order for processing", zap.String("order_number", order.Number))
				return nil
			}
		})

		if err != nil {
			s.logger.Error("Error streaming orders from database", zap.Error(err))
		}
	}()

	wg.Wait()

	s.logger.Debug("Parallel order processing completed")
}

func (s *AccrualServiceDefault) processOrder(ctx context.Context, order model.Order) error {
	accrualURL := s.accrualSystemURL.ResolveReference(&url.URL{
		Path: fmt.Sprintf("/api/orders/%s", order.Number),
	})

	s.logger.Debug("Querying accrual system",
		zap.String("url", accrualURL.String()),
		zap.String("order_number", order.Number))

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
			updatedOrder.Accrual = &roundedAccrual
		} else {
			updatedOrder.Accrual = nil
		}

		if err := s.orderRepository.Update(ctx, &updatedOrder); err != nil {
			return fmt.Errorf("failed to update order: %w", err)
		}

		if updatedOrder.StatusCode == model.OrderStatusProcessed && updatedOrder.Accrual != nil {
			s.logger.Info("Accrual confirmed, increasing balance",
				zap.String("order_number", order.Number),
				zap.Float64("accrual", *updatedOrder.Accrual),
				zap.Int64("user_id", order.UserID))

			if err := s.accountService.IncreaseBalance(ctx, order.UserID, order.Number, *updatedOrder.Accrual); err != nil {
				s.logger.Error("Failed to increase balance",
					zap.String("order_number", order.Number),
					zap.Int64("user_id", order.UserID),
					zap.Error(err))
			}
		}

	case http.StatusNoContent:
		s.logger.Debug("Order not found in accrual system, will retry later",
			zap.String("order_number", order.Number))

	case http.StatusTooManyRequests:
		retryAfter := resp.Header.Get("Retry-After")
		s.logger.Warn("Too many requests",
			zap.String("retry_after", retryAfter),
			zap.String("order_number", order.Number))

	case http.StatusInternalServerError:
		s.logger.Warn("Internal server error from accrual system",
			zap.Int("status_code", resp.StatusCode),
			zap.String("order_number", order.Number))

	default:
		s.logger.Warn("Unexpected status from accrual system",
			zap.Int("status_code", resp.StatusCode),
			zap.String("order_number", order.Number))
	}

	return nil
}
