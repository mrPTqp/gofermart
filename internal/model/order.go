package model

import "time"

type OrderStatus string

const (
	OrderStatusNew        OrderStatus = "NEW"
	OrderStatusProcessing OrderStatus = "PROCESSING"
	OrderStatusInvalid    OrderStatus = "INVALID"
	OrderStatusProcessed  OrderStatus = "PROCESSED"
	OrderStatusRegistered OrderStatus = "REGISTERED"
)

type Order struct {
	UserID        int64       `json:"user_id"`
	Number        string      `json:"number"`
	StatusCode    OrderStatus `json:"status"`
	Accrual       *float64    `json:"accrual,omitempty"`
	UploadedAt    time.Time   `json:"uploaded_at"`
	CreatedAt     time.Time   `json:"created_at"`
	UpdatedAt     time.Time   `json:"updated_at"`
	LastCheckedAt time.Time   `json:"-"`
}
