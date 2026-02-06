package ledger

import (
	"fmt"
	"math"
	"strconv"
	"strings"
)

type CurrencyDef struct {
	Code     string
	Name     string
	Exponent int // 2 for USD (100 cents), 0 for JPY
}

var Currencies = map[string]CurrencyDef{
	"USD": {Code: "USD", Name: "US Dollar", Exponent: 2},
	"EUR": {Code: "EUR", Name: "Euro", Exponent: 2},
	"GBP": {Code: "GBP", Name: "Pound Sterling", Exponent: 2},
	"JPY": {Code: "JPY", Name: "Japanese Yen", Exponent: 0},
	"CHF": {Code: "CHF", Name: "Swiss Franc", Exponent: 2},
	"AUD": {Code: "AUD", Name: "Australian Dollar", Exponent: 2},
	"CAD": {Code: "CAD", Name: "Canadian Dollar", Exponent: 2},
	"CNY": {Code: "CNY", Name: "Chinese Yuan", Exponent: 2},
	"INR": {Code: "INR", Name: "Indian Rupee", Exponent: 2},
	"SGD": {Code: "SGD", Name: "Singapore Dollar", Exponent: 2},
	"HKD": {Code: "HKD", Name: "Hong Kong Dollar", Exponent: 2},
	"NZD": {Code: "NZD", Name: "New Zealand Dollar", Exponent: 2},
	"SEK": {Code: "SEK", Name: "Swedish Krona", Exponent: 2},
	"NOK": {Code: "NOK", Name: "Norwegian Krone", Exponent: 2},
	"KRW": {Code: "KRW", Name: "South Korean Won", Exponent: 0},
	"BRL": {Code: "BRL", Name: "Brazilian Real", Exponent: 2},
	"ZAR": {Code: "ZAR", Name: "South African Rand", Exponent: 2},
}

func ValidCurrency(code string) bool {
	_, ok := Currencies[code]
	return ok
}

// ToMinorUnits converts a decimal string like "10.50" to 1050 for USD.
func ToMinorUnits(amount string, currency string) (int64, error) {
	cur, ok := Currencies[currency]
	if !ok {
		return 0, fmt.Errorf("%w: %s", ErrInvalidCurrency, currency)
	}

	amount = strings.TrimSpace(amount)
	negative := false
	if strings.HasPrefix(amount, "-") {
		negative = true
		amount = amount[1:]
	} else if strings.HasPrefix(amount, "+") {
		amount = amount[1:]
	}

	parts := strings.SplitN(amount, ".", 2)
	wholePart := parts[0]
	fracPart := ""
	if len(parts) == 2 {
		fracPart = parts[1]
	}

	whole, err := strconv.ParseInt(wholePart, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid amount: %w", err)
	}

	multiplier := int64(math.Pow10(cur.Exponent))
	result := whole * multiplier

	if cur.Exponent > 0 && fracPart != "" {
		// Pad or truncate fractional part to match exponent
		for len(fracPart) < cur.Exponent {
			fracPart += "0"
		}
		fracPart = fracPart[:cur.Exponent]
		frac, err := strconv.ParseInt(fracPart, 10, 64)
		if err != nil {
			return 0, fmt.Errorf("invalid fractional amount: %w", err)
		}
		result += frac
	}

	if negative {
		result = -result
	}
	return result, nil
}

// FormatAmount converts minor units to a display string. E.g. 1050 USD -> "10.50".
func FormatAmount(amount int64, currency string) string {
	cur, ok := Currencies[currency]
	if !ok {
		return fmt.Sprintf("%d %s", amount, currency)
	}

	if cur.Exponent == 0 {
		return fmt.Sprintf("%d", amount)
	}

	negative := amount < 0
	if negative {
		amount = -amount
	}

	multiplier := int64(math.Pow10(cur.Exponent))
	whole := amount / multiplier
	frac := amount % multiplier

	sign := ""
	if negative {
		sign = "-"
	}

	format := fmt.Sprintf("%%s%%d.%%0%dd", cur.Exponent)
	return fmt.Sprintf(format, sign, whole, frac)
}

// CurrencyCodes returns a sorted list of supported currency codes.
func CurrencyCodes() []string {
	codes := make([]string, 0, len(Currencies))
	for code := range Currencies {
		codes = append(codes, code)
	}
	// Simple sort
	for i := 0; i < len(codes); i++ {
		for j := i + 1; j < len(codes); j++ {
			if codes[i] > codes[j] {
				codes[i], codes[j] = codes[j], codes[i]
			}
		}
	}
	return codes
}
