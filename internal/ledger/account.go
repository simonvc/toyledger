package ledger

import (
	"fmt"
	"regexp"
	"strings"
	"time"
)

type Category string

const (
	CategoryAssets      Category = "assets"
	CategoryLiabilities Category = "liabilities"
	CategoryEquity      Category = "equity"
	CategoryRevenue     Category = "revenue"
	CategoryExpenses    Category = "expenses"
)

var AllCategories = []Category{
	CategoryAssets,
	CategoryLiabilities,
	CategoryEquity,
	CategoryRevenue,
	CategoryExpenses,
}

type Account struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Code      int       `json:"code"`
	Category  Category  `json:"category"`
	Currency  string    `json:"currency"`
	IsSystem  bool      `json:"is_system"`
	CreatedAt time.Time `json:"created_at"`
}

// CategoryForCode derives the IFRS category from a 4-digit code.
func CategoryForCode(code int) (Category, error) {
	switch {
	case code >= 1000 && code < 2000:
		return CategoryAssets, nil
	case code >= 2000 && code < 3000:
		return CategoryLiabilities, nil
	case code >= 3000 && code < 4000:
		return CategoryEquity, nil
	case code >= 4000 && code < 5000:
		return CategoryRevenue, nil
	case code >= 5000 && code < 6000:
		return CategoryExpenses, nil
	default:
		return "", fmt.Errorf("%w: %d (must be 1000-5999)", ErrInvalidAccountCode, code)
	}
}

// CodeRange returns the valid code range for a category.
func CodeRange(cat Category) (int, int) {
	switch cat {
	case CategoryAssets:
		return 1000, 1999
	case CategoryLiabilities:
		return 2000, 2999
	case CategoryEquity:
		return 3000, 3999
	case CategoryRevenue:
		return 4000, 4999
	case CategoryExpenses:
		return 5000, 5999
	default:
		return 0, 0
	}
}

// CategoryLabel returns a human-readable label for a category.
func CategoryLabel(cat Category) string {
	switch cat {
	case CategoryAssets:
		return "Assets"
	case CategoryLiabilities:
		return "Liabilities"
	case CategoryEquity:
		return "Equity"
	case CategoryRevenue:
		return "Revenue"
	case CategoryExpenses:
		return "Expenses"
	default:
		return string(cat)
	}
}

var (
	nostroPattern = regexp.MustCompile(`^<[a-zA-Z0-9_-]+:[a-zA-Z]{3}>$`)
	vostroPattern = regexp.MustCompile(`^>[a-zA-Z0-9_-]+:[a-zA-Z]{3}<$`)
)

// ValidateCorrespondentID checks that accounts at codes 1010 (nostro) and
// 2010 (vostro) follow the directional arrow naming convention.
func ValidateCorrespondentID(code int, id string) error {
	switch code {
	case 1010:
		if !nostroPattern.MatchString(id) {
			return fmt.Errorf("%w: nostro accounts (1010) must use <bank:ccy> format, e.g. <jpmorgan:usd>", ErrInvalidCorrespondentID)
		}
	case 2010:
		if !vostroPattern.MatchString(id) {
			return fmt.Errorf("%w: vostro accounts (2010) must use >bank:ccy< format, e.g. >jpmorgan:usd<", ErrInvalidCorrespondentID)
		}
	}
	return nil
}

// Validate checks all account invariants.
func (a *Account) Validate() error {
	if a.ID == "" {
		return ErrInvalidAccountID
	}

	isSystemID := strings.HasPrefix(a.ID, "~")

	if a.IsSystem && !isSystemID {
		return ErrSystemAccountPrefix
	}
	if !a.IsSystem && isSystemID {
		return ErrNonSystemAccountTilde
	}

	// System accounts don't need standard IFRS code validation
	if a.IsSystem {
		if a.Name == "" {
			return fmt.Errorf("account name is required")
		}
		return nil
	}

	if a.Code < 1000 || a.Code > 5999 {
		return fmt.Errorf("%w: %d", ErrInvalidAccountCode, a.Code)
	}

	expectedCat, err := CategoryForCode(a.Code)
	if err != nil {
		return err
	}
	if a.Category != expectedCat {
		return fmt.Errorf("%w: code %d should be %s, got %s", ErrCodeCategoryMismatch, a.Code, expectedCat, a.Category)
	}

	if err := ValidateCorrespondentID(a.Code, a.ID); err != nil {
		return err
	}

	if a.Currency != "*" && !ValidCurrency(a.Currency) {
		return fmt.Errorf("%w: %s", ErrInvalidCurrency, a.Currency)
	}

	if a.Name == "" {
		return fmt.Errorf("account name is required")
	}

	return nil
}

// NormalBalance returns "Debit" or "Credit" for the account's category.
// Assets and Expenses are debit-normal; Liabilities, Equity, and Revenue are credit-normal.
func NormalBalance(cat Category) string {
	switch cat {
	case CategoryAssets, CategoryExpenses:
		return "Debit"
	default:
		return "Credit"
	}
}

// ValidCategory checks if a category string is valid.
func ValidCategory(cat Category) bool {
	for _, c := range AllCategories {
		if c == cat {
			return true
		}
	}
	return false
}
