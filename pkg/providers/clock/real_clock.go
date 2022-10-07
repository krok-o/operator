package clock

import "time"

type RealClock struct{}

func NewClock() *RealClock {
	return &RealClock{}
}

func (t *RealClock) Now() time.Time {
	return time.Now()
}
