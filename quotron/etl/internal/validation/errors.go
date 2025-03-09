package validation

import "errors"

// Validation errors
var (
	ErrUnreasonablePrice   = errors.New("unreasonable price value")
	ErrUnreasonableVolume  = errors.New("unreasonable volume value")
	ErrOutdatedTimestamp   = errors.New("outdated timestamp")
	ErrMissingRequiredField = errors.New("missing required field")
	ErrInvalidDataFormat   = errors.New("invalid data format")
	ErrInvalidSymbol       = errors.New("invalid stock symbol")
	ErrInvalidExchange     = errors.New("invalid exchange value")
	ErrInvalidMarketIndex  = errors.New("invalid market index value")
	ErrInvalidBatchID      = errors.New("invalid batch ID")
)