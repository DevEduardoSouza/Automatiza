package ctas

import (
	"errors"
	"fmt"
)

// CTANotFoundError indicates that a CTA was not found
type CTANotFoundError struct {
	ID string
}

func (e *CTANotFoundError) Error() string {
	return fmt.Sprintf("CTA não encontrado: %s", e.ID)
}

// IsCTANotFoundError checks if the error is a CTANotFoundError
func IsCTANotFoundError(err error) bool {
	var notFoundErr *CTANotFoundError
	return errors.As(err, &notFoundErr)
}
