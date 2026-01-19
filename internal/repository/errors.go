package repository

import "errors"

var (
	ErrNotFound          = errors.New("not found")
	ErrLoginExists       = errors.New("login already exists")
	ErrOrderExists       = errors.New("order already exists for this user")
	ErrOrderTaken        = errors.New("order already exists for another user")
	ErrInsufficientFunds = errors.New("insufficient funds")
)
