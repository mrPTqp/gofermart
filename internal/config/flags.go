package config

import (
	"flag"
	"time"
)

type Flags struct {
	Address            *string
	DatabaseDsn        *string
	AccrualAddress     *string
	JWTSecret          *string
	JWTTTL             *time.Duration
	OrderCheckInterval *time.Duration
}

func ParseFlags() *Flags {
	var addr string
	var databaseDsn string
	var accrualAddress string
	var JWTSecret string
	var JWTTTL time.Duration
	var orderCheckInterval time.Duration

	flag.StringVar(&addr, "a", "", "address and port to run server")
	flag.StringVar(&databaseDsn, "d", "", "databse connect string")
	flag.StringVar(&accrualAddress, "r", "", "accrual system address")
	flag.StringVar(&JWTSecret, "s", "", "JWT secret")
	flag.DurationVar(&JWTTTL, "t", 0, "JWT TTL")
	flag.DurationVar(&orderCheckInterval, "i", 0, "interval for checking order status updates")

	flag.Parse()

	return &Flags{
		Address:            &addr,
		DatabaseDsn:        &databaseDsn,
		AccrualAddress:     &accrualAddress,
		JWTSecret:          &JWTSecret,
		JWTTTL:             &JWTTTL,
		OrderCheckInterval: &orderCheckInterval,
	}
}
