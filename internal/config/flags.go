package config

import (
	"flag"
)

type Flags struct {
	Address        *string
	DatabaseDsn    *string
	AccrualAddress *string
}

func ParseFlags() *Flags {
	var addr string
	var databaseDsn string
	var accrualAddress string

	flag.StringVar(&addr, "a", "", "address and port to run server")
	flag.StringVar(&databaseDsn, "d", "", "databse connect string")
	flag.StringVar(&accrualAddress, "r", "", "accrual system address")

	flag.Parse()

	return &Flags{
		Address:        &addr,
		DatabaseDsn:    &databaseDsn,
		AccrualAddress: &accrualAddress,
	}
}
