package validation

import (
	"errors"
	"fmt"

	"encore.dev/beta/errs"
	"github.com/go-playground/validator/v10"
)

// validate holds the singleton validator instance, for input structure validation.
var validate = validator.New(validator.WithRequiredStructEnabled())

// Struct validates a struct using the 'validate' tags.
// It returns an Encore-compatible error if validation fails.
func Struct(s any) error {
	if s == nil {
		return nil
	}
	if err := validate.Struct(s); err != nil {
		// Safely check if it's a ValidationErrors
		var validationErrors validator.ValidationErrors
		if errors.As(err, &validationErrors) && len(validationErrors) > 0 {
			// Create a user-friendly error message with specific field and rule
			firstErr := validationErrors[0]
			msg := fmt.Sprintf("Validation failed for field '%s' with rule '%s'", firstErr.Field(), firstErr.Tag())

			return &errs.Error{
				Code:    errs.InvalidArgument,
				Message: msg,
			}
		}

		// Fallback for unexpected error types - log the actual error for debugging
		// but return a generic validation error to the client
		return &errs.Error{
			Code:    errs.InvalidArgument,
			Message: fmt.Sprintf("Validation failed: %v", err),
		}
	}

	return nil
}
