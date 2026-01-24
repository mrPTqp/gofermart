package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/mrPTqp/gofermart/internal/model"
	"github.com/mrPTqp/gofermart/internal/repository"
)

type mockOrderRepository struct {
	orders map[string]model.Order
	err    error
}

func (m *mockOrderRepository) GetByNumber(ctx context.Context, number string) (*model.Order, error) {
	if m.err != nil {
		return nil, m.err
	}
	order, ok := m.orders[number]
	if !ok {
		return nil, repository.ErrNotFound
	}
	return &order, nil
}

func (m *mockOrderRepository) Create(ctx context.Context, number string, userID int64) error {
	if m.err != nil {
		return m.err
	}
	if existing, exists := m.orders[number]; exists {
		if existing.UserID == userID {
			return repository.ErrOrderExists
		}
		return repository.ErrAnotherUser
	}
	m.orders[number] = model.Order{
		Number:     number,
		UserID:     userID,
		StatusCode: model.OrderStatusNew,
		UploadedAt: time.Now(),
	}
	return nil
}

func (m *mockOrderRepository) GetByUser(ctx context.Context, userID int64) ([]model.Order, error) {
	if m.err != nil {
		return nil, m.err
	}
	var orders []model.Order
	for _, order := range m.orders {
		if order.UserID == userID {
			orders = append(orders, order)
		}
	}
	return orders, nil
}

func (m *mockOrderRepository) StreamOrdersForProcessing(ctx context.Context, processor repository.OrderProcessor) error {
	if m.err != nil {
		return m.err
	}
	for _, order := range m.orders {
		if order.StatusCode == model.OrderStatusNew || order.StatusCode == model.OrderStatusProcessing {
			if err := processor(order); err != nil {
				return err
			}
		}
	}
	return nil
}

func (m *mockOrderRepository) Update(ctx context.Context, order *model.Order) error {
	if m.err != nil {
		return m.err
	}
	m.orders[order.Number] = *order
	return nil
}

func (m *mockOrderRepository) UpdateStatus(ctx context.Context, number, status string) error {
	if m.err != nil {
		return m.err
	}
	order, ok := m.orders[number]
	if !ok {
		return repository.ErrNotFound
	}
	order.StatusCode = model.OrderStatus(status)
	order.UpdatedAt = time.Now()
	order.LastCheckedAt = time.Now()
	m.orders[number] = order
	return nil
}

func TestOrderService_UploadOrder(t *testing.T) {
	tests := []struct {
		name       string
		userID     int64
		orderNum   string
		mockOrders map[string]model.Order
		mockErr    error
		wantErr    bool
		wantErrIs  error
	}{
		{
			name:       "new order success",
			userID:     1,
			orderNum:   "12345",
			mockOrders: map[string]model.Order{},
			wantErr:    false,
		},
		{
			name:       "already uploaded by same user",
			userID:     1,
			orderNum:   "12345",
			mockOrders: map[string]model.Order{"12345": {Number: "12345", UserID: 1}},
			wantErr:    true,
			wantErrIs:  repository.ErrOrderExists,
		},
		{
			name:       "uploaded by another user",
			userID:     2,
			orderNum:   "12345",
			mockOrders: map[string]model.Order{"12345": {Number: "12345", UserID: 1}},
			wantErr:    true,
			wantErrIs:  repository.ErrAnotherUser,
		},
		{
			name:     "repo error on get",
			userID:   1,
			orderNum: "12345",
			mockErr:  errors.New("db error"),
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &mockOrderRepository{orders: tt.mockOrders, err: tt.mockErr}
			svc := NewOrderService(repo)

			err := svc.UploadOrder(context.Background(), tt.userID, tt.orderNum)

			if (err != nil) != tt.wantErr {
				t.Fatalf("UploadOrder() error = %v, wantErr = %v", err, tt.wantErr)
			}
			if tt.wantErrIs != nil {
				if !errors.Is(err, tt.wantErrIs) {
					t.Errorf("Expected error %v, got %v", tt.wantErrIs, err)
				}
			}
		})
	}
}

func TestOrderService_GetUserOrders(t *testing.T) {
	tests := []struct {
		name       string
		userID     int64
		mockOrders map[string]model.Order
		mockErr    error
		wantErr    bool
		wantLen    int
	}{
		{
			name:       "orders found",
			userID:     1,
			mockOrders: map[string]model.Order{"123": {UserID: 1}, "456": {UserID: 1}},
			wantErr:    false,
			wantLen:    2,
		},
		{
			name:       "no orders for user",
			userID:     2,
			mockOrders: map[string]model.Order{"123": {UserID: 1}},
			wantErr:    false,
			wantLen:    0,
		},
		{
			name:    "repo error",
			userID:  1,
			mockErr: errors.New("db error"),
			wantErr: true,
			wantLen: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &mockOrderRepository{orders: tt.mockOrders, err: tt.mockErr}
			svc := NewOrderService(repo)

			orders, err := svc.GetUserOrders(context.Background(), tt.userID)

			if (err != nil) != tt.wantErr {
				t.Fatalf("GetUserOrders() error = %v, wantErr = %v", err, tt.wantErr)
			}
			if len(orders) != tt.wantLen {
				t.Errorf("Expected %d orders, got %d", tt.wantLen, len(orders))
			}
		})
	}
}
