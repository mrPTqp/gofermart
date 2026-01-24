package luhn

import (
	"strconv"
)

func IsValid(number string) bool {
	if len(number) == 0 {
		return false
	}
	var sum int
	double := false
	for i := len(number) - 1; i >= 0; i-- {
		digit, err := strconv.Atoi(string(number[i]))
		if err != nil {
			return false
		}
		if double {
			digit *= 2
			if digit > 9 {
				digit = digit%10 + 1
			}
		}
		sum += digit
		double = !double
	}
	return sum%10 == 0
}
