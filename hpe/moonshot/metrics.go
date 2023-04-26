package moonshot

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
			"fanSpeed":          newServerMetric("thermal_fan_speed", "Current fan speed in the unit of percentage, possible values are 0 - 100", nil, []string{"name"}),
			"fanStatus":         newServerMetric("thermal_fan_status", "Current fan status 1 = OK, 0 = BAD", nil, []string{"name"}),
			"sensorTemperature": newServerMetric("thermal_sensor_temperature", "Current sensor temperature reading in Celsius", nil, []string{"name"}),
			"sensorStatus":      newServerMetric("thermal_sensor_status", "Current sensor status 1 = OK, 0 = BAD", nil, []string{"name"}),
		}

		PowerMetrics = &metrics{
			"supplyOutput":        newServerMetric("power_supply_output", "Power supply output in watts", nil, []string{"name", "sparePartNumber"}),
			"supplyStatus":        newServerMetric("power_supply_status", "Current power supply status 1 = OK, 0 = BAD", nil, []string{"name", "sparePartNumber"}),
			"supplyTotalConsumed": newServerMetric("power_supply_total_consumed", "Total output of all power supplies in watts", nil, []string{}),
			"supplyTotalCapacity": newServerMetric("power_supply_total_capacity", "Total output capacity of all the power supplies", nil, []string{}),
		}

		SwitchMetrics = &metrics{
			"moonshotSwitchStatus": newServerMetric("moonshot_switch_status", "Current Moonshot switch status 1 = OK, 0 = BAD", nil, []string{"name", "serialNumber"}),
		}

		SwitchThermalMetrics = &metrics{
			"moonshotSwitchSensorTemperature": newServerMetric("moonshot_switch_thermal_sensor_temperature", "Current sensor temperature reading in Celsius", nil, []string{"name"}),
			"moonshotSwitchSensorStatus":      newServerMetric("moonshot_switch_thermal_sensor_status", "Current sensor status 1 = OK, 0 = BAD", nil, []string{"name"}),
		}

		SwitchPowerMetrics = &metrics{
			"moonshotSwitchSupplyOutput": newServerMetric("moonshot_switch_power_supply_output", "Power supply output in watts", nil, []string{"name"}),
		}

		Metrics = &map[string]*metrics{
			"thermalMetrics":   ThermalMetrics,
			"powerMetrics":     PowerMetrics,
			"swMetrics":        SwitchMetrics,
			"swThermalMetrics": SwitchThermalMetrics,
			"swPowerMetrics":   SwitchPowerMetrics,
		}
	)

	return Metrics
}
