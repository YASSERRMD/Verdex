package scalability

import (
	"errors"
	"fmt"
)

// Sentinel errors that callers can test with errors.Is.
var (
	// ErrInvalidChecklistAnswers is returned when ChecklistAnswers fails
	// structural validation (see Contract.Verify).
	ErrInvalidChecklistAnswers = errors.New("scalability: invalid checklist answers")

	// ErrEmptyServiceName is returned when a checklist or capacity
	// operation is called with a blank service name.
	ErrEmptyServiceName = errors.New("scalability: service name is required")

	// ErrInvalidPartitionCount is returned when a Partitioner is
	// constructed with a non-positive partition count.
	ErrInvalidPartitionCount = errors.New("scalability: partition count must be positive")

	// ErrInvalidShardCount is returned when a ShardStrategy is
	// constructed with a non-positive shard count.
	ErrInvalidShardCount = errors.New("scalability: shard count must be positive")

	// ErrEmptyShardKey is returned when ShardFor is called with a blank
	// key.
	ErrEmptyShardKey = errors.New("scalability: shard key is required")

	// ErrInvalidScalingPolicy is returned when a ScalingPolicy fails
	// structural validation (see ScalingPolicy.Validate).
	ErrInvalidScalingPolicy = errors.New("scalability: invalid scaling policy")

	// ErrInvalidBackpressureConfig is returned when a
	// BackpressureConfig fails structural validation.
	ErrInvalidBackpressureConfig = errors.New("scalability: invalid backpressure config")

	// ErrLoadShed is returned by BackpressureController.Admit when the
	// configured threshold has been exceeded and the caller must shed
	// (reject) the request.
	ErrLoadShed = errors.New("scalability: load shed, threshold exceeded")

	// ErrControllerClosed is returned when Admit/Release is called on a
	// BackpressureController that has been closed.
	ErrControllerClosed = errors.New("scalability: backpressure controller is closed")

	// ErrInvalidCapacityInput is returned when EstimateReplicas is
	// called with structurally invalid historical throughput or SLA
	// inputs (zero/negative where a positive value is required).
	ErrInvalidCapacityInput = errors.New("scalability: invalid capacity planning input")

	// ErrInvalidScaleTestConfig is returned when a ScaleTestConfig fails
	// structural validation.
	ErrInvalidScaleTestConfig = errors.New("scalability: invalid scale test config")

	// ErrNilOperation is returned when a ScaleTest is run with a nil
	// operation function.
	ErrNilOperation = errors.New("scalability: operation must not be nil")
)

// wrapf mirrors the fmt.Errorf("pkg: fn: %w", err) convention used
// throughout this repository's packages.
func wrapf(fn string, err error) error {
	return fmt.Errorf("scalability: %s: %w", fn, err)
}
