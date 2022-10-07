package providers

import "time"

// Clock abstracts time so that we can mock and test for generated names.
type Clock interface {
	Now() time.Time
}
