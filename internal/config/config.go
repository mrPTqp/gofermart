package config

import (
	"time"

	"github.com/mrPTqp/gofermart/internal/model"
)

type Config struct {
	Address            model.NetAddress
	DatabaseDsn        *string
	AccrualAddress     model.NetAddress
	JWTSecret          string
	JWTTTL             time.Duration
	OrderCheckInterval time.Duration
}

func LoadConfig() *Config {
	envs := ParseEnvs()
	flags := ParseFlags()

	na := model.NetAddress{}
	address := pickValue(envs.Address, flags.Address, "localhost:8080")
	if err := na.SetAddress(address); err != nil {
		panic("invalid address: " + address + " error: " + err.Error())
	}

	databaseDsn := pickValue(envs.DatabaseDsn, flags.DatabaseDsn, "")

	ana := model.NetAddress{}
	accrualAddress := pickValue(envs.AccrualAddress, flags.AccrualAddress, "localhost:8081")
	if err := ana.SetAddress(accrualAddress); err != nil {
		panic("invalid accrual address: " + accrualAddress + " error: " + err.Error())
	}

	jwtSecret := pickValue(envs.JWTSecret, flags.JWTSecret, "superSecretKey")

	jwtTTL := pickValue(envs.JWTTTL, flags.JWTTTL, 24*time.Hour)

	orderCheckInterval := pickValue(envs.OrderCheckInterval, flags.OrderCheckInterval, 10*time.Second)

	return &Config{
		Address:            na,
		DatabaseDsn:        &databaseDsn,
		AccrualAddress:     ana,
		JWTSecret:          jwtSecret,
		JWTTTL:             jwtTTL,
		OrderCheckInterval: orderCheckInterval,
	}
}

func pickValue[T comparable](env, flag *T, def T) T {
	var zero T
	if env != nil && *env != zero {
		return *env
	}
	if flag != nil && *flag != zero {
		return *flag
	}
	return def
}
