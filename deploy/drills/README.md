# Non-production failure proxy

This Compose project is exclusively for controlled non-production drills. Set `DRILL_DATABASE_UPSTREAM` and `DRILL_PROVIDER_UPSTREAM` to `host:port` targets reachable from the proxy, then start it with:

```bash
docker compose -f deploy/drills/compose.example.yaml up -d fault-proxy
docker compose -f deploy/drills/compose.example.yaml run --rm configure-postgres
docker compose -f deploy/drills/compose.example.yaml run --rm configure-provider
```

Point a disposable application deployment at `fault-proxy:15432` and the proxied provider endpoint at `fault-proxy:18080`. A systemd-hosted test instance can instead use the loopback-published ports. Never place production credentials or traffic behind this proxy.

Use the control profile to inject and remove failures:

```bash
# Database outage
docker compose -f deploy/drills/compose.example.yaml --profile control run --rm control -host fault-proxy:8474 toggle ecommerce-postgres

# Provider timeout storm
docker compose -f deploy/drills/compose.example.yaml --profile control run --rm control -host fault-proxy:8474 toxic add -t timeout -a timeout=1000 ecommerce-provider
docker compose -f deploy/drills/compose.example.yaml --profile control run --rm control -host fault-proxy:8474 toxic remove -n timeout_downstream ecommerce-provider
```

Follow `wiki/Failure-Drills.md` for safety checks, worker-crash execution, expected alerts, recovery, and evidence requirements.
