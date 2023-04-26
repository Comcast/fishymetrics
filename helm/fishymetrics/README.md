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

Parameter | Description | Default
--------- | ----------- | -------
`image.repo` | container image repo for fishymetrics | `"comcast/fishymetrics"`
`image.tag` | container image tag for fishymetrics | `"0.6.15"`
`image.pullPolicy` | container image pull policy | `"IfNotPresent"`
`replicas` | number of replica sets to initially deploy | `1`
`exporter.port` | exporter port to listen on | `9533`
`logPath` | directory path where log files are written | `"/var/log/fishymetrics"`
`oob.scheme` | Out-of-Bound endpoint to collect data from | `"https"`
`oob.username` | username to use when logging into Out-of-Bound | `""`
`oob.password` | password to use when logging into Out-of-Bound | `""`
`oob.timeout` | Out-of-Bound timeout | `15s`
`vault.address` | Vault instance address to get chassis credentials from | `"https://vault.com"`
`vault.roleId` | Vault Role ID for AppRole | `""`
`vault.secretId` | Vault Secret ID for AppRole | `""`
`vault.kv2MountPath` | Vault config path where kv2 secrets are mounted | `"kv2"`
`vault.kv2Path` | Vault path where kv2 secrets will be retreived | `"path/to/secret"`
`vault.kv2UserField` | Vault kv2 secret field where we get the username | `"user"`
`vault.kv2PasswordField` | Vault kv2 secret field where we get the password | `"password"`
`elastic.enabled` | boolean flag to enable/disable vector log forwarding | `false`
`elastic.image.repo` | container image repo for datadog vector | `"timberio/vector"`
`elastic.image.tag` | container image tag for datadog vector | `"0.26.0-alpine"`
`elastic.image.pullPolicy` | container image pull policy | `"IfNotPresent"`
`elastic.user` | username authorization to external elastic instance | `""`
`elastic.pass` | password authorization to external elastic instance | `""`
`elastic.endpoint` | external elastic instance endpoint | `""`
`elastic.indexName` | external elastic index name to forward logs to | `""`
`elastic.resources` | amount of cpu and memory resources to give vector container | `{}`
`resources` | amount of cpu and memory resources to give fishymetrics container | `{}`
`horizontalPodAutoscaler.enabled` | boolean flag to enable/disable kubernetes Horizontal Pod Autoscaler | `false`
`horizontalPodAutoscaler.minReplicas` | minimum replicas for kubernetes Horizontal Pod Autoscaler | `1`
`horizontalPodAutoscaler.maxReplicas` | maximum replicas for kubernetes Horizontal Pod Autoscaler | `1`
`horizontalPodAutoscaler.targetMemoryUtilizationPercentage` | percentage of pod memory utilization the app must reach before another pod is deployed | `80`
`ingress.enabled` | boolean flag to enable/disable the kubernetes ingress resource to the fishymetrics service | `false`
`ingress.hosts` | list of host(s) to listen for fishymetrics requests | `[]`
`ingress.annotations` | annotation labels to add to the ingress kubernetes resource | `{}`
`ingress.tls` | list of tls certificates to be applied to the ingress host(s) | `[]`
`affinity` | affinity to apply to the kubernetes deployment | `{}`
`nodeSelector` | nodeSelector to apply to the kubernetes deployment | `{}`
`tolerations` | tolerations to apply to the kubernetes deployment | `[]`
