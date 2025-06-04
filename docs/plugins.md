# plugins

Plugins are a way to extend the capabilities of fishymetrics, by allowing the exporter to execute custom code after all the initializations are complete for a devices standard redfish metrics scrape. The main purpose for plugins is to collect component metrics data that is inaccessible using the preferred redfish API endpoints. Below is an example of how to create a custom plugin.

## directory location

All plugins should be created as a new package and placed in the `plugins/` folder at the root level. 

## interface

All new plugins must fulfill the `Plugin interface` by supplying an `Apply(*Exporter)` function.

```go
type Plugin interface {
	Apply(*Exporter) error
}
```

## Hooking into Exporters prometheus metrics

Inside a plugins `Apply(*Exporter)` function you assign the current exporter's prometheus metrics hashmap pointer to the plugin's pointer reference so that the custom handler function(s) have access to the prometheus metrics hashmap. i.e.

```go
type NewPlugin exporter.Exporter

func (n *NewPlugin) exportMetrics(body []byte) error {

	var state float64
	var metrx MetricsStruct
	var f = (*n.DeviceMetrics)["fishyMetrics"]
	...

	(*f)["fishyStatus"].WithLabelValues(metrx.Label1, metrx.Label2, n.ChassisSerialNumber, n.Model).Set(state)

	return nil
}

func (n *NewPlugin) Apply(e *exporter.Exporter) error {
  
  var handlers []common.Handler
  ...
    n.DeviceMetrics = e.DeviceMetrics
    handlers = append(handlers, n.exportMetrics)
  ...
}
```

## Append URL endpoint calls

Lastly, we need to append the additional custom endpoint call(s) to the task pool so that they are executed when the prometheus collector initiates the `scrape()` function.

```go
func (n *NewPlugin) Apply(e *exporter.Exporter) error {
  
  var handlers []common.Handler
  ...
    handlers = append(handlers, n.exportMetrics)
    e.GetPool().AddTask(pool.NewTask(Fetch("http://hostname/new-endpoint", ...), handlers))
  ...
}
```