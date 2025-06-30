package vehicle

import (
	"github.com/evcc-io/evcc/api"
)

// TODO align phases with OnIdentify
type embed struct {
	Title_       string           `mapstructure:"title"`
	Icon_        string           `mapstructure:"icon"`
	Capacity_    float64          `mapstructure:"capacity"`
	Phases_      int              `mapstructure:"phases"`
	Identifiers_ []string         `mapstructure:"identifiers"`
	Features_    []api.Feature    `mapstructure:"features"`
	OnIdentify   api.ActionConfig `mapstructure:"onIdentify"`

	// Charging speed-based SoC estimation configuration
	ChargingSpeedLimit struct {
		Enabled               bool    `mapstructure:"enabled"`               // Enable charging speed-based SoC estimation
		TargetSoc             int     `mapstructure:"targetSoc"`             // Target SoC percentage (e.g., 80)
		MaxPowerWindow        string  `mapstructure:"maxPowerWindow"`        // Time window to determine max charging power (default: "10m")
		ReductionThreshold    float64 `mapstructure:"reductionThreshold"`    // Power reduction threshold to trigger SoC estimation (default: 0.15 = 15%)
		MinChargingTime       string  `mapstructure:"minChargingTime"`       // Minimum charging time before estimation starts (default: "15m")
		SampleInterval        string  `mapstructure:"sampleInterval"`        // How often to sample power (default: "30s")
		HistoryRetention      string  `mapstructure:"historyRetention"`      // How long to keep power history (default: "2h")
		StabilityWindow       string  `mapstructure:"stabilityWindow"`       // Window to check for stable power reduction (default: "5m")
		MinPowerForEstimation float64 `mapstructure:"minPowerForEstimation"` // Minimum power to consider for estimation (default: 1000W)
	} `mapstructure:"chargingSpeedLimit"`
}

// Title implements the api.Vehicle interface
func (v *embed) fromVehicle(title string, capacity float64) {
	if v.Title_ == "" {
		v.Title_ = title
	}
	if v.Capacity_ == 0 {
		v.Capacity_ = capacity
	}
}

// GetTitle implements the api.Vehicle interface
func (v *embed) GetTitle() string {
	return v.Title_
}

// SetTitle implements the api.TitleSetter interface
func (v *embed) SetTitle(title string) {
	v.Title_ = title
}

// Capacity implements the api.Vehicle interface
func (v *embed) Capacity() float64 {
	return v.Capacity_
}

var _ api.PhaseDescriber = (*embed)(nil)

// Phases returns the phases used by the vehicle
func (v *embed) Phases() int {
	return v.Phases_
}

// Identifiers implements the api.Identifier interface
func (v *embed) Identifiers() []string {
	return v.Identifiers_
}

// OnIdentified returns the identify action
func (v *embed) OnIdentified() api.ActionConfig {
	return v.OnIdentify
}

var _ api.IconDescriber = (*embed)(nil)

// Icon implements the api.IconDescriber interface
func (v *embed) Icon() string {
	return v.Icon_
}

var _ api.FeatureDescriber = (*embed)(nil)

// Features implements the api.FeatureDescriber interface
func (v *embed) Features() []api.Feature {
	return v.Features_
}

// GetChargingSpeedLimitConfig returns the charging speed limit configuration
func (v *embed) GetChargingSpeedLimitConfig() map[string]interface{} {
	config := make(map[string]interface{})
	config["enabled"] = v.ChargingSpeedLimit.Enabled
	config["targetSoc"] = v.ChargingSpeedLimit.TargetSoc
	config["maxPowerWindow"] = v.ChargingSpeedLimit.MaxPowerWindow
	config["reductionThreshold"] = v.ChargingSpeedLimit.ReductionThreshold
	config["minChargingTime"] = v.ChargingSpeedLimit.MinChargingTime
	config["sampleInterval"] = v.ChargingSpeedLimit.SampleInterval
	config["historyRetention"] = v.ChargingSpeedLimit.HistoryRetention
	config["stabilityWindow"] = v.ChargingSpeedLimit.StabilityWindow
	config["minPowerForEstimation"] = v.ChargingSpeedLimit.MinPowerForEstimation
	return config
}
