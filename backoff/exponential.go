package backoff

import (
	"math"
	"math/rand"
	"time"
)

// ExponentialConfig holds the parameters used by the ExponentialTimer.
type ExponentialConfig struct {
	MaxDuration time.Duration
	GrowthBase  int
	Base        time.Duration
	Random      *rand.Rand
}

// DefaultExponentialConfig creates an ExponentialConfig with default values.
func DefaultExponentialConfig() ExponentialConfig {
	return ExponentialConfig{
		MaxDuration: 120 * time.Second,
		GrowthBase:  2,
		Base:        time.Second,
		Random:      rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

// DefaultExponentialDelayProvider creates a new instance of an ExponentialDelayProvider using the default config.
func DefaultExponentialDelayProvider() DelayProvider {
	return ExponentialDelayProvider(DefaultExponentialConfig())
}

// ExponentialDelayProvider creates a new instance of an ExponentialDelayProvider using the provided config.
func ExponentialDelayProvider(conf ExponentialConfig) DelayProvider {
	return func(numRetries int, _ error) time.Duration {
		newExpDuration := time.Duration(int64(math.Pow(float64(conf.GrowthBase), float64(numRetries))) *
			int64(conf.Base))

		if newExpDuration > conf.MaxDuration {
			newExpDuration = conf.MaxDuration
		}

		return time.Duration(conf.Random.Int63n(int64(newExpDuration)))
	}
}

// ExponentialTimer performs an exponential backoff following the FullJitter implementation described in
// https://aws.amazon.com/blogs/architecture/exponential-backoff-and-jitter/
type ExponentialTimer struct {
	CustomTimer
}

// NewExponentialTimer creates an exponential backoff timer using the default configuration.
func NewExponentialTimer() *ExponentialTimer {
	return NewCustomExponentialTimer(DefaultExponentialConfig())
}

// NewCustomExponentialTimer creates an exponential backoff timer using the provided configuration.
func NewCustomExponentialTimer(conf ExponentialConfig) *ExponentialTimer {
	return &ExponentialTimer{
		CustomTimer: *NewCustomTimer(ExponentialDelayProvider(conf)),
	}
}
