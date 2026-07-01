package accounting

import "errors"

// ErrBudgetExceeded is returned by BudgetChecker.Check when the tenant has
// exceeded a configured hard-stop limit.
var ErrBudgetExceeded = errors.New("accounting: budget exceeded")

// ErrUsageNotFound is returned when a requested usage summary does not exist.
var ErrUsageNotFound = errors.New("accounting: usage not found")

// ErrNegativeTokens is returned when a UsageRecord contains a negative token
// count, which indicates a programming error in the caller.
var ErrNegativeTokens = errors.New("accounting: negative token count")

// ErrInvalidPeriod is returned when a period string is not one of the
// recognised values ("daily", "monthly").
var ErrInvalidPeriod = errors.New("accounting: invalid period; must be 'daily' or 'monthly'")
