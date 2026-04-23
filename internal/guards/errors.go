package guards

import "errors"

var (
	ErrAuthRequired = errors.New("authentication required")
	ErrForbidden    = errors.New("insufficient permissions")
)
