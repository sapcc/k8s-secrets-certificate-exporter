# Kubernetes Secret Certificate Exporter

Exports expiry metrics for certificates found in kubernetes secrets.

## Features

  - Automatically discovers secrets via Kubernetes API
  - Exposes Prometheus metrics

## Requirements

  - go 1.11

## Example metrics

```
# HELP secrets_exporter_certificate_not_after How long the certificate is valid.
# TYPE secrets_exporter_certificate_not_after gauge

secrets_exporter_certificate_not_after{host="",name="ca.crt",secret="default/my-secret"} 1.806907249e+09
secrets_exporter_certificate_not_after{host="",name="tls.crt",secret="default/my-secret"} 1.806907249e+09

# HELP secrets_exporter_certificate_not_before How long the certificate is valid.
# TYPE secrets_exporter_certificate_not_before gauge
secrets_exporter_certificate_not_before{host="",name="ca.crt",secret="default/my-secret"} 1.491547249e+09
secrets_exporter_certificate_not_before{host="",name="tls.crt",secret="default/my-secret"} 1.491547249e+09
```