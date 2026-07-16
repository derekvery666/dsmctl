package synology

import (
	"errors"
	"fmt"
)

type APIError struct {
	API    string
	Method string
	Code   int
}

func (e *APIError) Error() string {
	return fmt.Sprintf("Synology API %s.%s failed with code %d", e.API, e.Method, e.Code)
}

func isSessionError(err error) bool {
	var apiErr *APIError
	return errors.As(err, &apiErr) && (apiErr.Code == 106 || apiErr.Code == 119)
}
