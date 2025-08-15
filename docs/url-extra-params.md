# URL Extra Parameters Feature

## Overview

The URL Extra Parameters feature allows you to dynamically pass additional query parameters to fishymetrics that can be used to customize Vault credential paths. This is particularly useful when you need to:
- Use different credentials for different environments
- Include dynamic identifiers in Vault paths
- Support multi-tenant scenarios where credentials are organized by customer or region

## Configuration

### Command Line Flag
```bash
--url.extra-params="param1:alias1,param2:alias2"
```

### Environment Variable
```bash
export URL_EXTRA_PARAMS="param1:alias1,param2:alias2"
```

## How It Works

1. **Define Parameters**: Configure which URL parameters to capture and their internal aliases
2. **Pass Parameters**: Include these parameters in your scrape URL
3. **Path Substitution**: The parameters are automatically substituted into Vault credential paths

### Parameter Format

The format is `url_param:internal_alias` where:
- `url_param`: The parameter name as it appears in the URL query string
- `internal_alias`: The variable name used internally for Vault path substitution

## Usage Examples

### Example 1: Environment-Based Credentials

Configure fishymetrics to accept an environment parameter:

```bash
./fishymetrics \
  --vault.addr="https://vault.example.com" \
  --vault.role-id="$VAULT_ROLE_ID" \
  --vault.secret-id="$VAULT_SECRET_ID" \
  --url.extra-params="env:environment"
```

Then scrape with the environment parameter:
```bash
curl "http://localhost:10023/scrape?target=10.0.0.1&env=production&credential_profile=servers"
```

If your credential profile has a path like:
```yaml
profiles:
  - name: servers
    mountPath: "kv2"
    path: "bmc/{environment}/credentials"
    userField: "username"
    passwordField: "password"
```

The `{environment}` placeholder will be replaced with `production`, resulting in the Vault path:
```
kv2/bmc/production/credentials
```

### Example 2: Multi-Tenant Setup

For a multi-tenant environment where credentials are organized by customer:

```bash
./fishymetrics \
  --url.extra-params="customer_id:customer,region:datacenter"
```

Scrape URL:
```bash
curl "http://localhost:10023/scrape?target=10.0.0.1&customer_id=acme&region=us-east&credential_profile=tenant"
```

With credential profile:
```yaml
profiles:
  - name: tenant
    mountPath: "kv2"
    path: "customers/{customer}/dc/{datacenter}/bmc"
    userField: "user"
    passwordField: "pass"
```

Results in Vault path:
```
kv2/customers/acme/dc/us-east/bmc
```

### Example 3: Device-Specific Credentials

For scenarios where credentials are organized by device type or location:

```bash
./fishymetrics \
  --url.extra-params="rack:rack_id,dc:datacenter,type:device_type"
```

Scrape URL:
```bash
curl "http://localhost:10023/scrape?target=10.0.0.1&rack=A42&dc=nyc&type=dell&credential_profile=hardware"
```

## Prometheus Configuration

### Basic Configuration

```yaml
scrape_configs:
  - job_name: 'fishymetrics_env'
    static_configs:
      - targets:
        - server1.example.com
        labels:
          environment: production
      - targets:
        - server2.example.com
        labels:
          environment: staging
    metrics_path: /scrape
    params:
      model: ['hp']
      credential_profile: ['servers']
    relabel_configs:
      - source_labels: [__address__]
        target_label: __param_target
      - source_labels: [environment]
        target_label: __param_env  # Maps to 'env' URL parameter
      - source_labels: [__param_target]
        target_label: instance
      - target_label: __address__
        replacement: fishymetrics:10023
```

### Multi-Tenant Configuration

```yaml
scrape_configs:
  - job_name: 'fishymetrics_multitenant'
    static_configs:
      - targets:
        - bmc1.customer1.com
        labels:
          customer_id: customer1
          region: us-west
      - targets:
        - bmc2.customer2.com
        labels:
          customer_id: customer2
          region: eu-central
    metrics_path: /scrape
    params:
      model: ['dell']
      credential_profile: ['tenant']
    relabel_configs:
      - source_labels: [__address__]
        target_label: __param_target
      - source_labels: [customer_id]
        target_label: __param_customer_id
      - source_labels: [region]
        target_label: __param_region
      - source_labels: [__param_target]
        target_label: instance
      - target_label: __address__
        replacement: fishymetrics:10023
```

## Credential Profile Configuration

When using URL extra parameters, configure your credential profiles to include placeholders:

### YAML Configuration

```yaml
credentials:
  profiles:
    - name: dynamic
      mountPath: "kv2"
      path: "bmc/{environment}/{rack}/credentials"
      userField: "username"
      passwordField: "password"
```

### JSON Configuration

```json
{
  "profiles": [
    {
      "name": "dynamic",
      "mountPath": "kv2",
      "path": "bmc/{environment}/{rack}/credentials",
      "userField": "username",
      "passwordField": "password"
    }
  ]
}
```
