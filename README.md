# bndry

`bndry` is a friendly CLI wrapper for HashiCorp Boundary focused on making SSH workflows simple and fast. It replaces ID hunting and multi-step workflows with interactive prompts, smart defaults, and human-readable names.

## Features

- **Interactive login** with saved defaults and OIDC browser flow
- **Universal config file** (Viper-backed) for reusable defaults across all commands
- **Smart target selection** via menus instead of memorizing IDs
- **One-command target creation** — `bndry add 10.0.0.5` does all the wiring automatically
- **Guided setup wizard** for creating catalog → host → host set → target → role chains
- **Name-based SSH** — connect by friendly name instead of `ttcp_...` IDs
- **Target inspection** to diagnose broken host sources and host wiring
- **Cross-scope search** to find targets in any project

Built with:

- [Cobra](https://github.com/spf13/cobra) for command structure
- [Bubble Tea v2](https://github.com/charmbracelet/bubbletea) for interactive prompts
- [Lip Gloss](https://github.com/charmbracelet/lipgloss) for styled output
- [Official Boundary Go SDK](https://github.com/hashicorp/boundary/tree/main/api) for all API operations

## Table of Contents

- [Prerequisites](#prerequisites)
- [Installation](#installation)
- [First-Time Setup](#first-time-setup)
- [Command Reference](#command-reference)
  - [bndry login](#bndry-login)
  - [bndry add](#bndry-add)
  - [bndry setup](#bndry-setup)
  - [bndry ssh](#bndry-ssh)
  - [bndry targets](#bndry-targets)
  - [bndry scopes](#bndry-scopes)
  - [bndry config](#bndry-config)
- [Configuration](#configuration)
  - [Config File Location](#config-file-location)
  - [Config Fields Reference](#config-fields-reference)
  - [Environment Variables](#environment-variables)
- [Workflows](#workflows)
  - [Quick Add New Target](#quick-add-new-target)
  - [Guided Setup for First Target](#guided-setup-for-first-target)
  - [Connect to Existing Target](#connect-to-existing-target)
  - [Troubleshoot Broken Target](#troubleshoot-broken-target)
- [Troubleshooting](#troubleshooting)
- [Development](#development)

## Prerequisites

Before using `bndry`, you need:

1. **A running Boundary deployment:**
   - Boundary controller accessible via HTTPS
   - At least one worker registered and connected
   - Network connectivity from your machine to the controller

2. **OIDC authentication configured:**
   - An OIDC auth method set up in Boundary (e.g., Azure AD, Okta, Google Workspace)
   - Your user account granted access via the OIDC provider
   - The auth method ID (e.g., `amoidc_xxxxx`) or configured as the default

3. **A project scope:**
   - At least one project scope where you can create resources
   - Permissions to create hosts, host catalogs, host sets, targets, and optionally roles

4. **SSH client:**
   - Standard `ssh` binary installed and available on your `PATH`
   - Used for the actual SSH connection after Boundary authorizes the session

**Note:** Unlike shell-script wrappers, `bndry` does **not** require the `boundary` CLI or `jq`. It uses the official Boundary Go SDK directly.

## Installation

### Homebrew (macOS/Linux)

The easiest way to install `bndry` is via Homebrew:

```bash
# Add the tap
brew tap ofkm/tap

# Install bndry
brew install bndry
```

### Build from Source

If you prefer to build from source:

```bash
# Clone the repository
git clone https://github.com/ofkm/bndry.git
cd bndry

# Build the binary
go build -o bin/bndry ./cmd/bndry

# Optional: move to your PATH
sudo mv bin/bndry /usr/local/bin/
```

Or using the included `Justfile`:

```bash
just build
```

### Download Pre-built Binaries

Pre-built binaries for macOS and Linux are available on the [releases page](https://github.com/ofkm/bndry/releases).

### Verify Installation

```bash
bndry --help
```

You should see the main help screen with all available commands.

## First-Time Setup

After installing `bndry`, follow these steps to get started:

### 1. Login to Boundary

```bash
bndry login
```

You'll be prompted for:

- **Boundary controller URL** (e.g., `https://boundary.example.com`)
- **OIDC auth method ID** (optional if your controller has a default OIDC provider)

The command will:

1. Save your controller URL to the config file
2. Open your browser to complete the OIDC login flow
3. Wait for authentication to complete
4. Save the resulting auth token to the config file

**Example:**

```
→ Boundary controller address: https://boundary.example.com
→ OIDC auth method ID (leave blank to use server default): amoidc_1234567890
→ Opening browser for OIDC authentication...
→ Waiting for authentication to complete...
✓ Authentication successful
✓ Config saved to /Users/you/.config/bndry/config.yaml
```

### 2. Create Your First Target

You have two options:

#### Option A: Quick Add (Fast)

```bash
bndry add 192.168.1.100 -n my-server
```

This creates everything automatically using config defaults.

#### Option B: Guided Setup (Interactive)

```bash
bndry setup
```

This walks you through all the steps interactively, letting you customize names and settings.

### 3. Connect to Your Target

```bash
bndry ssh my-server
```

That's it! You're now SSHed into your target via Boundary.

## Command Reference

### `bndry login`

Authenticate with your Boundary controller using OIDC.

**Usage:**
```bash
bndry login
```

**Interactive prompts:**
1. Boundary controller address (e.g., `https://boundary.example.com`)
2. OIDC auth method ID (optional—leave blank to use the server's default OIDC provider)

**What it does:**
- Saves the controller URL to your config file
- Initiates an OIDC authentication flow
- Opens your browser to complete the login
- Polls Boundary for the auth token
- Saves the token to your config file

**Example:**
```bash
$ bndry login
→ Boundary controller address: https://boundary.mycompany.com
→ OIDC auth method ID (leave blank to use server default): 
→ Opening browser for OIDC authentication...
✓ Authentication successful
```

**Notes:**
- The auth token is saved in plaintext in your config file (`~/.config/bndry/config.yaml`)
- Token expiration depends on your Boundary controller settings
- Re-run `bndry login` when your token expires

---

### `bndry add`

Quickly add a new SSH target with automatic resource creation.

**Usage:**
```bash
bndry add [ip-address] [flags]
```

**Flags:**
- `-n, --name` — Target name (auto-generated from IP if omitted)
- `-p, --port` — Target port (defaults to `default_target_port` from config, or 22)
- `-g, --group` — Host set name (defaults to `default_host_set_name` from config)
- `--catalog` — Host catalog name (defaults to `default_catalog_name` from config)
- `--no-connect` — Skip automatic connection after creation

**What it does automatically:**
1. Creates or reuses a static host catalog
2. Creates a static host with the provided IP address
3. Creates or reuses a host set
4. Adds the host to the host set
5. Creates a TCP target with the specified port
6. Attaches the host set to the target
7. Optionally creates a role grant (if `default_create_role` is true)
8. Optionally connects to the target via SSH (if `default_connect_after_create` is true and `--no-connect` is not set)

**Examples:**

```bash
# Simplest form (name auto-generated as "host-10-0-0-5")
bndry add 10.0.0.5

# With custom name
bndry add 192.168.1.100 -n web-server

# Custom name and port
bndry add 192.168.1.200 -n db-server -p 5432

# Organize into a specific group/host set
bndry add 172.18.24.10 -g production-web -n web-01

# Create without connecting
bndry add 10.0.0.50 -n jumpbox --no-connect
```

**Notes:**
- If `default_project_scope_id` is not set in your config, you'll be prompted to select a project scope
- Host catalog and host set are reused across multiple `add` commands if they have the same name
- Auto-generated names follow the pattern `host-X-X-X-X` (IP octets separated by hyphens)

---

### `bndry setup`

Interactive guided workflow to create a complete SSH target from scratch.

**Usage:**
```bash
bndry setup
# or
bndry setup ssh
```

**What it does:**
- Walks you through creating all required Boundary resources step by step
- Allows full customization of all names and settings
- Creates: host catalog → host → host set → target
- Optionally creates a role grant for access control

**Interactive prompts:**
1. Project scope selection
2. Host catalog name
3. Host name
4. Host address (IP or DNS)
5. Host set name
6. Target name
7. Target port
8. Whether to create a role grant
9. (If yes) Role name and principal ID
10. Whether to use the selected scope as your default
11. Whether to connect immediately after creation

**Example flow:**
```bash
$ bndry setup

  Choose the project scope
  
  › My Production Env (p_abc123)
    Development (p_def456)
    Staging (p_ghi789)

→ Static host catalog name: Homelab Hosts
→ Host name: webserver-01
→ Host address or DNS name: 192.168.1.100
→ Host set name: web-servers
→ Target name: webserver-01
→ Target port: 22
→ Create a role grant for this target now? Yes
→ Role name: ssh-users
→ Principal ID to add (optional): mgoidc_abc123
→ Use this project scope as your default scope? Yes
→ Creating Boundary resources...
✓ Boundary resources created
  Catalog:  Homelab Hosts (hcst_xyz...)
  Host:     webserver-01 (hst_abc... -> 192.168.1.100)
  Host set: web-servers (hsst_def...)
  Target:   webserver-01 (ttcp_ghi...)
  Role:     ssh-users (r_jkl...)
→ Connect to the new target now? Yes
```

**Notes:**
- Use `bndry add` for faster target creation if you're happy with defaults
- Use `bndry setup` when you need full control over naming and configuration
- Settings chosen during setup can be saved as defaults in your config file

---

### `bndry ssh`

Connect to a Boundary target by friendly name, or manage SSH connections.

**Usage:**
```bash
# Connect to a target by name
bndry ssh [target-name]

# List all available targets
bndry ssh list

# Inspect a target's configuration
bndry ssh inspect [target-name]
```

#### `bndry ssh [target-name]`

Connect to a target using its friendly name instead of the Boundary ID.

**Examples:**
```bash
# Connect to a specific target
bndry ssh webserver-01

# If no target name is provided, you'll be prompted to select one
bndry ssh
```

**What it does:**
1. Searches for targets matching the given name across all project scopes
2. If multiple matches found, prompts you to select one
3. Authorizes an SSH session via Boundary
4. Starts a local proxy listener
5. Launches your SSH client with the correct connection details

**Notes:**
- Name matching is case-insensitive
- Searches exact matches first, then falls back to case-insensitive matching
- The SSH session is routed through Boundary's worker network

#### `bndry ssh list`

List all targets available for SSH across all your project scopes.

**Usage:**
```bash
bndry ssh list
```

**Example output:**
```
Target              ID                Type    Scope                    Port
cobalt-ssh          ttcp_uJFU5W1C9P   tcp     OFKM (p_rwg0FqzcCK)      22
webserver-01        ttcp_abc123       tcp     Production (p_prod01)    22
db-server           ttcp_def456       tcp     Production (p_prod01)    5432
```

**Notes:**
- Shows targets from all project scopes you have access to
- Useful for discovering what's available before connecting
- Targets are sorted alphabetically by name

#### `bndry ssh inspect [target-name]`

Inspect a target's host sources, host catalogs, and backing hosts to diagnose connection issues.

**Usage:**
```bash
bndry ssh inspect [target-name]
```

**Example:**
```bash
$ bndry ssh inspect webserver-01

Target:   webserver-01 (ttcp_abc123)
Scope:    Production (p_prod01)
Type:     tcp
Port:     22
Sources:  1 host source

Host source 1
  Host set:    web-servers (hsst_xyz...)
  Catalog:     Homelab Hosts (hcst_abc...)
  Host:        webserver-01 (hst_def...)
               Address: 192.168.1.100
```

**If target is broken:**
```bash
$ bndry ssh inspect broken-target

Target:   broken-target (ttcp_ghi789)
Scope:    Production (p_prod01)
Type:     tcp
Port:     22
Sources:  none
! This target has no direct address and no host sources, so Boundary cannot connect to it yet.
```

**Notes:**
- Use this command when `bndry ssh` fails with "No host sources or address found"
- Shows you exactly what hosts are wired to the target
- Helps diagnose missing host sets or empty host sets

---

### `bndry targets`

List all targets in a selected project scope.

**Usage:**
```bash
bndry targets
```

**What it does:**
- Prompts you to select a project scope
- Lists all targets in that scope with their IDs and types

**Example output:**
```
Targets in Production (p_prod01):

webserver-01    ttcp_abc123    tcp
db-server       ttcp_def456    tcp
jumpbox         ttcp_ghi789    tcp
```

**Notes:**
- Unlike `bndry ssh list`, this command only shows targets from one scope at a time
- Useful when managing resources within a specific project

---

### `bndry scopes`

List all scopes recursively from the global scope.

**Usage:**
```bash
bndry scopes
```

**Example output:**
```
global (global)
└── My Organization (o_abc123)
    ├── Production (p_prod01)
    ├── Staging (p_staging02)
    └── Development (p_dev03)
```

**Notes:**
- Shows the full scope hierarchy
- Displays both scope names and IDs
- Useful for finding project scope IDs to save in your config

---

### `bndry config`

Manage your `bndry` configuration file.

**Usage:**
```bash
# Create a new config file with defaults
bndry config init

# Show the effective configuration
bndry config show

# Print the config file path
bndry config path
```

#### `bndry config init`

Create a new config file with default values.

**Usage:**
```bash
bndry config init
```

**What it does:**
- Creates `~/.config/bndry/config.yaml` (or the path specified by `BNDRY_CONFIG`)
- Populates it with built-in default values
- Prompts for confirmation before overwriting an existing file

**Notes:**
- Safe to run multiple times (won't overwrite without confirmation)
- Generated file includes comments explaining each setting

#### `bndry config show`

Print the effective configuration after applying defaults, file values, and environment overrides.

**Usage:**
```bash
bndry config show
```

**Example output:**
```yaml
boundary_addr: https://boundary.example.com
oidc_auth_method_id: amoidc_1234567890
auth_token: at_token_here_redacted
default_project_scope_id: p_prod01
default_catalog_name: Homelab Static Hosts
default_host_set_name: linux-servers
default_role_name: linux-ssh-users
default_principal_id: mgoidc_user123
default_target_port: 22
default_create_role: true
default_connect_after_create: true
auto_select_default_scope: false
last_host_catalog_id: hcst_xyz789
```

**Notes:**
- Shows the actual config that `bndry` is using
- Includes values from environment variables if set
- Useful for verifying your configuration is correct

#### `bndry config path`

Print the path to the config file currently in use.

**Usage:**
```bash
bndry config path
```

**Example output:**
```
/Users/you/.config/bndry/config.yaml
```

**Notes:**
- Useful for finding the config file to edit manually
- Respects the `BNDRY_CONFIG` environment variable

---

## Configuration

### Config File Location

`bndry` looks for its config file in the following order:

1. **Explicit path via environment variable:**
   ```bash
   export BNDRY_CONFIG=/path/to/custom/config.yaml
   bndry ssh list
   ```

2. **First existing file in `~/.config/bndry/`:**
   - `~/.config/bndry/config.yaml`
   - `~/.config/bndry/config.yml`
   - `~/.config/bndry/config.json`

3. **Default path (created on first use):**
   - `~/.config/bndry/config.yaml`

**Creating a config file:**
```bash
bndry config init
```

### Config Fields Reference

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `boundary_addr` | string | *(none)* | Boundary controller URL (e.g., `https://boundary.example.com`) |
| `oidc_auth_method_id` | string | *(none)* | OIDC auth method ID (e.g., `amoidc_xxxxx`); optional if server has a default |
| `auth_token` | string | *(none)* | Boundary auth token (set automatically by `bndry login`) |
| `default_project_scope_id` | string | *(none)* | Default project scope ID (e.g., `p_xxxxx`); skips scope selection when set |
| `default_catalog_name` | string | `"Homelab Static Hosts"` | Default host catalog name for `bndry add` |
| `default_host_set_name` | string | `"linux-servers"` | Default host set name for `bndry add` |
| `default_role_name` | string | `"linux-ssh-users"` | Default role name when creating role grants |
| `default_principal_id` | string | *(none)* | Default principal ID to add to new roles (e.g., `mgoidc_xxxxx`) |
| `default_target_port` | int | `22` | Default SSH port for new targets |
| `default_create_role` | bool | `true` | Whether to create role grants automatically |
| `default_connect_after_create` | bool | `true` | Whether to connect immediately after creating a target |
| `auto_select_default_scope` | bool | `false` | Skip scope selection if `default_project_scope_id` is set |
| `last_host_catalog_id` | string | *(none)* | Last used host catalog ID (managed automatically) |

**Example config:**

```yaml
boundary_addr: https://boundary.example.com
oidc_auth_method_id: amoidc_1234567890
auth_token: at_very_secret_token_here
default_project_scope_id: p_prod01

default_catalog_name: My Servers
default_host_set_name: ssh-hosts
default_role_name: ssh-access
default_principal_id: mgoidc_myuser
default_target_port: 22

default_create_role: true
default_connect_after_create: true
auto_select_default_scope: false
```

### Environment Variables

All config fields can be overridden with environment variables using the `BNDRY_` prefix:

| Environment Variable | Config Field |
|---------------------|--------------|
| `BNDRY_BOUNDARY_ADDR` | `boundary_addr` |
| `BNDRY_OIDC_AUTH_METHOD_ID` | `oidc_auth_method_id` |
| `BNDRY_AUTH_TOKEN` | `auth_token` |
| `BNDRY_DEFAULT_PROJECT_SCOPE_ID` | `default_project_scope_id` |
| `BNDRY_DEFAULT_CATALOG_NAME` | `default_catalog_name` |
| `BNDRY_DEFAULT_HOST_SET_NAME` | `default_host_set_name` |
| `BNDRY_DEFAULT_ROLE_NAME` | `default_role_name` |
| `BNDRY_DEFAULT_PRINCIPAL_ID` | `default_principal_id` |
| `BNDRY_DEFAULT_TARGET_PORT` | `default_target_port` |
| `BNDRY_DEFAULT_CREATE_ROLE` | `default_create_role` |
| `BNDRY_DEFAULT_CONNECT_AFTER_CREATE` | `default_connect_after_create` |
| `BNDRY_AUTO_SELECT_DEFAULT_SCOPE` | `auto_select_default_scope` |
| `BNDRY_CONFIG` | *(config file path)* |

**Example:**
```bash
export BNDRY_DEFAULT_TARGET_PORT=2222
bndry add 10.0.0.5  # Uses port 2222 instead of 22
```

**Precedence (highest to lowest):**
1. Environment variables (`BNDRY_*`)
2. Config file values
3. Built-in defaults

---

## Workflows

### Quick Add New Target

The fastest way to add a target when you already have `bndry` configured:

```bash
# Basic (auto-generated name)
bndry add 192.168.1.100

# With custom name
bndry add 192.168.1.100 -n my-server

# With custom port
bndry add 192.168.1.100 -n db-server -p 5432

# Organize into a group
bndry add 192.168.1.100 -n web-01 -g production-web
```

### Guided Setup for First Target

When setting up your first target or when you want full control:

```bash
bndry setup
```

This will:
1. Walk you through creating all resources interactively
2. Let you customize all names and settings
3. Optionally save your choices as defaults
4. Optionally connect immediately when done

### Connect to Existing Target

```bash
# By name
bndry ssh my-server

# Interactive selection
bndry ssh

# List all available targets first
bndry ssh list
bndry ssh my-server
```

### Troubleshoot Broken Target

If `bndry ssh my-server` fails with an error about missing host sources:

```bash
# Inspect the target
bndry ssh inspect my-server
```

This will show:
- Target metadata (ID, scope, port)
- All attached host sources
- Host sets and their hosts
- Host addresses

If you see `Sources: none`, the target isn't wired to any hosts or host sets.

---

## Troubleshooting

### "No host sources or address found for given target"

**Cause:** The target exists but isn't connected to any hosts or host sets.

**Solution:**
1. Inspect the target: `bndry ssh inspect <target-name>`
2. Verify the target has at least one host source
3. If `Sources: none`, you need to attach a host set to the target using the Boundary web UI or create a new target with `bndry add`

### "Authentication failed" or "Token expired"

**Cause:** Your auth token has expired or is invalid.

**Solution:**
```bash
bndry login
```

Log in again to get a fresh token.

### "Permission denied" errors

**Cause:** Your user doesn't have the required permissions in Boundary.

**Solution:**
- Ensure you have `authorize-session` permission on the target
- Check role grants in the Boundary web UI
- If you're creating resources, ensure you have admin permissions in the project scope

### Can't find config file

**Check where `bndry` is looking:**
```bash
bndry config path
```

**Create a new config:**
```bash
bndry config init
```

### Targets not showing up in `bndry ssh list`

**Possible causes:**
1. Targets are in a different scope than you expect
2. Your user doesn't have permissions to see those targets
3. Targets don't have the TCP type

**Debug:**
```bash
# List all scopes you have access to
bndry scopes

# Check targets in a specific scope
bndry targets
```

### `bndry add` creates duplicate resources

**Expected behavior:** `bndry add` reuses host catalogs and host sets with matching names.

**Each `bndry add` creates:**
- ✅ New host (unique per IP)
- ✅ New target (unique per name)
- ♻️ Reuses host catalog (by name)
- ♻️ Reuses host set (by name)

**To avoid duplicates:**
- Use consistent `--catalog` and `--group` names
- Or rely on the config defaults (`default_catalog_name`, `default_host_set_name`)

---

## Development

### Building from Source

```bash
go build -o bin/bndry ./cmd/bndry
```

### Running Tests

```bash
go test ./...
```

### Code Formatting

```bash
gofmt -w .
```

### Using Justfile

This project includes a `Justfile` for common tasks:

```bash
just build      # Build the binary
just test       # Run tests
just fmt        # Format code
just tidy       # Tidy go.mod
just run        # Build and run
```

**Install `just`:**
- macOS: `brew install just`
- Linux: See [just installation docs](https://github.com/casey/just#installation)

### Project Structure

```
bdry-cli/
├── cmd/bndry/          # Main entry point
│   └── main.go
├── internal/
│   ├── app/            # Command implementations
│   │   ├── app.go      # Core app logic
│   │   └── command.go  # Cobra command tree
│   ├── boundary/       # Boundary SDK wrapper
│   │   ├── client.go   # API client
│   │   └── types.go    # Type definitions
│   ├── config/         # Config management (Viper)
│   │   └── config.go
│   └── ui/             # Interactive prompts (Bubble Tea)
│       ├── ui.go       # Prompt implementations
│       └── styles.go   # Lip Gloss styling
├── go.mod
├── go.sum
├── Justfile
└── README.md
```

### Releasing

This project uses [GoReleaser](https://goreleaser.com/) for automated releases.

**Create a new release:**

```bash
# Tag the release
git tag -a v1.0.0 -m "Release v1.0.0"
git push origin v1.0.0

# GoReleaser will automatically:
# - Build binaries for macOS and Linux (amd64/arm64)
# - Create GitHub release with binaries and checksums
# - Update the Homebrew tap at ofkm/homebrew-tap
```

**Test the release process locally:**

```bash
# Install goreleaser
brew install goreleaser

# Test the build
goreleaser build --snapshot --clean

# Test the full release process (no publish)
goreleaser release --snapshot --clean
```

**Requirements for releases:**
- `GITHUB_TOKEN` environment variable with repo access
- Push access to `ofkm/homebrew-tap` repository

---

## Why `bndry` Exists

Boundary is powerful, but the CLI workflow for basic SSH tasks is... not simple:

```bash
# The official way (before bndry):
boundary authenticate oidc -auth-method-id amoidc_xxxxx
boundary connect ssh -target-id ttcp_1234567890
```

That requires:
- Memorizing or looking up target IDs
- Managing auth tokens manually
- Knowing the exact auth method ID
- Typing long IDs every time

**With `bndry`:**

```bash
bndry login
bndry add 192.168.1.100 -n webserver
bndry ssh webserver
```

Done. No IDs to memorize, no multi-step setup, just names and workflows.

---

## License

[MIT License](LICENSE)

---

## Contributing

Contributions welcome! Please open an issue or PR.

---

## Support

For issues or questions:
- Open an issue on GitHub
- Check the [Troubleshooting](#troubleshooting) section above

For Boundary itself, see the [official HashiCorp Boundary documentation](https://developer.hashicorp.com/boundary/docs).

---

## Config

`bndry` now uses a Viper-backed universal config file.

Path resolution works like this:

1. if `BNDRY_CONFIG` is set, that file is used
2. otherwise, `bndry` reuses the first existing file in `~/.config/bndry/`
3. if none exists yet, it defaults to `~/.config/bndry/config.yaml`

Create one with:

```bash
bndry config init
```

Supported/default fields:

- `boundary_addr`
- `oidc_auth_method_id` — optional when your Boundary controller already has a primary/default OIDC provider
- `auth_token`
- `default_project_scope_id`
- `default_catalog_name`
- `default_host_set_name`
- `default_role_name`
- `default_principal_id`
- `default_target_port`
- `default_create_role`
- `default_connect_after_create`
- `auto_select_default_scope`
- `last_host_catalog_id`

Example:

```yaml
boundary_addr: https://boundary.ofkm.us
oidc_auth_method_id: amoidc_Uch9ZdZ6Hd
auth_token: at_very_secret_token_here
default_project_scope_id: p_1234567890

default_catalog_name: Homelab Static Hosts
default_host_set_name: linux-servers
default_role_name: linux-ssh-users
default_principal_id: mgoidc_abcdef1234
default_target_port: 22

default_create_role: true
default_connect_after_create: true
auto_select_default_scope: false
```

Environment overrides are also supported with the `BNDRY_` prefix, for example:

- `BNDRY_BOUNDARY_ADDR`
- `BNDRY_AUTH_TOKEN`
- `BNDRY_DEFAULT_PROJECT_SCOPE_ID`
- `BNDRY_DEFAULT_TARGET_PORT`

## Why this exists

Boundary is powerful, but the CLI is not exactly... what one would call “forgettable in a good way.”

`bndry` keeps Boundary's official APIs under the hood while removing most of the ID-hunting and argument soup from the common SSH onboarding workflow.
