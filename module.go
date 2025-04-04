package powerusagetracker

import (
	"context"
	"fmt"
	"sync"
	"time"

	"go.viam.com/rdk/components/powersensor"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/resource"
)

var (
	EnergyTracker = resource.NewModel("stevebriskin", "power_sensor", "energy_tracker")
)

func init() {
	resource.RegisterComponent(powersensor.API, EnergyTracker,
		resource.Registration[powersensor.PowerSensor, *Config]{
			Constructor: newEnergyTrackerEnergyTracker,
		},
	)
}

type Config struct {
	RefreshRateMSec int    `json:"refresh_rate_msec,omitempty"`
	SourceSensor    string `json:"source_sensor"`
}

// Validate ensures all parts of the config are valid and important fields exist.
// Returns implicit dependencies based on the config.
// The path is the JSON path in your robot's config (not the `Config` struct) to the
// resource being validated; e.g. "components.0".
func (cfg *Config) Validate(path string) ([]string, error) {
	return []string{cfg.SourceSensor}, nil
}

type energyTracker struct {
	resource.AlwaysRebuild

	name resource.Name

	logger logging.Logger
	cfg    *Config

	sourceSensor powersensor.PowerSensor
	cancelCtx    context.Context
	cancelFunc   func()

	refreshRateMSec int
	lastReadingTime time.Time

	mu sync.Mutex

	totalEnergyWh  float64
	totalCurrentAh float64
}

func newEnergyTrackerEnergyTracker(ctx context.Context, deps resource.Dependencies, rawConf resource.Config, logger logging.Logger) (powersensor.PowerSensor, error) {
	conf, err := resource.NativeConfig[*Config](rawConf)
	if err != nil {
		return nil, err
	}

	return NewEnergyTracker(ctx, deps, rawConf.ResourceName(), conf, logger)

}

func NewEnergyTracker(ctx context.Context, deps resource.Dependencies, name resource.Name, conf *Config, logger logging.Logger) (powersensor.PowerSensor, error) {

	depSensorName := resource.NewName(powersensor.API, conf.SourceSensor)
	sourceSensor, err := deps.Lookup(depSensorName)
	if err != nil {
		return nil, resource.DependencyNotFoundError(depSensorName)
	}

	sourceSensorPS, ok := sourceSensor.(powersensor.PowerSensor)
	if !ok {
		return nil, resource.DependencyTypeError[powersensor.PowerSensor](depSensorName, sourceSensor)
	}

	if conf.RefreshRateMSec < 0 {
		return nil, fmt.Errorf("refresh_rate_msec must be greater than 0")
	}

	if conf.RefreshRateMSec == 0 {
		conf.RefreshRateMSec = 2000
	}

	cancelCtx, cancelFunc := context.WithCancel(context.Background())

	s := &energyTracker{
		name:            name,
		logger:          logger,
		cfg:             conf,
		cancelCtx:       cancelCtx,
		cancelFunc:      cancelFunc,
		sourceSensor:    sourceSensorPS,
		refreshRateMSec: conf.RefreshRateMSec,
		lastReadingTime: time.Now(),
		totalEnergyWh:   0,
		totalCurrentAh:  0,
	}

	// start a background go routine to update the energy reading
	go func() {
		for {
			select {
			case <-s.cancelCtx.Done():
				return
			case <-time.NewTicker(time.Duration(s.refreshRateMSec) * time.Millisecond).C:
				// Get power reading
				current, _, err := s.sourceSensor.Current(s.cancelCtx, nil)
				if err != nil {
					continue
				}
				power, err := s.sourceSensor.Power(s.cancelCtx, nil)
				if err != nil {
					continue
				}

				// Calculate energy based on elapsed time
				now := time.Now()

				// Calculate elapsed time in hours (since energy is typically watt-hours)
				elapsedHours := now.Sub(s.lastReadingTime).Hours()

				// Calculate energy (power * time) and add to running total
				s.mu.Lock()
				s.totalEnergyWh += power * elapsedHours
				s.totalCurrentAh += current * elapsedHours
				s.lastReadingTime = now
				s.mu.Unlock()
			}
		}
	}()
	return s, nil
}

func (s *energyTracker) Name() resource.Name {
	return s.name
}

func (s *energyTracker) Voltage(ctx context.Context, extra map[string]interface{}) (float64, bool, error) {
	return s.sourceSensor.Voltage(ctx, extra)
}

func (s *energyTracker) Current(ctx context.Context, extra map[string]interface{}) (float64, bool, error) {
	return s.sourceSensor.Current(ctx, extra)
}

func (s *energyTracker) Power(ctx context.Context, extra map[string]interface{}) (float64, error) {
	return s.sourceSensor.Power(ctx, extra)
}

func (s *energyTracker) Readings(ctx context.Context, extra map[string]interface{}) (map[string]interface{}, error) {
	readings, err := s.sourceSensor.Readings(ctx, extra)
	if err != nil {
		return nil, err
	}
	readings["total_energy_Wh"] = s.totalEnergyWh
	readings["total_current_Ah"] = s.totalCurrentAh
	return readings, nil
}

func (s *energyTracker) DoCommand(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error) {
	// Check if the command is requesting to reset the energy counters
	if reset, ok := cmd["reset"]; ok {
		// If Reset is true, reset the energy counters
		if resetBool, ok := reset.(bool); ok && resetBool {
			s.mu.Lock()
			s.totalEnergyWh = 0
			s.totalCurrentAh = 0
			s.lastReadingTime = time.Now()
			s.mu.Unlock()
			return map[string]interface{}{"status": "success"}, nil
		}
	}
	return nil, nil
}

func (s *energyTracker) Close(context.Context) error {
	s.cancelFunc()
	return nil
}
