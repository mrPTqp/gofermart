package config

import (
	"os"
	"time"
)

type Envs struct {
	Address        *string
	DatabaseDsn    *string
	AccrualAddress *string
	JWTSecret      *string
	JWTTTL         *time.Duration
}

func ParseEnvs() *Envs {
	var address string
	var databaseDsn string
	var accrualAddress string
	var JWTSecret string
	var JWTTTL time.Duration

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

	var err error
	if envJWTTTL := os.Getenv("JWT_TTL"); envJWTTTL != "" {
		JWTTTL, err = time.ParseDuration(envJWTTTL)
		if err != nil {
			panic("wrong JWT_TTL type value: " + envJWTTTL)
		}
	}

	return &Envs{
		Address:        &address,
		DatabaseDsn:    &databaseDsn,
		AccrualAddress: &accrualAddress,
		JWTSecret:      &JWTSecret,
		JWTTTL:         &JWTTTL,
	}
}
