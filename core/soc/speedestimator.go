package soc

import (
	"math"
	"sync"
	"time"

	"github.com/benbjohnson/clock"
	"github.com/evcc-io/evcc/util"
)

// PowerMeasurement represents a single power measurement with timestamp
type PowerMeasurement struct {
	Timestamp time.Time
	Power     float64 // Watts
}

// ChargingSpeedConfig holds configuration for charging speed-based SoC estimation
type ChargingSpeedConfig struct {
	Enabled               bool          `mapstructure:"enabled"`               // Enable charging speed-based SoC estimation
	TargetSoc             int           `mapstructure:"targetSoc"`             // Target SoC percentage (e.g., 80)
	MaxPowerWindow        time.Duration `mapstructure:"maxPowerWindow"`        // Time window to determine max charging power (default: 10min)
	ReductionThreshold    float64       `mapstructure:"reductionThreshold"`    // Power reduction threshold to trigger SoC estimation (default: 0.15 = 15%)
	MinChargingTime       time.Duration `mapstructure:"minChargingTime"`       // Minimum charging time before estimation starts (default: 15min)
	SampleInterval        time.Duration `mapstructure:"sampleInterval"`        // How often to sample power (default: 30s)
	HistoryRetention      time.Duration `mapstructure:"historyRetention"`      // How long to keep power history (default: 2h)
	StabilityWindow       time.Duration `mapstructure:"stabilityWindow"`       // Window to check for stable power reduction (default: 5min)
	MinPowerForEstimation float64       `mapstructure:"minPowerForEstimation"` // Minimum power to consider for estimation (default: 1000W)
}

// DefaultChargingSpeedConfig returns default configuration
func DefaultChargingSpeedConfig() ChargingSpeedConfig {
	return ChargingSpeedConfig{
		Enabled:               false,
		TargetSoc:             80,
		MaxPowerWindow:        10 * time.Minute,
		ReductionThreshold:    0.15, // 15% reduction
		MinChargingTime:       15 * time.Minute,
		SampleInterval:        30 * time.Second,
		HistoryRetention:      2 * time.Hour,
		StabilityWindow:       5 * time.Minute,
		MinPowerForEstimation: 1000, // 1kW minimum
	}
}

// SpeedEstimator estimates SoC based on charging speed reduction patterns
type SpeedEstimator struct {
	sync.RWMutex
	log    *util.Logger
	clock  clock.Clock
	config ChargingSpeedConfig

	// Power history tracking
	powerHistory    []PowerMeasurement
	maxPower        float64   // Maximum observed charging power
	maxPowerTime    time.Time // When max power was observed
	chargingStarted time.Time // When current charging session started

	// Estimation state
	estimationActive bool      // Whether SoC estimation is currently active
	estimatedSoc     float64   // Current estimated SoC
	targetReached    bool      // Whether target SoC has been reached
	lastSample       time.Time // Last time we sampled power
}

// NewSpeedEstimator creates a new charging speed-based SoC estimator
func NewSpeedEstimator(log *util.Logger, config ChargingSpeedConfig) *SpeedEstimator {
	if config.SampleInterval == 0 {
		config = DefaultChargingSpeedConfig()
	}

	return &SpeedEstimator{
		log:    log,
		clock:  clock.New(),
		config: config,
	}
}

// StartCharging initializes the estimator for a new charging session
func (se *SpeedEstimator) StartCharging() {
	se.Lock()
	defer se.Unlock()

	se.chargingStarted = se.clock.Now()
	se.powerHistory = nil
	se.maxPower = 0
	se.maxPowerTime = time.Time{}
	se.estimationActive = false
	se.estimatedSoc = 0
	se.targetReached = false
	se.lastSample = time.Time{}

	se.log.DEBUG.Println("speed estimator: charging session started")
}

// StopCharging stops the estimation and clears state
func (se *SpeedEstimator) StopCharging() {
	se.Lock()
	defer se.Unlock()

	se.estimationActive = false
	se.targetReached = false
	se.log.DEBUG.Println("speed estimator: charging session stopped")
}

// UpdatePower adds a new power measurement and updates SoC estimation
func (se *SpeedEstimator) UpdatePower(power float64) {
	if !se.config.Enabled {
		return
	}

	se.Lock()
	defer se.Unlock()

	now := se.clock.Now()

	// Check if we should sample (rate limiting)
	if !se.lastSample.IsZero() && now.Sub(se.lastSample) < se.config.SampleInterval {
		return
	}
	se.lastSample = now

	// Add measurement to history
	measurement := PowerMeasurement{
		Timestamp: now,
		Power:     power,
	}
	se.powerHistory = append(se.powerHistory, measurement)

	// Clean old measurements
	se.cleanOldMeasurements()

	// Update max power if this is higher and within the max power window
	if power > se.maxPower && (se.maxPowerTime.IsZero() || now.Sub(se.chargingStarted) <= se.config.MaxPowerWindow) {
		se.maxPower = power
		se.maxPowerTime = now
		se.log.DEBUG.Printf("speed estimator: new max power %.0fW", power)
	}

	// Check if we can start estimation
	if !se.estimationActive && se.canStartEstimation(now, power) {
		se.estimationActive = true
		se.log.INFO.Printf("speed estimator: starting SoC estimation (max power: %.0fW, current: %.0fW)", se.maxPower, power)
	}

	// Update SoC estimation if active
	if se.estimationActive {
		se.updateSocEstimation(power)
	}
}

