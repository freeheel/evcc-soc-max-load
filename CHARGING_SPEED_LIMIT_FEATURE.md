# Charging Speed-Based SoC Limiting Feature

## Overview

This feature implements automatic charging limitation based on charging speed reduction patterns for vehicles that don't natively report their state of charge (SoC). The system monitors charging power over time and estimates when the vehicle has reached a target charge level (e.g., 80%) by detecting the natural power reduction that occurs as EV batteries approach full charge.

## Implementation Summary

### Core Components

1. **SpeedEstimator** (`core/soc/speedestimator.go`)
   - Monitors charging power over time
   - Detects power reduction patterns
   - Estimates SoC based on charging speed curves
   - Determines when target SoC is reached

2. **Configuration Integration** (`vehicle/embed.go`)
   - Added `chargingSpeedLimit` configuration section
   - Supports all necessary parameters for fine-tuning

3. **Loadpoint Integration** (`core/loadpoint.go`, `core/loadpoint_vehicle.go`)
   - Integrated speed estimator with charging lifecycle
   - Added power monitoring and SoC limit checking
   - Published status information for UI display

4. **UI Keys** (`core/keys/loadpoint.go`)
   - Added keys for publishing speed estimator status
   - Enables UI display of estimation progress

### Key Features

- **Configurable Parameters**: All aspects of the algorithm can be tuned
- **Safety Mechanisms**: Multiple checks prevent false triggers
- **Stability Verification**: Ensures power reduction is consistent
- **History Management**: Automatic cleanup of old measurements
- **Status Monitoring**: Real-time status available for UI and debugging

### Configuration Example

```yaml
vehicles:
  - name: my_ev
    type: template
    template: generic
    capacity: 75

    chargingSpeedLimit:
      enabled: true              # Enable the feature
      targetSoc: 80              # Target SoC percentage
      maxPowerWindow: "10m"      # Time to determine max power
      reductionThreshold: 0.15   # 15% power reduction threshold
      minChargingTime: "15m"     # Minimum time before estimation
      sampleInterval: "30s"      # Power sampling frequency
      historyRetention: "2h"     # Data retention period
      stabilityWindow: "5m"      # Stability verification window
      minPowerForEstimation: 1000 # Minimum power for estimation (W)
```

### Algorithm Logic

1. **Power Monitoring**: Continuously samples charging power at configured intervals
2. **Max Power Detection**: Records maximum power during initial charging phase
3. **Reduction Detection**: Identifies when power drops significantly below maximum
4. **Stability Check**: Verifies power reduction is stable over time window
5. **SoC Estimation**: Maps power reduction to estimated SoC using linear model
6. **Target Detection**: Stops charging when estimated SoC reaches target

### Safety Features

- **Minimum Charging Time**: Prevents premature activation
- **Power Threshold**: Requires minimum power level for reliable estimation
- **Stability Window**: Ensures consistent power reduction before activation
- **Conservative Estimation**: Algorithm errs on side of caution

### Testing

Comprehensive test suite (`core/soc/speedestimator_test.go`) covers:
- Configuration validation
- Power monitoring and sampling
- Max power tracking
- Estimation activation logic
- SoC calculation accuracy
- History management
- Status reporting

All tests pass successfully, validating the implementation.

### Files Modified/Created

**New Files:**
- `core/soc/speedestimator.go` - Core speed estimator implementation
- `core/soc/speedestimator_test.go` - Comprehensive test suite
- `CHARGING_SPEED_LIMIT_FEATURE.md` - This documentation

**Modified Files:**
- `vehicle/embed.go` - Added configuration structure
- `core/loadpoint.go` - Added speed estimator field and publishing
- `core/loadpoint_vehicle.go` - Added initialization and integration
- `core/keys/loadpoint.go` - Added UI keys for status publishing
- `assets/js/components/Config/defaultYaml/vehicle.yaml` - Added config example

### Usage Instructions

1. **Enable the Feature**: Add `chargingSpeedLimit` section to vehicle configuration
2. **Configure Parameters**: Adjust settings based on your vehicle's charging characteristics
3. **Monitor Initial Sessions**: Watch first few charging sessions to verify operation
4. **Fine-tune Settings**: Adjust parameters based on observed behavior

### Limitations

- **Vehicle Dependency**: Effectiveness depends on vehicle's charging curve characteristics
- **Environmental Factors**: Temperature and battery condition can affect accuracy
- **AC Charging Preferred**: Works best with AC charging due to predictable power curves
- **Learning Period**: May require several sessions to optimize for specific vehicle

### Future Enhancements

Potential improvements for future versions:
- **Machine Learning**: Adaptive algorithms that learn vehicle-specific patterns
- **Temperature Compensation**: Adjust estimation based on ambient temperature
- **Multiple Curves**: Support for different charging curves based on conditions
- **Calibration Mode**: Guided setup to optimize parameters for specific vehicles

## Conclusion

This implementation provides a robust solution for charging limitation when vehicle SoC data isn't available. The feature is designed with safety and configurability in mind, allowing users to adapt it to their specific vehicle and charging setup while maintaining reliable operation.