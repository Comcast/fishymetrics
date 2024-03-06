# fishymetrics exporter for Prometheus

This is a simple server that scrapes a baremetal chassis' managers stats using the redfish API and 
exports them via HTTP for Prometheus consumption.

Current device models supported
- HP Moonshot
- HP DL380
- HP DL360
- HP DL560
- HP DL20
- HP XL420
- Cisco UCS C220 M5
- Cisco UCS S3260 M4
- Cisco UCS S3260 M5

## Getting Started

To run it:

```bash
$ ./fishymetrics --help
usage: fishymetrics [<flags>]

redfish api exporter with all the bells and whistles

Flags:
  -h, --help                    Show context-sensitive help (also try --help-long and --help-man).
      --user=""                 BMC static username
      --password=""             BMC static password
      --timeout=15s             BMC scrape timeout
      --scheme="https"          BMC Scheme to use
      --log.method=[file|vector]
                                alternative method for logging in addition to stdout
      --log.file-path="/var/log/fishymetrics"
                                directory path where log files are written if log-method is file
      --log.file-max-size=256   max file size in megabytes if log-method is file
      --log.file-max-backups=1  max file backups before they are rotated if log-method is file
      --log.file-max-age=1      max file age in days before they are rotated if log-method is file
      --vector.endpoint="http://0.0.0.0:4444"
                                vector endpoint to send structured json logs to
      --port="9533"             exporter port
      --vault.addr="https://vault.com"
                                Vault instance address to get chassis credentials from
      --vault.role-id=""        Vault Role ID for AppRole
      --vault.secret-id=""      Vault Secret ID for AppRole
      --credential.profiles=CREDENTIAL.PROFILES
                                profile(s) with all necessary parameters to obtain BMC credential from secrets backend, i.e.

                                  --credential.profiles="
                                    profiles:
                                      - name: profile1
                                        mountPath: "kv2"
                                        path: "path/to/secret"
                                        userField: "user"
                                        passwordField: "password"
                                      ...
                                  "

                                --credential.profiles='{"profiles":[{"name":"profile1","mountPath":"kv2","path":"path/to/secret","userField":"user","passwordField":"password"},...]}'
```

Or set the following ENV Variables:
```bash
BMC_USERNAME=<string>
BMC_PASSWORD=<string>
BMC_TIMEOUT=<duration> (Default: 15s)
BMC_SCHEME=<string> (Default: https)
EXPORTER_PORT=<int> (Default: 9533)
LOG_PATH=<string> (Default: /var/log/fishymetrics)
VAULT_ADDRESS=<string>
VAULT_ROLE_ID=<string>
VAULT_SECRET_ID=<string>
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
curl http://localhost:9533/info
```

### metrics URL

Responds with the application's runtime metrics

<aside class="notice">
_if deployed on ones localhost_
</aside>

```bash
curl http://localhost:9533/metrics
```

### redfish API `/scrape`

To test a scrape of a host's redfish API, you can curl `fishymetrics`

```bash
curl 'http://localhost:9533/scrape?module=<module-name>&target=1.2.3.4'
```

If you have a credential profile configured you can add the extra URL query parameter

```bash
curl 'http://localhost:9533/scrape?module=<module-name>&target=1.2.3.4&credential_profile=<profile-name>'
```

### Docker

To run the fishymetrics exporter as a Docker container using static crdentials, run:

#### Using ENV variables
```bash
docker run --name fishymetrics -d -p <EXPORTER_PORT>:<EXPORTER_PORT> \
-e BMC_USERNAME='<user>' \
-e BMC_PASSWORD='<password>' \
-e BMC_TIMEOUT=15s \
-e EXPORTER_PORT=1234 \
comcast/fishymetrics:latest
```

#### Using command line args
```bash
docker run --name fishymetrics -d -p <EXPORTER_PORT>:<EXPORTER_PORT> \
-user '<user>' \
-password '<password>' \
-timeout 15s \
-port 1234 \
comcast/fishymetrics:latest
```

## Prometheus Configuration

The fishymetrics exporter needs to be passed the address as a parameter, this can be
done with relabelling. available module options `["moonshot", "dl360", "dl20", "dl380", "dl560", "xl420", "c220", "s3260m4", "s3260m5"]`

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
