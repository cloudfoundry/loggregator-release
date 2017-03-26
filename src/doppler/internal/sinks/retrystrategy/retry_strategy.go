package retrystrategy

import (
	"math"
	"math/rand"
	"time"
)

type RetryStrategy func(counter int) time.Duration

func Exponential() RetryStrategy {
	exponential := func(counter int) time.Duration {
		if counter == 0 {
			return time.Millisecond
		}
		if counter > 23 {
			counter = 23
		}
		tenthDuration := int(math.Pow(2, float64(counter-1)) * 100)
		duration := tenthDuration * 10
		randomOffset := rand.Intn(tenthDuration*2) - tenthDuration
		return (time.Duration(duration) * time.Microsecond) + (time.Duration(randomOffset) * time.Microsecond)
	}
	return exponential
}

func CappedDouble(start, cap time.Duration) RetryStrategy {
	return func(counter int) time.Duration {
		delay := start * time.Duration(math.Pow(2, float64(counter)))
		if delay > cap {
			return cap
		}
		return delay
	}
}
