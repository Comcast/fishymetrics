# fishymetrics

fishymetrics is a simple server that scrapes a baremetal chassis' managers stats using the redfish API and
exports them via HTTP for Prometheus consumption.

## Installing the Chart

To install the chart with the release name `my-release`:

```console
$ helm install --name my-release <repo>/fishymetrics
```

The command deploys fishymetrics on the Kubernetes cluster in the default configuration. The [configuration](#configuration) section lists the parameters that can be configured during installation.

> **Tip**: List all releases using `helm list`

## Uninstalling the Chart

To uninstall/delete the `my-release` deployment:

```console
$ helm delete my-release
```

## Configuration

The following table lists the configurable parameters of the fishymetrics chart and their default values.

| Parameter                                                   | Description                                                                                | Default                   |
| ----------------------------------------------------------- | ------------------------------------------------------------------------------------------ | ------------------------- |
| `image.repo`                                                | container image repo for fishymetrics                                                      | `"comcast/fishymetrics"`  |
| `image.tag`                                                 | container image tag for fishymetrics                                                       | `"0.12.1"`                |
| `image.pullPolicy`                                          | container image pull policy                                                                | `"IfNotPresent"`          |
| `replicas`                                                  | number of replica sets to initially deploy                                                 | `1`                       |
| `exporter.port`                                             | exporter port to listen on                                                                 | `10023`                    |
| `log.level`                                                 | log level verbosity                                                                        | `"info"`                  |
| `log.method`                                                | alternative method for logging in addition to stdout                                       | `""`                      |
| `log.filePath`                                              | directory path where log files are written                                                 | `"/var/log/fishymetrics"` |
| `log.fileMaxSize`                                           | maximum size of log files in MiB before they are rotated                                   | `256`                     |
| `log.fileMaxBackups`                                        | maximum number of log files to keep                                                        | `1`                       |
| `log.fileMaxAge`                                            | maximum number of days to keep log files                                                   | `1`                       |
| `bmc.scheme`                                                | baseboard management controller endpoint to collect data from                              | `"https"`                 |
| `bmc.username`                                              | username to use when logging into baseboard management controller                          | `""`                      |
| `bmc.password`                                              | password to use when logging into baseboard management controller                          | `""`                      |
| `bmc.timeout`                                               | baseboard management controller request timeout                                            | `15s`                     |
| `bmc.insecureSkipVerify`                                    | boolean flag to enable/disable TLS verification to baseboard management controller         | `false`                   |
| `vault.address`                                             | vault instance address to get chassis credentials from                                     | `"https://vault.com"`     |
| `vault.roleId`                                              | vault Role ID for AppRole                                                                  | `""`                      |
| `vault.secretId`                                            | vault Secret ID for AppRole                                                                | `""`                      |
| `vault.kv2MountPath`                                        | vault config path where kv2 secrets are mounted                                            | `"kv2"`                   |
| `vault.kv2Path`                                             | vault path where kv2 secrets will be retreived                                             | `"path/to/secret"`        |
| `vault.kv2UserField`                                        | vault kv2 secret field where we get the username                                           | `"user"`                  |
| `vault.kv2PasswordField`                                    | vault kv2 secret field where we get the password                                           | `"password"`              |
| `collector.drives.modulesExclude`                           | drive module(s) to exclude from the scrape                                                 | `""`                      |
| `collector.firmware.modulesExclude`                         | firmware module(s) to exclude from the scrape                                              | `""`                      |
| `credentials.profiles`                                      | profile(s) with all necessary parameters to obtain BMC credential from secrets backend     | `[]`                      |
| `vector.enabled`                                            | boolean flag to enable/disable vector log forwarding                                       | `false`                   |
| `vector.endpoint`                                           | vector client endpoint, in most cases this is deployed to localhost                        | `"http://localhost:4444"` |
| `vector.port`                                               | vector client port                                                                         | `4444`                    |
| `vector.image.repo`                                         | container image repo for datadog vector image                                              | `"timberio/vector"`       |
| `vector.image.tag`                                          | container image tag for datadog vector image                                               | `"0.26.0-alpine"`         |
| `vector.image.pullPolicy`                                   | container image pull policy                                                                | `"IfNotPresent"`          |
| `vector.elasticsearch.user`                                 | username authorization to external elastic instance                                        | `""`                      |
| `vector.elasticsearch.pass`                                 | password authorization to external elastic instance                                        | `""`                      |
| `vector.elasticsearch.endpoint`                             | external elastic instance endpoint                                                         | `""`                      |
| `vector.elasticsearch.indexName`                            | external elastic index name to forward logs to                                             | `""`                      |
| `vector.resources`                                          | amount of cpu and memory resources to give vector container                                | `{}`                      |
| `resources`                                                 | amount of cpu and memory resources to give fishymetrics container                          | `{}`                      |
| `horizontalPodAutoscaler.enabled`                           | boolean flag to enable/disable kubernetes Horizontal Pod Autoscaler                        | `false`                   |
| `horizontalPodAutoscaler.minReplicas`                       | minimum replicas for kubernetes Horizontal Pod Autoscaler                                  | `1`                       |
| `horizontalPodAutoscaler.maxReplicas`                       | maximum replicas for kubernetes Horizontal Pod Autoscaler                                  | `1`                       |
| `horizontalPodAutoscaler.targetMemoryUtilizationPercentage` | percentage of pod memory utilization the app must reach before another pod is deployed     | `80`                      |
| `ingress.enabled`                                           | boolean flag to enable/disable the kubernetes ingress resource to the fishymetrics service | `false`                   |
| `ingress.hosts`                                             | list of host(s) to listen for fishymetrics requests                                        | `[]`                      |
| `ingress.annotations`                                       | annotation labels to add to the ingress kubernetes resource                                | `{}`                      |
| `ingress.tls`                                               | list of tls certificates to be applied to the ingress host(s)                              | `[]`                      |
| `affinity`                                                  | affinity to apply to the kubernetes deployment                                             | `{}`                      |
| `nodeSelector`                                              | nodeSelector to apply to the kubernetes deployment                                         | `{}`                      |
| `tolerations`                                               | tolerations to apply to the kubernetes deployment                                          | `[]`                      |
