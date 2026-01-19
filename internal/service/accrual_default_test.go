// internal/service/accrual_default_test.go
package service

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/mrPTqp/gofermart/internal/model"
	"go.uber.org/zap/zaptest"
)

func TestAccrualService_ProcessOrders(t *testing.T) {
	logger := zaptest.NewLogger(t)

	tests := []struct {
		name            string
		orders          []model.Order
		handler         http.HandlerFunc
		repoErr         error
		wantUpdated     map[string]model.OrderStatus
		wantIncrease    bool
		wantIncreaseErr error // ✅ Исправлено: было bool, стало error
	}{
		{
			name: "processed with accrual",
			orders: []model.Order{
				{Number: "12345", StatusCode: model.OrderStatusNew},
			},
			handler: func(w http.ResponseWriter, r *http.Request) {
				resp := AccrualResponse{
					Order:   "12345",
					Status:  "PROCESSED",
					Accrual: float64Ptr(50.5),
				}
				json.NewEncoder(w).Encode(resp)
			},
			wantUpdated:     map[string]model.OrderStatus{"12345": model.OrderStatusProcessed},
			wantIncrease:    true,
			wantIncreaseErr: nil, // ✅ Явно nil
		},
		{
			name: "invalid order",
			orders: []model.Order{
				{Number: "67890", StatusCode: model.OrderStatusNew},
			},
			handler: func(w http.ResponseWriter, r *http.Request) {
				resp := AccrualResponse{
					Order:  "67890",
					Status: "INVALID",
				}
				json.NewEncoder(w).Encode(resp)
			},
			wantUpdated:     map[string]model.OrderStatus{"67890": model.OrderStatusInvalid},
			wantIncrease:    false,
			wantIncreaseErr: nil,
		},
		{
			name: "accrual system returns 404",
			orders: []model.Order{
				{Number: "11111", StatusCode: model.OrderStatusNew},
			},
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusNoContent)
			},
			wantUpdated:     nil,
			wantIncrease:    false,
			wantIncreaseErr: nil,
		},
		{
			name: "repo error on update",
			orders: []model.Order{
				{Number: "22222", StatusCode: model.OrderStatusNew},
			},
			handler: func(w http.ResponseWriter, r *http.Request) {
				resp := AccrualResponse{
					Order:  "22222",
					Status: "PROCESSED",
				}
				json.NewEncoder(w).Encode(resp)
			},
			repoErr:         errors.New("db error"),
			wantUpdated:     nil,
			wantIncrease:    true,
			wantIncreaseErr: nil,
		},
		{
			name: "error on increase balance",
			orders: []model.Order{
				{Number: "33333", StatusCode: model.OrderStatusNew},
			},
			handler: func(w http.ResponseWriter, r *http.Request) {
				resp := AccrualResponse{
					Order:   "33333",
					Status:  "PROCESSED",
					Accrual: float64Ptr(25.0),
				}
				json.NewEncoder(w).Encode(resp)
			},
			wantUpdated:     map[string]model.OrderStatus{"33333": model.OrderStatusProcessed},
			wantIncrease:    true,
			wantIncreaseErr: errors.New("balance update failed"), // ✅ Ошибка при начислении
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := httptest.NewServer(tt.handler)
			defer srv.Close()

			accrualURL, _ := url.Parse(srv.URL)

			mockRepo := &mockOrderRepository{orders: map[string]model.Order{}}
			if tt.repoErr != nil {
				mockRepo.err = tt.repoErr
			}

			// Заполняем заказы в репо
			for _, order := range tt.orders {
				mockRepo.orders[order.Number] = order
			}

			mockAccService := &AccountServiceDefault{
				repo: &mockAccountRepository{
					increaseErr: tt.wantIncreaseErr, // ✅ Теперь корректный тип
				},
			}

			accrualSvc, err := NewAccrualService(accrualURL.String(), mockRepo, mockAccService, logger)
			if err != nil {
				t.Fatalf("Failed to create accrual service: %v", err)
			}

			accrualSvc.ProcessOrders(context.Background())

			// Проверяем статусы заказов
			for num, wantStatus := range tt.wantUpdated {
				updated, err := mockRepo.GetByNumber(context.Background(), num)
				if err != nil {
					t.Fatalf("Order %s not found after processing: %v", num, err)
				}
				if updated.StatusCode != wantStatus {
					t.Errorf("Order %s status = %s, want %s", num, updated.StatusCode, wantStatus)
				}
			}
		})
	}
}

func float64Ptr(v float64) *float64 {
	return &v
}
