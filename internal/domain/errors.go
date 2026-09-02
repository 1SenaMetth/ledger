package domain

import "errors"

// Sentinel errors for the ledger domain. Callers compare with errors.Is.
// The HTTP layer maps these to status codes; the domain never knows about HTTP.
var (
	ErrCurrencyMismatch      = errors.New("currency mismatch")
	ErrInvalidCurrency       = errors.New("invalid currency")
	ErrOverflow              = errors.New("monetary amount overflow")
	ErrNonPositiveAmount     = errors.New("amount must be positive")
	ErrInsufficientFunds     = errors.New("insufficient funds")
	ErrUnbalancedTransaction = errors.New("transaction entries do not sum to zero")
	ErrSameAccount           = errors.New("source and destination accounts are identical")
	ErrAccountNotFound       = errors.New("account not found")
	ErrAccountClosed         = errors.New("account is closed")
	ErrInsufficientEntries   = errors.New("transaction must have at least two entries")
)
