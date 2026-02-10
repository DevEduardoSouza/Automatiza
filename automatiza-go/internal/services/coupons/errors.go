package coupons

import (
	"errors"
	"fmt"
)

// DuplicateCodeError indica que já existe um cupom com o mesmo código
type DuplicateCodeError struct {
	Code string
}

func (e *DuplicateCodeError) Error() string {
	return fmt.Sprintf("já existe um cupom com o código '%s'", e.Code)
}

// IsDuplicateCodeError checks if the error is a DuplicateCodeError
func IsDuplicateCodeError(err error) bool {
	var dupErr *DuplicateCodeError
	return errors.As(err, &dupErr)
}

// CouponNotFoundError indica que o cupom não foi encontrado
type CouponNotFoundError struct {
	ID string
}

func (e *CouponNotFoundError) Error() string {
	return fmt.Sprintf("cupom não encontrado: %s", e.ID)
}

// IsCouponNotFoundError checks if the error is a CouponNotFoundError
func IsCouponNotFoundError(err error) bool {
	_, ok := err.(*CouponNotFoundError)
	return ok
}
