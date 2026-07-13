package domain

import (
	"fmt"
	"math"
	"strconv"
	"strings"
)

// ParseAmountCents parses a decimal money string (e.g. "10.00", "10.5") into cents.
func ParseAmountCents(value string) (int64, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0, fmt.Errorf("amount value is empty")
	}
	parts := strings.Split(value, ".")
	if len(parts) > 2 {
		return 0, fmt.Errorf("invalid amount format: %q", value)
	}

	dollars, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid amount dollars: %q", value)
	}
	if dollars < 0 {
		return 0, fmt.Errorf("amount must be non-negative")
	}

	var cents int64
	if len(parts) == 2 {
		frac := parts[1]
		if len(frac) == 0 || len(frac) > 2 {
			return 0, fmt.Errorf("invalid amount fraction: %q", value)
		}
		if len(frac) == 1 {
			frac += "0"
		}
		cents, err = strconv.ParseInt(frac, 10, 64)
		if err != nil {
			return 0, fmt.Errorf("invalid amount cents: %q", value)
		}
	}

	if dollars > (math.MaxInt64-cents)/100 {
		return 0, fmt.Errorf("amount overflow")
	}
	return dollars*100 + cents, nil
}

// FormatCents formats cents as a two-decimal money string.
func FormatCents(cents int64) string {
	sign := ""
	if cents < 0 {
		sign = "-"
		cents = -cents
	}
	return fmt.Sprintf("%s%d.%02d", sign, cents/100, cents%100)
}
