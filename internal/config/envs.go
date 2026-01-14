package config

import (
	"os"
)

type Envs struct {
	Address        *string
	DatabaseDsn    *string
	AccrualAddress *string
}

func ParseEnvs() *Envs {
	var address string
	var databaseDsn string
	var accrualAddress string

	if envAddr := os.Getenv("RUN_ADDRESS"); envAddr != "" {
		address = envAddr
	}

	if envDatabaseDsn := os.Getenv("DATABASE_URI"); envDatabaseDsn != "" {
		databaseDsn = envDatabaseDsn
	}

	if envAccrualAddr := os.Getenv("ACCRUAL_SYSTEM_ADDRESS"); envAccrualAddr != "" {
		accrualAddress = envAccrualAddr
	}

	return &Envs{
		Address:        &address,
		DatabaseDsn:    &databaseDsn,
		AccrualAddress: &accrualAddress,
	}
}
