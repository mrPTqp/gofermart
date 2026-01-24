package config

import (
	"os"
	"time"
)

type Envs struct {
	Address            *string
	DatabaseDsn        *string
	AccrualAddress     *string
	JWTSecret          *string
	JWTTTL             *time.Duration
	OrderCheckInterval *time.Duration
}

func ParseEnvs() *Envs {
	var address string
	var databaseDsn string
	var accrualAddress string
	var JWTSecret string
	var JWTTTL time.Duration
	var orderCheckInterval time.Duration
	var err error

	if envAddr := os.Getenv("RUN_ADDRESS"); envAddr != "" {
		address = envAddr
	}

	if envDatabaseDsn := os.Getenv("DATABASE_URI"); envDatabaseDsn != "" {
		databaseDsn = envDatabaseDsn
	}

	if envAccrualAddr := os.Getenv("ACCRUAL_SYSTEM_ADDRESS"); envAccrualAddr != "" {
		accrualAddress = envAccrualAddr
	}

	if envJWTSecret := os.Getenv("JWT_SECRET"); envJWTSecret != "" {
		JWTSecret = envJWTSecret
	}

	if envJWTTTL := os.Getenv("JWT_TTL"); envJWTTTL != "" {
		JWTTTL, err = time.ParseDuration(envJWTTTL)
		if err != nil {
			panic("wrong JWT_TTL type value: " + envJWTTTL)
		}
	}

	if envInterval := os.Getenv("ORDER_CHECK_INTERVAL"); envInterval != "" {
		orderCheckInterval, err = time.ParseDuration(envInterval)
		if err != nil {
			panic("wrong ORDER_CHECK_INTERVAL duration: " + envInterval)
		}
	}

	return &Envs{
		Address:            &address,
		DatabaseDsn:        &databaseDsn,
		AccrualAddress:     &accrualAddress,
		JWTSecret:          &JWTSecret,
		JWTTTL:             &JWTTTL,
		OrderCheckInterval: &orderCheckInterval,
	}
}
