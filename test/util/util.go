package util

import (
	"time"
)

var (
	// Timeout - timeout which can be used for instance in waiting operation
	Timeout = time.Second * 30
	// ShortTimeout - half ot Timeout - can be used for instance in waiting operation
	ShortTimeout = time.Second * 15
	// RetryInterval - retry interval time
	RetryInterval = time.Second * 1
)
