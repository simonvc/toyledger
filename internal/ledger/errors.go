package ledger

import "errors"

var (
	ErrInvalidAccountCode    = errors.New("invalid account code")
	ErrInvalidAccountID      = errors.New("invalid account id")
	ErrInvalidCategory       = errors.New("invalid account category")
	ErrCodeCategoryMismatch  = errors.New("account code does not match category")
	ErrSystemAccountPrefix   = errors.New("system accounts must be prefixed with ~")
	ErrNonSystemAccountTilde = errors.New("non-system accounts cannot start with ~")
	ErrUnbalancedTransaction = errors.New("transaction entries do not balance")
	ErrTooFewEntries         = errors.New("transaction must have at least 2 entries")
	ErrEmptyDescription      = errors.New("transaction description is required")
	ErrInvalidCurrency       = errors.New("invalid or unsupported currency code")
	ErrCurrencyMismatch      = errors.New("entry currency does not match account currency")
	ErrAccountNotFound       = errors.New("account not found")
	ErrTransactionNotFound   = errors.New("transaction not found")
	ErrDuplicateAccount        = errors.New("account already exists")
	ErrInvertedBalance         = errors.New("transaction would create inverted balance")
	ErrEntryDirectionViolation = errors.New("entry violates direction constraint")
)
