# Maxtap CPA Usage Cutover Runbook

Date: 2026-05-17

## Goal

Make this repository's `cpa-usage` service the only CPA Usage owner on maxtap. The old `cpa-usage-keeper` is not a rollback service; it is backed up and stopped.

## Fixed Context

- Compose directory: `/etc/dokploy/compose/cpa-cliproxyapi-hazmcp/code`
- Public host: `https://cpa.maxtap.net`
- New app path: `/cpa-usage`
- Old keeper path: `/usage`
- Old keeper data volume: `cpa-cliproxyapi-hazmcp_cpa-usage-keeper-data`
- New app data volume: `cpa-cliproxyapi-hazmcp_cpa-usage-data`

## Current Cutover Evidence

- Keeper was stopped on 2026-05-17.
- Keeper backup: `/root/cpa-usage-cutover-backups/20260517T152241Z/cpa-usage-keeper-data.tgz`
- Backup SHA256: `d0091456fc28b8e8a2bcfc8781223bec8ff233039836d88906916f53901f6506`
- `https://cpa.maxtap.net/usage/healthz`: 404 after keeper shutdown.
- `https://cpa.maxtap.net/`: 200 after keeper shutdown.

## Preflight

```sh
cd /etc/dokploy/compose/cpa-cliproxyapi-hazmcp/code
sha256sum docker-compose.yml
docker compose config --services
docker compose ps postgres cliproxyapi cpa-usage-keeper cpa-usage || true
docker ps -a --format '{{.Names}}|{{.Image}}|{{.Status}}' | grep -E 'cpa-usage|cliproxyapi|postgres' || true
docker inspect cpa-cliproxyapi-hazmcp-cpa-usage-keeper-1 --format '{{range .Mounts}}{{.Name}}|{{.Source}}|{{.Destination}}{{println}}{{end}}' 2>/dev/null || true
```

Public checks:

```sh
curl -k -i https://cpa.maxtap.net/
curl -k -i https://cpa.maxtap.net/usage/healthz || true
curl -k -i https://cpa.maxtap.net/cpa-usage/healthz || true
curl -k -i https://cpa.maxtap.net/cpa-usage/ || true
```

## Stop Old Keeper And Back Up Data

This step has already been executed once. Re-run only if the keeper is running again.

```sh
cd /etc/dokploy/compose/cpa-cliproxyapi-hazmcp/code
stamp=$(date -u +%Y%m%dT%H%M%SZ)
backup_dir=/root/cpa-usage-cutover-backups/$stamp
mkdir -p "$backup_dir"

docker compose stop cpa-usage-keeper

keeper_data=/var/lib/docker/volumes/cpa-cliproxyapi-hazmcp_cpa-usage-keeper-data/_data
tar -C "$keeper_data" -czf "$backup_dir/cpa-usage-keeper-data.tgz" .
sha256sum "$backup_dir/cpa-usage-keeper-data.tgz" > "$backup_dir/SHA256SUMS"
cat "$backup_dir/SHA256SUMS"
docker compose ps cpa-usage-keeper || true
```

After this point, `/usage` downtime is expected until `cpa-usage` is online.

## Prepare New Data Volume

Use the keeper backup as the source of truth. The simplest cutover path is to restore the keeper data directory into `cpa-usage-data` and let the new binary run its migrations on startup.

```sh
backup=/root/cpa-usage-cutover-backups/20260517T152241Z/cpa-usage-keeper-data.tgz
target=/var/lib/docker/volumes/cpa-cliproxyapi-hazmcp_cpa-usage-data/_data

mkdir -p "$target"
tar -C "$target" -czf "/root/cpa-usage-cutover-backups/pre-new-volume-$(date -u +%Y%m%dT%H%M%SZ).tgz" . 2>/dev/null || true
rm -rf "$target"/*
tar -C "$target" -xzf "$backup"
```

Do not commit copied database files or secrets to the repository.

## Add New Service

Use [deploy/maxtap/cpa-usage.cutover.compose.yml](../../deploy/maxtap/cpa-usage.cutover.compose.yml) as the service definition source.

Set runtime secrets on maxtap, not in git:

```sh
export CPA_USAGE_IMAGE=ghcr.io/evenluo/cpa-usage:<immutable-commit-sha-tag>
export CPA_MANAGEMENT_KEY='<secret>'
export LOGIN_PASSWORD='<secret>'
```

Merge the service into the Dokploy compose file or run with an explicit overlay after copying the overlay to maxtap:

```sh
docker compose -f docker-compose.yml -f cpa-usage.cutover.compose.yml config
docker compose -f docker-compose.yml -f cpa-usage.cutover.compose.yml up -d cpa-usage
docker compose -f docker-compose.yml -f cpa-usage.cutover.compose.yml ps cpa-usage
```

If the overlay is merged into `docker-compose.yml`, use plain `docker compose up -d cpa-usage`.

## Smoke

From a trusted machine:

```sh
BASE_URL=https://cpa.maxtap.net \
BASE_PATH=/cpa-usage \
LOGIN_PASSWORD='<secret>' \
EXPECT_KEEPER_STOPPED=true \
scripts/smoke-cpa-usage.sh
```

Manual checks:

```sh
curl -k -i https://cpa.maxtap.net/cpa-usage/healthz
curl -k -i https://cpa.maxtap.net/cpa-usage/
curl -k -i https://cpa.maxtap.net/
curl -k -i https://cpa.maxtap.net/usage/healthz || true
```

Expected:

- `/cpa-usage/healthz`: 200
- `/cpa-usage/`: 200 HTML or login shell
- `/`: 200 CPA root JSON
- `/usage/healthz`: not 200 after keeper shutdown

## Recovery

Do not restart `cpa-usage-keeper` as a normal rollback path. Recovery should use:

- the keeper backup tarball,
- a corrected `cpa-usage` image,
- and a redeploy of `cpa-usage`.

Only touch `cliproxyapi` or `postgres` if direct evidence points to those services.
