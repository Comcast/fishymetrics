package dl20

import (
	"github.com/prometheus/client_golang/prometheus"
)

type metrics map[string]*prometheus.GaugeVec

func newServerMetric(metricName string, docString string, constLabels prometheus.Labels, labelNames []string) *prometheus.GaugeVec {
	return prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name:        metricName,
			Help:        docString,
			ConstLabels: constLabels,
		},
		labelNames,
	)
}

func NewDeviceMetrics() *map[string]*metrics {
	var (
		ThermalMetrics = &metrics{
			"fanSpeed":          newServerMetric("dl20_thermal_fan_speed", "Current fan speed in the unit of percentage, possible values are 0 - 100", nil, []string{"name"}),
			"fanStatus":         newServerMetric("dl20_thermal_fan_status", "Current fan status 1 = OK, 0 = BAD", nil, []string{"name"}),
			"sensorTemperature": newServerMetric("dl20_thermal_sensor_temperature", "Current sensor temperature reading in Celsius", nil, []string{"name"}),
			"sensorStatus":      newServerMetric("dl20_thermal_sensor_status", "Current sensor status 1 = OK, 0 = BAD", nil, []string{"name"}),
		}

		PowerMetrics = &metrics{
			"supplyOutput":        newServerMetric("dl20_power_supply_output", "Power supply output in watts", nil, []string{"memberId", "sparePartNumber"}),
			"supplyStatus":        newServerMetric("dl20_power_supply_status", "Current power supply status 1 = OK, 0 = BAD", nil, []string{"memberId", "sparePartNumber"}),
			"supplyTotalConsumed": newServerMetric("dl20_power_supply_total_consumed", "Total output of all power supplies in watts", nil, []string{"memberId"}),
			"supplyTotalCapacity": newServerMetric("dl20_power_supply_total_capacity", "Total output capacity of all the power supplies", nil, []string{"memberId"}),
		}

		DriveMetrics = &metrics{
			"logicalDriveStatus": newServerMetric("dl20_logical_drive_status", "Current logical drive status 1 = OK, 0 = BAD, -1 = DISABLED", nil, []string{"name", "logicalDriveNumber", "raid"}),
		}

		MemoryMetrics = &metrics{
			"memoryStatus": newServerMetric("dl20_memory_status", "Current memory status 1 = OK, 0 = BAD", nil, []string{"totalSystemMemoryGiB"}),
		}

		Metrics = &map[string]*metrics{
			"thermalMetrics": ThermalMetrics,
			"powerMetrics":   PowerMetrics,
			"driveMetrics":   DriveMetrics,
			"memoryMetrics":  MemoryMetrics,
		}
	)

	return Metrics
}
