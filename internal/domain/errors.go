package domain

import "errors"

var (
	ErrNotFound               = errors.New("record not found")
	ErrUnauthorized           = errors.New("unauthorized")
	ErrForbidden              = errors.New("forbidden")
	ErrBadRequest             = errors.New("bad request")
	ErrConflict               = errors.New("conflict: duplicate record")
	ErrOptimisticLock         = errors.New("optimistic locking conflict")
	ErrInsufficientPoints     = errors.New("insufficient points balance")
	ErrOutOfStock             = errors.New("voucher out of stock")
	ErrMemberInactive         = errors.New("member account is not active")
	ErrEarningCapExceeded     = errors.New("earning cap exceeded")
	ErrRateLimitExceeded      = errors.New("rate limit exceeded")
	ErrMissingIdempotencyKey  = errors.New("missing or invalid idempotency key")
	ErrDuplicateIdempotencyKey= errors.New("duplicate idempotency key")
	ErrInternal               = errors.New("internal server error")
)

