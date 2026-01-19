// internal/luhn/luhn_test.go
package luhn

import (
	"testing"
)

func TestIsValid(t *testing.T) {
	tests := []struct {
		name   string
		number string
		want   bool
	}{
		{"valid Luhn 79927398713", "79927398713", true},
		{"valid Luhn 4532015112830366", "4532015112830366", true},
		{"invalid Luhn 79927398714", "79927398714", false},
		{"empty string", "", false},
		{"non-digit character", "123a", false},
		{"single digit valid", "0", true},
		{"single digit invalid", "1", false},
		{"all zeros", "0000", true},
		{"odd length", "12345", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsValid(tt.number); got != tt.want {
				t.Errorf("IsValid(%q) = %v, want %v", tt.number, got, tt.want)
			}
		})
	}
}
