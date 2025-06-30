package soc

import (
	"testing"
	"time"

	"github.com/benbjohnson/clock"
	"github.com/evcc-io/evcc/util"
	"github.com/stretchr/testify/assert"
)

func TestSpeedEstimator_DefaultConfig(t *testing.T) {
	config := DefaultChargingSpeedConfig()

	assert.False(t, config.Enabled)
	assert.Equal(t, 80, config.TargetSoc)
	assert.Equal(t, 10*time.Minute, config.MaxPowerWindow)
	assert.Equal(t, 0.15, config.ReductionThreshold)
	assert.Equal(t, 15*time.Minute, config.MinChargingTime)
	assert.Equal(t, 30*time.Second, config.SampleInterval)
	assert.Equal(t, 2*time.Hour, config.HistoryRetention)
	assert.Equal(t, 5*time.Minute, config.StabilityWindow)
	assert.Equal(t, 1000.0, config.MinPowerForEstimation)
}

func TestSpeedEstimator_NewEstimator(t *testing.T) {
	log := util.NewLogger("test")
	config := DefaultChargingSpeedConfig()
	config.Enabled = true

	estimator := NewSpeedEstimator(log, config)

	assert.NotNil(t, estimator)
	assert.Equal(t, config.Enabled, estimator.config.Enabled)
	assert.Equal(t, config.TargetSoc, estimator.config.TargetSoc)
	assert.False(t, estimator.IsEstimationActive())
	assert.False(t, estimator.IsTargetReached())
	assert.Equal(t, 0.0, estimator.GetEstimatedSoc())
}

func TestSpeedEstimator_StartStopCharging(t *testing.T) {
	log := util.NewLogger("test")
	config := DefaultChargingSpeedConfig()
	config.Enabled = true

	estimator := NewSpeedEstimator(log, config)
	mockClock := clock.NewMock()
	estimator.clock = mockClock

	// Start charging
	estimator.StartCharging()

	assert.Equal(t, mockClock.Now(), estimator.chargingStarted)
	assert.Empty(t, estimator.powerHistory)
	assert.Equal(t, 0.0, estimator.maxPower)
	assert.False(t, estimator.estimationActive)
	assert.False(t, estimator.targetReached)

	// Stop charging
	estimator.StopCharging()

	assert.False(t, estimator.estimationActive)
	assert.False(t, estimator.targetReached)
}

func TestSpeedEstimator_UpdatePowerDisabled(t *testing.T) {
	log := util.NewLogger("test")
	config := DefaultChargingSpeedConfig()
	config.Enabled = false // Disabled

	estimator := NewSpeedEstimator(log, config)
	estimator.StartCharging()

	// Update power should do nothing when disabled
	estimator.UpdatePower(5000)

	assert.Empty(t, estimator.powerHistory)
	assert.Equal(t, 0.0, estimator.maxPower)
	assert.False(t, estimator.IsEstimationActive())
}

func TestSpeedEstimator_UpdatePowerSampling(t *testing.T) {
	log := util.NewLogger("test")
	config := DefaultChargingSpeedConfig()
	config.Enabled = true
	config.SampleInterval = 30 * time.Second

	estimator := NewSpeedEstimator(log, config)
	mockClock := clock.NewMock()
	estimator.clock = mockClock
	estimator.StartCharging()

	// First update should be recorded
	estimator.UpdatePower(5000)
	assert.Len(t, estimator.powerHistory, 1)
	assert.Equal(t, 5000.0, estimator.maxPower)

	// Second update within sample interval should be ignored
	mockClock.Add(10 * time.Second)
	estimator.UpdatePower(4500)
	assert.Len(t, estimator.powerHistory, 1) // Still only 1 measurement

	// Third update after sample interval should be recorded
	mockClock.Add(25 * time.Second) // Total 35 seconds
	estimator.UpdatePower(4500)
	assert.Len(t, estimator.powerHistory, 2)
}

func TestSpeedEstimator_MaxPowerTracking(t *testing.T) {
	log := util.NewLogger("test")
	config := DefaultChargingSpeedConfig()
	config.Enabled = true
	config.SampleInterval = 1 * time.Second
	config.MaxPowerWindow = 10 * time.Minute

	estimator := NewSpeedEstimator(log, config)
	mockClock := clock.NewMock()
	estimator.clock = mockClock
	estimator.StartCharging()

	// Update with increasing power
	estimator.UpdatePower(3000)
	assert.Equal(t, 3000.0, estimator.maxPower)

	mockClock.Add(2 * time.Second)
	estimator.UpdatePower(5000)
	assert.Equal(t, 5000.0, estimator.maxPower)

	mockClock.Add(2 * time.Second)
	estimator.UpdatePower(4000) // Lower power shouldn't update max
	assert.Equal(t, 5000.0, estimator.maxPower)

	// After max power window, higher power should still update max
	mockClock.Add(12 * time.Minute)
	estimator.UpdatePower(6000)
	assert.Equal(t, 5000.0, estimator.maxPower) // Shouldn't update after window
}

