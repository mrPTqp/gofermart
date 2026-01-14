package config

import (
	"github.com/mrPTqp/gofermart/internal/model"
)

type Config struct {
	Address        model.NetAddress
	DatabaseDsn    *string
	AccrualAddress model.NetAddress
}

func LoadConfig() *Config {
	envs := ParseEnvs()
	flags := ParseFlags()

	na := model.NetAddress{}
	address := "localhost:8080"
	if envs.Address != nil && *envs.Address != "" {
		address = *envs.Address
	} else if flags.Address != nil && *flags.Address != "" {
		address = *flags.Address
	}
	if err := na.SetAddress(address); err != nil {
		panic("invalid address: " + address + " error: " + err.Error())
	}

	var databaseDsn string
	if envs.DatabaseDsn != nil && *envs.DatabaseDsn != "" {
		databaseDsn = *envs.DatabaseDsn
	} else if flags.DatabaseDsn != nil && *flags.DatabaseDsn != "" {
		databaseDsn = *flags.DatabaseDsn
	}

	ana := model.NetAddress{}
	accrualAddress := "localhost:8080"
	if envs.AccrualAddress != nil && *envs.AccrualAddress != "" {
		accrualAddress = *envs.AccrualAddress
	} else if flags.AccrualAddress != nil && *flags.AccrualAddress != "" {
		accrualAddress = *flags.AccrualAddress
	}
	if err := ana.SetAddress(accrualAddress); err != nil {
		panic("invalid accrual address: " + accrualAddress + " error: " + err.Error())
	}

	return &Config{
		Address:        na,
		DatabaseDsn:    &databaseDsn,
		AccrualAddress: ana,
	}
}
