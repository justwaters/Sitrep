# Sitrep

Remote systems status reporting for Linux, secured with mutual TLS (mTLS). A central **manager** agent collects periodic status reports from one or more **worker** agents and exposes the results over a local JSON API so other software on the same machine can query them.

Every report includes an epoch timestamp and covers whichever of these checks the manager has enabled:

| Check | Name | What it does |
|---|---|---|
| Internet Access | `internet` | Shells out to the system `ping` binary against `8.8.8.8` |
| Disk Space | `disk` | Usage of the root filesystem (`/`) |
| DRAM Usage | `dram` | System memory usage |
| CPU Usage | `cpu` | Aggregate CPU utilization, sampled over a short window |
| Git Auth Status | `gitauth` | Runs `gh auth status` |

## Contents

- [Architecture](#architecture)
- [Requirements](#requirements)
- [Install](#install)
- [Set up the manager](#set-up-the-manager)
- [Enroll a worker](#enroll-a-worker)
- [Local query API](#local-query-api)
- [Configuration files](#configuration-files)
- [Managing the services](#managing-the-services)
- [Design tradeoffs](#design-tradeoffs)
- [Development](#development)

## Architecture

```
                 one-time enrollment token
                 + pinned cert fingerprint
   operator  ───────────────────────────────▶  worker wizard
                                                     │
                                                     │ CSR + token, over TLS
                                                     │ (pinned to manager's
                                                     │  server cert fingerprint)
                                                     ▼
                                          ┌─────────────────────┐
                                          │   manager: enroll    │  :8444  (no client cert required)
                                          │      listener        │
                                          └──────────┬───────────┘
                                                     │ issues client cert,
                                                     │ returns CA cert + config
                                                     ▼
   worker  ───── mTLS report (interval) ──▶ ┌─────────────────────┐
                                          │   manager: report     │  :8443  (RequireAndVerifyClientCert)
                                          │      listener         │
                                          └──────────┬───────────┘
                                                     │ Upsert(report)
                                                     ▼
                                          ┌─────────────────────┐
   other software  ◀── loopback JSON ──── │  manager: local API   │  127.0.0.1:8080
   on the same host                       └─────────────────────┘
```

- **The manager is its own certificate authority.** On first startup it generates a root CA and a server certificate — no external PKI required.
- **Enrollment is a one-time handshake.** The manager issues a short-lived, single-use token. The worker generates its own keypair locally (the private key never leaves the worker), presents the token plus a CSR over a bootstrap TLS connection pinned to the manager's server certificate fingerprint, and the manager signs and returns a client certificate. All subsequent traffic between that worker and the manager is full mTLS.
- **Two separate listeners, two separate ports.** The enrollment endpoint requires no client certificate; the report endpoint requires and verifies one (`tls.RequireAndVerifyClientCert`). This is a hard security boundary enforced by Go's TLS stack before any handler code runs — not a per-route check.
- **Config changes propagate through report acknowledgements.** There's no separate push channel: every report response carries the manager's *current* interval and enabled-checks list, and the worker self-adjusts (and persists the change locally) whenever those differ from what it's currently using.
- **No persistent history.** The manager keeps only the *latest* report per worker, in memory. This is a live-status view, not a trend/history store — restarting the manager clears it.
- **Both agents run as systemd services**, installed by their own setup wizards.

## Requirements

- Linux with systemd (the wizards install and manage `systemd` units)
- Root privileges for first-run setup (creates a dedicated `sitrep` system user, writes to `/etc/sitrep` and `/etc/systemd/system`)
- `ping` and `gh` on worker machines, for the `internet` and `gitauth` checks respectively — the worker wizard detects missing binaries and offers to install them via `apt`/`dnf`/`pacman`

## Install

Build from source (requires Go 1.27+):

```sh
git clone https://github.com/justwaters/Sitrep.git
cd Sitrep
make linux                 # cross-compiles bin/sitrep-linux-{amd64,arm64}
```

Copy the binary for your architecture to each target machine and put it on `PATH`, e.g.:

```sh
scp bin/sitrep-linux-amd64 myhost:/tmp/sitrep
ssh myhost 'sudo install -m 0755 /tmp/sitrep /usr/local/bin/sitrep'
```

(`make build` also exists for a host-platform build, useful for `go vet`/`go test` during development, but isn't the artifact you'd deploy — see [Development](#development).)

## Set up the manager

On the machine that will collect reports:

```sh
sudo sitrep manager start
```

This is a one-time interactive wizard:

1. **What do you want to call this machine?** — the manager's name, embedded in its CA and server certificate and shown in `token create` output.
2. Confirm or edit the listen addresses:
   - **Report listen address** (mTLS, default `0.0.0.0:8443`) — where workers send reports
   - **Enrollment listen address** (mTLS bootstrap, default `0.0.0.0:8444`) — where workers enroll
   - **Advertised address** — the hostname/IP workers should use to reach this manager (embedded in the server certificate's SANs and printed by `token create`)
   - **Local query API address** (default `127.0.0.1:8080`) — must be a loopback address
3. Choose which checks to enable (all five by default)
4. Set the reporting interval (Go duration syntax, e.g. `30s`, `1m`, `5m`)

The wizard then creates the `sitrep` system user, writes `/etc/sitrep/manager/config.yaml`, and installs + starts `sitrep-manager.service`. The manager generates its CA and server certificate itself on first actual startup (as the `sitrep` user).

Check it's running:

```sh
sitrep manager status
# or
systemctl status sitrep-manager
```

## Enroll a worker

**On the manager**, generate a one-time enrollment token for the new worker:

```sh
sudo sitrep manager token create
```

```
Enrollment token created for manager "prod-manager". Give the worker operator all four of these values:
  Manager address:  10.0.0.5:8444
  Token:            sitrep_3f9a...
  Expires:          2026-09-03T15:32:10Z
  Cert fingerprint: 8f2c1e...
```

The token is single-use and expires (15 minutes by default; override with `--ttl 1h`). If the manager restarts before the token is used, generate a new one — tokens are held in memory only (see [Design tradeoffs](#design-tradeoffs)).

**On the worker machine**, run:

```sh
sudo sitrep worker start
```

The wizard:

1. Asks **what you want to call this machine** — this is the name the manager will report it under.
2. Checks for `ping`/`gh` and, if either is missing, shows the exact install command for your distro's package manager (`apt`/`dnf`/`pacman`) and only runs it on explicit confirmation — nothing is installed silently.
3. Asks for the manager address, token, and certificate fingerprint from the previous step.
4. Performs the enrollment handshake, writes `/etc/sitrep/worker/config.yaml` plus the issued client certificate and the manager's CA certificate.
5. Installs and starts `sitrep-worker.service`.

The worker begins reporting immediately at the interval the manager pushed at enrollment, and will pick up any later interval/check changes automatically.

## Local query API

Plain HTTP+JSON, bound strictly to loopback (`127.0.0.1` by default) — no authentication or client certificate needed since access is already restricted to processes on the same machine.

### `GET /v1/workers`

List every known worker.

```sh
curl http://127.0.0.1:8080/v1/workers
```

```json
[
  {
    "id": "wkr_3f9ab2c1d4e5f6a7",
    "name": "web-01",
    "enrolled_at": 1767450000,
    "last_seen": 1767450300
  }
]
```

### `GET /v1/workers/{id}`

A single worker plus its most recent report. `404` if unknown.

```sh
curl http://127.0.0.1:8080/v1/workers/wkr_3f9ab2c1d4e5f6a7
```

```json
{
  "id": "wkr_3f9ab2c1d4e5f6a7",
  "name": "web-01",
  "enrolled_at": 1767450000,
  "last_seen": 1767450300,
  "last_report": {
    "worker_id": "wkr_3f9ab2c1d4e5f6a7",
    "name": "web-01",
    "timestamp": 1767450300,
    "stats": {
      "internet": { "ok": true, "value": { "reachable_ms": 14, "packet_loss": false } },
      "disk":     { "ok": true, "value": { "path": "/", "total_bytes": 107374182400, "used_bytes": 42949672960, "used_percent": 40.0 } },
      "dram":     { "ok": true, "value": { "total_bytes": 16777216000, "used_bytes": 8388608000, "used_percent": 50.0 } },
      "cpu":      { "ok": true, "value": { "used_percent": 12.5 } },
      "gitauth":  { "ok": true, "value": { "logged_in": true, "account": "someuser" } }
    }
  }
}
```

A check that failed to run (e.g. `gh` not installed) reports `"ok": false` with an `"error"` message instead of `"value"`.

### `GET /v1/config`

The manager's current live configuration.

```sh
curl http://127.0.0.1:8080/v1/config
```

```json
{
  "name": "prod-manager",
  "listen_addr": "0.0.0.0:8443",
  "api_listen_addr": "127.0.0.1:8080",
  "interval_seconds": 60,
  "enabled_checks": ["internet", "disk", "dram", "cpu", "gitauth"]
}
```

### `PATCH /v1/config`

Update the reporting interval and/or enabled checks. Omitted fields are left unchanged. The change is persisted to `config.yaml` immediately and rolls out to every enrolled worker within one reporting interval — no manager restart and no per-worker action required.

```sh
curl -X PATCH http://127.0.0.1:8080/v1/config \
  -d '{"interval_seconds": 30, "enabled_checks": ["disk", "cpu"]}'
```

Returns the updated `ConfigResponse`. `interval_seconds` must be positive (`400` otherwise).

### `POST /v1/tokens`

Create a one-time worker enrollment token. This is what `sitrep manager token create` calls under the hood; call it directly if you're scripting worker provisioning.

```sh
curl -X POST http://127.0.0.1:8080/v1/tokens -d '{"ttl_seconds": 3600}'
```

```json
{
  "token": "sitrep_3f9a...",
  "expires_at": 1767453600,
  "enroll_addr": "10.0.0.5:8444",
  "server_cert_fingerprint": "8f2c1e...",
  "manager_name": "prod-manager"
}
```

`ttl_seconds` is optional (defaults to 15 minutes).

## Configuration files

| Path | Owner | Contents |
|---|---|---|
| `/etc/sitrep/manager/config.yaml` | `sitrep` | Listen addresses, advertised host, interval, enabled checks |
| `/etc/sitrep/manager/ca.{crt,key}.pem` | `sitrep` | The manager's self-signed root CA |
| `/etc/sitrep/manager/server.{crt,key}.pem` | `sitrep` | The manager's server certificate (auto-renewed within 30 days of expiry) |
| `/etc/sitrep/worker/config.yaml` | `sitrep` | Worker ID, manager address, current interval/enabled checks (cached from the manager) |
| `/etc/sitrep/worker/client.{crt,key}.pem` | `sitrep` | This worker's client certificate |
| `/etc/sitrep/worker/ca.crt.pem` | `sitrep` | The manager's CA certificate, for verifying the manager on report connections |

All directories are `0700`; private keys are `0600`; certificates are `0644`.

## Managing the services

```sh
systemctl status sitrep-manager      # or sitrep-worker
systemctl restart sitrep-manager
journalctl -u sitrep-manager -f      # follow logs
```

`sitrep manager status` / `sitrep worker status` are thin wrappers around `systemctl is-active` for convenience.

To re-run setup from scratch on a machine, stop the service, remove its data directory (`/etc/sitrep/manager` or `/etc/sitrep/worker`), and run `sitrep manager start` / `sitrep worker start` again.

## Design tradeoffs

These are deliberate, not oversights — noted here so they don't surprise you in production:

- **Enrollment tokens are in-memory only.** A manager restart invalidates any outstanding, unused token; re-run `sitrep manager token create` (or `POST /v1/tokens`) and retry. Tokens that have already been consumed are unaffected — enrolled workers keep working.
- **No automated certificate rotation.** CA certs are valid 10 years; manager server certs are valid 2 years and silently auto-renewed on startup within 30 days of expiry; worker client certs are valid 1 year with **no** auto-renewal. An expired worker cert simply fails its next mTLS handshake — the fix is re-enrolling that worker (`sitrep worker start` again with a fresh token), not a background renewal protocol.
- **No persistent report history.** The manager only ever holds each worker's *latest* report, in memory. If you need trend data, poll `GET /v1/workers/{id}` from your own tooling and store it yourself.

## Development

```sh
make build   # bin/sitrep for the host platform (dev/test convenience, not the deploy artifact)
make linux   # cross-compiled bin/sitrep-linux-{amd64,arm64} — what you actually ship
make test    # go test ./...
make vet     # go vet ./...
make fmt     # gofmt -l . (lists any unformatted files)
```

`go test ./...` covers PKI/cert issuance, the enrollment token store, all five stat checkers, the mTLS handshake (client-cert required vs. not, fingerprint pinning), the worker registry under `-race`, and the local API's handlers. Actually installing systemd units (`sysd`, `useradd`, `apt`/`dnf`/`pacman`) is Linux-only and can only be exercised on a real Linux host, VM, or systemd-enabled container — not in `go test`.

Manual end-to-end check on a Linux box:

1. `sudo sitrep manager start` → wizard → `systemctl status sitrep-manager` should be `active`.
2. `sudo sitrep manager token create` → copy the four printed values.
3. On the same or another machine, `sudo sitrep worker start` → wizard → `systemctl status sitrep-worker` should be `active`.
4. After one interval: `curl http://127.0.0.1:8080/v1/workers` on the manager should show the worker with a fresh `last_report`.
