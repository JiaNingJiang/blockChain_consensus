package utils

import "errors"

var (
	MSGENOUGH = errors.New("The number of consensus messages at the current stage has reached the requirements")
)
