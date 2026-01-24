package model

type User struct {
	ID       int64  `json:"id"`
	Login    string `json:"login"`
	PasswordHash string `json:"-"`
}