// canStartEstimation checks if conditions are met to start SoC estimation
func (se *SpeedEstimator) canStartEstimation(now time.Time, currentPower float64) bool {
	// Must have been charging for minimum time
	if now.Sub(se.chargingStarted) < se.config.MinChargingTime {
		return false
	}

	// Must have observed significant max power
	if se.maxPower < se.config.MinPowerForEstimation {
		return false
	}

	// Current power must show significant reduction from max
	powerReduction := (se.maxPower - currentPower) / se.maxPower
	if powerReduction < se.config.ReductionThreshold {
		return false
	}

	// Check for stable power reduction over stability window
	return se.hasStablePowerReduction(now, currentPower)
}

// hasStablePowerReduction checks if power has been consistently reduced over the stability window
func (se *SpeedEstimator) hasStablePowerReduction(now time.Time, _ float64) bool {
	stabilityStart := now.Add(-se.config.StabilityWindow)

	var recentMeasurements []PowerMeasurement
	for _, m := range se.powerHistory {
		if m.Timestamp.After(stabilityStart) {
			recentMeasurements = append(recentMeasurements, m)
		}
	}

	if len(recentMeasurements) < 3 {
		return false
	}

	// Check that power has been consistently below threshold
	threshold := se.maxPower * (1 - se.config.ReductionThreshold)
	for _, m := range recentMeasurements {
		if m.Power > threshold {
			return false
		}
	}

	return true
}

// updateSocEstimation calculates estimated SoC based on power reduction curve
func (se *SpeedEstimator) updateSocEstimation(currentPower float64) {
	// Simple linear estimation based on power reduction
	// This is a basic implementation - could be enhanced with more sophisticated curves
	powerReduction := (se.maxPower - currentPower) / se.maxPower

	// Assume linear relationship between power reduction and SoC increase
	// This is vehicle-specific and could be calibrated
	socIncrease := powerReduction * 30 // Assume 30% SoC range where power reduces significantly

	// Estimate current SoC (assuming we started estimation around 70% and target is 80%)
	baseSoc := float64(se.config.TargetSoc) - 10 // Start estimation 10% before target
	se.estimatedSoc = baseSoc + socIncrease

	// Clamp to reasonable range
	se.estimatedSoc = math.Max(0, math.Min(100, se.estimatedSoc))

	// Check if target is reached
	if se.estimatedSoc >= float64(se.config.TargetSoc) && !se.targetReached {
		se.targetReached = true
		se.log.INFO.Printf("speed estimator: target SoC %d%% reached (estimated: %.1f%%)", se.config.TargetSoc, se.estimatedSoc)
	}

	se.log.DEBUG.Printf("speed estimator: power %.0fW (%.1f%% of max), estimated SoC: %.1f%%",
		currentPower, (currentPower/se.maxPower)*100, se.estimatedSoc)
}

// cleanOldMeasurements removes measurements older than retention period
func (se *SpeedEstimator) cleanOldMeasurements() {
	cutoff := se.clock.Now().Add(-se.config.HistoryRetention)

	var filtered []PowerMeasurement
	for _, m := range se.powerHistory {
		if m.Timestamp.After(cutoff) {
			filtered = append(filtered, m)
		}
	}
	se.powerHistory = filtered
}

// IsTargetReached returns true if the target SoC has been reached
func (se *SpeedEstimator) IsTargetReached() bool {
	se.RLock()
	defer se.RUnlock()
	return se.targetReached
}

// GetEstimatedSoc returns the current estimated SoC
func (se *SpeedEstimator) GetEstimatedSoc() float64 {
	se.RLock()
	defer se.RUnlock()
	return se.estimatedSoc
}

// IsEstimationActive returns true if SoC estimation is currently active
func (se *SpeedEstimator) IsEstimationActive() bool {
	se.RLock()
	defer se.RUnlock()
	return se.estimationActive
}

// GetStatus returns current status information for debugging/UI
func (se *SpeedEstimator) GetStatus() map[string]interface{} {
	se.RLock()
	defer se.RUnlock()

	return map[string]interface{}{
		"enabled":          se.config.Enabled,
		"estimationActive": se.estimationActive,
		"estimatedSoc":     se.estimatedSoc,
		"targetSoc":        se.config.TargetSoc,
		"targetReached":    se.targetReached,
		"maxPower":         se.maxPower,
		"measurementCount": len(se.powerHistory),
		"chargingDuration": se.clock.Now().Sub(se.chargingStarted).String(),
	}
}
