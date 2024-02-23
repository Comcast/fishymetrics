# fishymetrics exporter for Prometheus

This is a simple server that scrapes a baremetal chassis' managers stats using the redfish API and 
exports them via HTTP for Prometheus consumption.

Current device models supported
- HP Moonshot
- HP DL380
- HP DL360
- HP DL560
- HP DL20
- Cisco UCS C220 M5
- Cisco UCS S3260 M4
- Cisco UCS S3260 M5

## Getting Started

To run it:

```bash
./fishymetrics [flags]
```

Help on flags:

```bash
./fishymetrics --help
```

```bash
  -log-path string
    	directory path where log files are written (default "/var/log/fishymetrics")
  -password string
    	OOB static password
  -port string
    	exporter port (default "9533")
  -scheme string
    	OOB Scheme to use (default "https")
  -timeout duration
    	OOB scrape timeout (default 15s)
  -user string
    	OOB static username
  -vault-addr string
    	Vault address to get chassis credentials from (default "https://vault.com")
  -vault-kv2-mount-path string
    	Vault config path where kv2 secrets are mounted (default "kv2")
  -vault-kv2-password-field string
    	Vault kv2 secret field where we get the password (default "password")
  -vault-kv2-path string
    	Vault path where kv2 secrets will be retreived (default "path/to/secrets")
  -vault-kv2-user-field string
    	Vault kv2 secret field where we get the username (default "user")
  -vault-role-id string
    	Vault Role ID for AppRole
  -vault-secret-id string
    	Vault Secret ID for AppRole
```

Or set the following ENV Variables:
```bash
USERNAME=<string>
PASSWORD=<string>
OOB_TIMEOUT=<duration> (Default: 15s)
OOB_SCHEME=<string> (Default: https)
EXPORTER_PORT=<int> (Default: 9533)
LOG_PATH=<string> (Default: /var/log/fishymetrics)
VAULT_ADDRESS=<string>
VAULT_ROLE_ID=<string>
VAULT_SECRET_ID=<string>
VAULT_KV2_PATH=<string>
VAULT_KV2_MOUNT_PATH=<string>
VAULT_KV2_USER_FIELD=<string>
VAULT_KV2_PASS_FIELD=<string>
```
```bash
./fishymetrics
```

## Usage

### build info URL

Responds with the application's `version`, `build_date`, `go_version`, `etc`

<aside class="notice">
_if deployed on ones localhost_
</aside>

```bash
http://localhost:9533/info
```

### metrics URL

Responds with the application's runtime metrics

<aside class="notice">
_if deployed on ones localhost_
</aside>

```bash
http://localhost:9533/metrics
```

### Docker

To run the fishymetrics exporter as a Docker container, run:

#### Using ENV variables
```bash
docker run --name fishymetrics -d -p <EXPORTER_PORT>:<EXPORTER_PORT> \
-e HP_USERNAME='<user>' \
-e HP_PASSWORD='<password>' \
-e CISCO_USERNAME='<user>' \
-e CISCO_PASSWORD='<password>' \
-e OOB_TIMEOUT=15s \
-e EXPORTER_PORT=1234 \
comcast/fishymetrics:latest
```

#### Using command line args
```bash
docker run --name fishymetrics -d -p <EXPORTER_PORT>:<EXPORTER_PORT> \
-hp-user '<user>' \
-hp-password '<password>' \
-cisco-user '<user>' \
-cisco-password '<password>' \
-timeout 15s \
-port 1234 \
comcast/fishymetrics:latest
```

## Prometheus Configuration

The fishymetrics exporter needs to be passed the address as a parameter, this can be
done with relabelling. available module options `["moonshot", "dl360", "dl20", "dl380", "dl560", "c220", "s3260m4", "s3260m5"]`

Example config:
```YAML
scrape_configs:
  - job_name: 'fishymetrics'
    static_configs:
      - targets:
        - ilo-fdqn-p1.example.com
        labels:
          foo: bar
      - targets:
        - ilo-fdqn-p2.example.com
        labels:
          foo: bar
    metrics_path: /scrape
    scrape_interval: 5m
    params:
      module: ["moonshot"]
    relabel_configs:
      - source_labels: [__address__]
        target_label: __param_target
      - source_labels: [__param_target]
        target_label: instance
      - target_label: __address__
        replacement: fishymetrics.example.com  # Kubernetes cluster nginx-ingress FQDN or any host IP/FQDN you deployed with
```

## Development

### Building

#### linux binary
```
make build
```

#### docker image
```
make docker
```

### Testing

```
make test
```
