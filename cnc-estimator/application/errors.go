package application

import "fmt"

type ValidationError struct {
	Message string
	Cause   error
}

func (err *ValidationError) Error() string {
	if err.Cause == nil {
		return err.Message
	}
	return fmt.Sprintf("%s: %v", err.Message, err.Cause)
}

func (err *ValidationError) Unwrap() error {
	return err.Cause
}

func Invalid(message string, cause error) error {
	return &ValidationError{Message: message, Cause: cause}
}
