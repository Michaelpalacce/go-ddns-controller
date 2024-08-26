Prepartion for v1.0.0 checklist

# Public IP Fetcher

- [ ] New CRD that is meant to fetch the public IP address of the cluster. The CRD will be updated by a controller on intervals. If a change is detected, the providers will be trigerred and updated.

# Tests

- [ ] Finalize e2e tests. Use gitignored secrets for the tests to connect to the DNS provider. Create a local webhook for tests.
- [ ] Write unit tests for the notifier
- [ ] Unit test the Provider Clients

# Documentation

- [ ] Write better documentation in a `docs` folder perhaps?

# Notifiers

- [ ] Add Telegram as an available notifier
- [ ] Add Slack as an available notifier
- [ ] Add Discord as an available notifier
- [ ] Add Microsoft Teams as an available notifier
- [ ] Add Email as an available notifier
- [ ] Integrate apprise as a notifier

# Providers

- [ ] Add DuckDNS as an available provider
- [ ] Add DigitalOcean as an available provider
- [ ] Add Google Domains as an available provider
- [ ] Add AliDNS as an available provider
- [ ] Add Linode as an available provider
- [ ] Add Scaleway as an available provider
- [ ] Add No-IP as an available provider
- [ ] Add Hetzner as an available provider
- [ ] Add Dynu as an available provider
- [ ] Add OVH as an available provider
- [ ] Add DNSPod as an available provider
- [ ] Add Strato as an available provider
