package model

import (
	"errors"
	"strconv"
	"strings"
)

type NetAddress struct {
	Host string
	Port int
}

func (na *NetAddress) SetAddress(address string) error {
	if strings.HasPrefix(address, "http://") {
		address = strings.TrimPrefix(address, "http://")
	} else if strings.HasPrefix(address, "https://") {
		address = strings.TrimPrefix(address, "https://")
	}
	parts := strings.Split(address, ":")
	if len(parts) != 2 {
		return errors.New("wrong address format, expected host:port")
	}
	na.Host = parts[0]
	port, err := strconv.Atoi(parts[1])
	if err != nil {
		return errors.New("invalid port number: " + parts[1])
	}
	na.Port = port
	return nil
}

func (na *NetAddress) String() string {
	return na.Host + ":" + strconv.Itoa(na.Port)
}