func TestSpeedEstimator_EstimationActivation(t *testing.T) {
	log := util.NewLogger("test")
	config := DefaultChargingSpeedConfig()
	config.Enabled = true
	config.SampleInterval = 1 * time.Second
	config.MinChargingTime = 15 * time.Minute
	config.ReductionThreshold = 0.15 // 15% reduction
	config.MinPowerForEstimation = 1000
	config.StabilityWindow = 5 * time.Minute

	estimator := NewSpeedEstimator(log, config)
	mockClock := clock.NewMock()
	estimator.clock = mockClock
	estimator.StartCharging()

	// Build up max power
	estimator.UpdatePower(5000)
	mockClock.Add(2 * time.Second)
	estimator.UpdatePower(5200)

	// Not enough time passed
	mockClock.Add(5 * time.Minute)
	estimator.UpdatePower(4000) // 23% reduction
	assert.False(t, estimator.IsEstimationActive())

	// Enough time passed but need stable reduction
	mockClock.Add(12 * time.Minute) // Total 17 minutes
	estimator.UpdatePower(4000)
	assert.False(t, estimator.IsEstimationActive()) // Need stability window

	// Add more measurements for stability
	for i := 0; i < 6; i++ {
		mockClock.Add(1 * time.Minute)
		estimator.UpdatePower(4000 - float64(i*10)) // Gradually decreasing
	}

	assert.True(t, estimator.IsEstimationActive())
}

func TestSpeedEstimator_SocEstimation(t *testing.T) {
	log := util.NewLogger("test")
	config := DefaultChargingSpeedConfig()
	config.Enabled = true
	config.SampleInterval = 1 * time.Second
	config.MinChargingTime = 1 * time.Minute // Short for testing
	config.ReductionThreshold = 0.15
	config.MinPowerForEstimation = 1000
	config.StabilityWindow = 1 * time.Minute // Short for testing
	config.TargetSoc = 80

	estimator := NewSpeedEstimator(log, config)
	mockClock := clock.NewMock()
	estimator.clock = mockClock
	estimator.StartCharging()

	// Build up max power
	estimator.UpdatePower(5000)

	// Wait for minimum charging time
	mockClock.Add(2 * time.Minute)

	// Add stable reduced power measurements over the stability window
	reducedPower := 4000.0 // 20% reduction from 5000W
	for i := 0; i < 5; i++ {
		estimator.UpdatePower(reducedPower)
		mockClock.Add(20 * time.Second) // Spread over stability window
	}

	assert.True(t, estimator.IsEstimationActive())

	// Continue with further power reduction
	mockClock.Add(1 * time.Second)
	estimator.UpdatePower(3500) // 30% reduction

	estimatedSoc := estimator.GetEstimatedSoc()
	assert.Greater(t, estimatedSoc, 70.0) // Should be above base
	assert.Less(t, estimatedSoc, 100.0)   // Should be reasonable

	// Reduce power enough to trigger target
	mockClock.Add(1 * time.Second)
	estimator.UpdatePower(3000) // 40% reduction

	// Should reach target
	assert.True(t, estimator.IsTargetReached())
	assert.GreaterOrEqual(t, estimator.GetEstimatedSoc(), float64(config.TargetSoc))
}

func TestSpeedEstimator_HistoryCleanup(t *testing.T) {
	log := util.NewLogger("test")
	config := DefaultChargingSpeedConfig()
	config.Enabled = true
	config.SampleInterval = 1 * time.Second
	config.HistoryRetention = 10 * time.Minute

	estimator := NewSpeedEstimator(log, config)
	mockClock := clock.NewMock()
	estimator.clock = mockClock
	estimator.StartCharging()

	// Add measurements over time
	for i := 0; i < 20; i++ {
		estimator.UpdatePower(5000)
		mockClock.Add(1 * time.Minute)
	}

	// Should have cleaned up old measurements
	assert.LessOrEqual(t, len(estimator.powerHistory), 11) // 10 minutes + current
}

func TestSpeedEstimator_GetStatus(t *testing.T) {
	log := util.NewLogger("test")
	config := DefaultChargingSpeedConfig()
	config.Enabled = true
	config.TargetSoc = 85

	estimator := NewSpeedEstimator(log, config)
	estimator.StartCharging()
	estimator.UpdatePower(5000)

	status := estimator.GetStatus()

	assert.Equal(t, true, status["enabled"])
	assert.Equal(t, false, status["estimationActive"])
	assert.Equal(t, 0.0, status["estimatedSoc"])
	assert.Equal(t, 85, status["targetSoc"])
	assert.Equal(t, false, status["targetReached"])
	assert.Equal(t, 5000.0, status["maxPower"])
	assert.Equal(t, 1, status["measurementCount"])
	assert.NotEmpty(t, status["chargingDuration"])
}
