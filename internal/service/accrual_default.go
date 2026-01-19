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
	logger           *zap.SugaredLogger
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
	logger *zap.SugaredLogger,
) (*AccrualServiceDefault, error) {
	accrualURL, err := url.Parse(accrualSystemAddress)
	if err != nil {
		return nil, fmt.Errorf("invalid accrual system address: %v", err)
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
	s.logger.Info("Starting order processing cycle")

	// Получаем список заказов со статусом NEW или PROCESSING
	orders, err := s.orderRepository.GetOrdersForProcessing(ctx)
	if err != nil {
		s.logger.Errorf("failed to get orders for processing: %v", err)
		return
	}
	s.logger.Infof("orders with status NEW or PROCESSING %s", orders)

	for _, order := range orders {
		s.logger.Infof("start processing order %s", order.Number)
		if err := s.processOrder(ctx, order); err != nil {
			s.logger.Errorf("failed to process order %s: %v", order.Number, err)
		}
	}
}

func (s *AccrualServiceDefault) processOrder(ctx context.Context, order model.Order) error {
	// Формируем URL для запроса к системе начислений
	accrualURL := s.accrualSystemURL.ResolveReference(&url.URL{
		Path: fmt.Sprintf("/api/orders/%s", order.Number),
	})

	// Выполняем запрос
	resp, err := s.client.Get(accrualURL.String())
	if err != nil {
		return fmt.Errorf("failed to query accrual system: %v", err)
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusOK:
		var accrualResp AccrualResponse
		if err := json.NewDecoder(resp.Body).Decode(&accrualResp); err != nil {
			return fmt.Errorf("failed to decode accrual response: %v", err)
		}
		s.logger.Infof("response from accrualService %s", accrualResp)

		// Преобразуем статус из системы начислений в наш статус
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
			s.logger.Warnf("unknown status from accrual system: %s", accrualResp.Status)
			return nil // Пропускаем обработку при неизвестном статусе
		}

		// Обновляем статус заказа
		updatedOrder := order
		updatedOrder.StatusCode = newStatus
		updatedOrder.UpdatedAt = time.Now()

		// Округляем начисление при сохранении
		if accrualResp.Accrual != nil {
			roundedAccrual := floatutils.Round(*accrualResp.Accrual, 2)
			s.logger.Infof("rounded accrual %f", roundedAccrual)
			updatedOrder.Accrual = &roundedAccrual
			s.logger.Infof("updated accrual %f", *updatedOrder.Accrual)
		} else {
			updatedOrder.Accrual = nil
		}

		if err := s.orderRepository.Update(ctx, &updatedOrder); err != nil {
			return fmt.Errorf("failed to update order: %v", err)
		}

		if updatedOrder.StatusCode == model.OrderStatusProcessed && updatedOrder.Accrual != nil {
			// Передаём номер заказа в IncreaseBalance
			if err := s.accountService.IncreaseBalance(ctx, order.UserID, order.Number, *updatedOrder.Accrual); err != nil {
				s.logger.Errorf("failed to update user balance: %v", err)
			}
		}

	case http.StatusNoContent:
		// Заказ не найден в системе расчета - оставляем в статусе NEW для повторной проверки позже
		s.logger.Infof("Order %s not found in accrual system, will retry later", order.Number)

	case http.StatusTooManyRequests:
		// Превышено количество запросов - пропускаем эту итерацию
		s.logger.Warn("Too many requests to accrual system, skipping this cycle")
		return nil

	case http.StatusInternalServerError:
		// Ошибка сервера - логируем и попробуем позже
		s.logger.Warnf("Internal error from accrual system for order %s", order.Number)

	default:
		s.logger.Warnf("Unexpected status %d from accrual system for order %s", resp.StatusCode, order.Number)
	}

	return nil
}
