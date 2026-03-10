# bndry

A friendly CLI for HashiCorp Boundary that makes SSH access simple. Connect to targets using friendly names instead of IDs, with Teleport-style `user@target` syntax.

```bash
bndry ssh kmendell@webserver-01
```

## Features

- **Teleport-style SSH** — `user@target` syntax for easy connections
- **Name-based access** — no more hunting for `ttcp_...` IDs
- **One-command setup** — `bndry add 10.0.0.5` creates everything automatically
- **Interactive workflows** — smart prompts when you need them
- **Zero dependencies** — uses the official Boundary Go SDK (no `boundary` CLI or `jq` required)

## Installation

### Homebrew

```bash
brew install ofkm/tap/bndry
```

### Go Install

```bash
go install github.com/ofkm/bndry/cmd/bndry@latest
```

### From Source

```bash
git clone https://github.com/ofkm/bndry.git
cd bndry
go build -o bndry ./cmd/bndry
```

### Pre-built Binaries

Download from [GitHub Releases](https://github.com/ofkm/bndry/releases)

## Quick Start

```bash
# 1. Login
bndry login

# 2. Add a target
bndry add 192.168.1.100 -n my-server

# 3. Connect
bndry ssh kmendell@my-server
```

## Commands

### `bndry login`

Authenticate with Boundary via OIDC.

### `bndry add <ip> [flags]`

Quickly create an SSH target.

**Flags:**

- `-n, --name` — Target name (auto-generated if omitted)
- `-p, --port` — SSH port (default: 22)
- `-g, --group` — Host set name
- `--no-connect` — Don't auto-connect after creation

**Example:**

```bash
bndry add 10.0.0.5 -n webserver -p 22
```

### `bndry setup`

Interactive guided setup for creating targets.

```bash
bndry setup
```

### `bndry ssh [user@]target`

Connect to a target via SSH.

```bash
# Teleport-style with username
bndry ssh kmendell@webserver-01

# Use Boundary credentials
bndry ssh webserver-01

# Interactive selection
bndry ssh

# List all targets
bndry ssh list

# Inspect target
bndry ssh inspect webserver-01
```

### `bndry config`

Manage configuration.

```bash
bndry config show  # Show config
bndry config path  # Show config file location
bndry config init  # Create new config
```

## Configuration

Config file: `~/.config/bndry/config.yaml` (or set `BNDRY_CONFIG`)

**Create config:**

```bash
bndry config init
```

**Example config:**

```yaml
boundary_addr: https://boundary.example.com
oidc_auth_method_id: amoidc_1234567890
auth_token: at_token_here
default_project_scope_id: p_prod01
default_catalog_name: Homelab Static Hosts
default_host_set_name: linux-servers
default_role_name: linux-ssh-users
default_target_port: 22
default_create_role: true
default_connect_after_create: true
```

**Environment variables** (override config with `BNDRY_` prefix):

```bash
export BNDRY_BOUNDARY_ADDR=https://boundary.example.com
export BNDRY_DEFAULT_TARGET_PORT=2222
```

## Troubleshooting

- **"No host sources found"** → `bndry ssh inspect <target>` to check target configuration
- **"Authentication failed"** → `bndry login` to refresh token
- **"Permission denied"** → Check Boundary role grants for authorize-session permission
- **Can't find config** → `bndry config path` to verify location, or `bndry config init` to create

## Development

```bash
# Build
go build -o bndry ./cmd/bndry

# Test
go test ./...

# Or use just
just build
just test
```

**Release:** Tag with `git tag v1.0.0 && git push origin v1.0.0`. GoReleaser builds binaries and updates [Homebrew tap](https://github.com/ofkm/homebrew-tap).

## License

Apache 2.0
