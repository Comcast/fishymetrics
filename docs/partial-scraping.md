# Partial Scraping Feature

## Overview

The partial scraping feature allows you to selectively collect metrics from specific components of a Redfish-enabled device, rather than performing a full scrape of all components. This can significantly reduce scrape time and resource usage when you only need metrics from certain subsystems.

## Endpoint

```
GET /scrape/partial
```

## Parameters

### Required Parameters

- **`target`** - The IP address or hostname of the BMC/iLO to scrape
- **`components`** - Comma-separated list of components to scrape

### Optional Parameters

- **`model`** - Device model (e.g., "hp", "dell", "cisco")
- **`credential_profile`** - Vault credential profile to use
- **`plugins`** - Additional plugins to enable (e.g., "nuova")

## Available Components

The following components can be specified in the `components` parameter:

| Component | Description | Metrics Collected |
|-----------|-------------|-------------------|
| `thermal` | Temperature sensors and fans | Fan speeds, sensor temperatures, thermal status |
| `power` | Power supplies and voltage | Power consumption, voltage levels, PSU status, line input voltage |
| `memory` | Memory DIMMs | DIMM status, capacity, memory health |
| `processor` | CPUs/Processors | Processor status, core count, socket information |
| `drives` | Storage drives (NVMe, SAS, SATA) | Drive health, capacity, failure prediction |
| `storage_controller` | RAID/Storage controllers | Controller status, firmware version |
| `firmware` | Firmware versions | Component firmware, iLO self-test |
| `system` | System information | BIOS version, serial numbers, memory summary |

## Usage Examples

### Single Component Scrape

Scrape only thermal metrics:
```bash
curl "http://localhost:10023/scrape/partial?target=10.0.0.1&components=thermal"
```

### Multiple Components Scrape

Scrape thermal and power metrics:
```bash
curl "http://localhost:10023/scrape/partial?target=10.0.0.1&components=thermal,power"
```

### With Credential Profile

Scrape memory and processor metrics using a specific credential profile:
```bash
curl "http://localhost:10023/scrape/partial?target=10.0.0.1&components=memory,processor&credential_profile=prod"
```

### All Hardware Components

Scrape all hardware-related metrics (excluding firmware):
```bash
curl "http://localhost:10023/scrape/partial?target=10.0.0.1&components=thermal,power,memory,processor,drives,storage_controller"
```

## Prometheus Configuration

You can configure Prometheus to use partial scraping for different job types:

```yaml
scrape_configs:
  # Full scrape for comprehensive monitoring
  - job_name: 'fishymetrics_full'
    scrape_interval: 5m
    scrape_timeout: 30s
    metrics_path: '/scrape'
    params:
      model: ['hp']
    static_configs:
      - targets:
        - server1.example.com
        - server2.example.com
    relabel_configs:
      - source_labels: [__address__]
        target_label: __param_target
      - source_labels: [__param_target]
        target_label: instance
      - target_label: __address__
        replacement: fishymetrics:10023

  # Partial scrape for thermal monitoring (more frequent)
  - job_name: 'fishymetrics_thermal'
    scrape_interval: 1m
    scrape_timeout: 10s
    metrics_path: '/scrape/partial'
    params:
      model: ['hp']
      components: ['thermal']
    static_configs:
      - targets:
        - server1.example.com
        - server2.example.com
    relabel_configs:
      - source_labels: [__address__]
        target_label: __param_target
      - source_labels: [__param_target]
        target_label: instance
      - target_label: __address__
        replacement: fishymetrics:10023

  # Partial scrape for power monitoring
  - job_name: 'fishymetrics_power'
    scrape_interval: 2m
    scrape_timeout: 15s
    metrics_path: '/scrape/partial'
    params:
      model: ['hp']
      components: ['power,thermal']
    static_configs:
      - targets:
        - server1.example.com
        - server2.example.com
    relabel_configs:
      - source_labels: [__address__]
        target_label: __param_target
      - source_labels: [__param_target]
        target_label: instance
      - target_label: __address__
        replacement: fishymetrics:10023
```

## Use Cases

### 1. High-Frequency Thermal Monitoring
Monitor temperature sensors every minute without overwhelming the BMC:
```
components=thermal
scrape_interval=1m
```

### 2. Power Usage Tracking
Track power consumption for capacity planning:
```
components=power
scrape_interval=2m
```

### 3. Storage Health Monitoring
Focus on drive health for storage servers:
```
components=drives,storage_controller
scrape_interval=5m
```

### 4. Quick Health Checks
Lightweight health monitoring:
```
components=system
scrape_interval=10m
```

### 5. Tiered Monitoring Strategy
- **Tier 1 (Critical)**: Thermal monitoring every 1 minute
- **Tier 2 (Important)**: Power and system every 5 minutes
- **Tier 3 (Standard)**: Full scrape every 15 minutes

## Best Practices

1. **Group Related Components**: When monitoring related metrics, group them in a single scrape to reduce overhead
   ```
   # Good - single scrape for related metrics
   components=thermal,power

   # Less efficient - separate scrapes
   components=thermal (first scrape)
   components=power (second scrape)
   ```

2. **Balance Frequency and Components**: More frequent scrapes should use fewer components
   ```
   # Good - frequent scrape with minimal components
   scrape_interval=30s
   components=thermal

   # Bad - frequent scrape with many components
   scrape_interval=30s
   components=thermal,power,memory,processor,drives
   ```

3. **Use Full Scrapes for Baseline**: Perform periodic full scrapes to ensure no metrics are missed
   ```yaml
   # Partial scrapes for monitoring
   - job_name: 'partial_monitoring'
     scrape_interval: 1m
     metrics_path: '/scrape/partial'
     params:
       components: ['thermal,power']

   # Full scrape for complete picture
   - job_name: 'full_baseline'
     scrape_interval: 30m
     metrics_path: '/scrape'
   ```

4. **Monitor Scrape Duration**: Use Prometheus metrics to monitor scrape duration and adjust components/frequency accordingly
   ```promql
   # Monitor scrape duration by job
   prometheus_target_scrape_duration_seconds{job="fishymetrics_partial"}
   ```

## Migration Guide

To migrate from full scraping to partial scraping:

1. **Identify Required Metrics**: Analyze which metrics you actually use in dashboards and alerts

2. **Map Metrics to Components**: Determine which components provide those metrics

3. **Test Partial Scrapes**: Verify partial scrapes return expected metrics
   ```bash
   # Test and compare outputs
   curl "http://localhost:10023/scrape?target=server1" > full.txt
   curl "http://localhost:10023/scrape/partial?target=server1&components=thermal,power" > partial.txt
   ```

4. **Update Prometheus Configuration**: Modify scrape configs to use partial endpoint

5. **Monitor and Adjust**: Monitor scrape performance and adjust components/frequency as needed
