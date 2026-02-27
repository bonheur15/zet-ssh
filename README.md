# Zet-SSH

A modern keyboard-driven SSH workspace in your terminal: profiles, secure auth, multi-session tabs, file transfer, and release-driven updates.

## Features

- Profile manager with create/edit and `~/.ssh/config` import
- Paste-import profile from SSH command (`ssh user@host -p 22`)
- Per-profile SSH agent forwarding toggle
- Secure SSH host key verification via `known_hosts`
- Multi-session SSH tabs with quick switching
- Dual-pane file browser (local + remote)
- File and directory upload/download with progress + cancel
- Command palette and vault unlock modal
- Theme overrides via `~/.config/zet-ssh/theme.json`
- Built-in updater from GitHub releases (`zet update`)

## Installation

Quick install (auto-detect OS/arch and fetch latest GitHub release):

```bash
curl -fsSL https://raw.githubusercontent.com/bonheur15/zet-ssh/main/scripts/install.sh | sh
```

Install a specific version tag:

```bash
curl -fsSL https://raw.githubusercontent.com/bonheur15/zet-ssh/main/scripts/install.sh | sh -s -- v0.1.0
```

## Build From Source

```bash
go build -o zet ./cmd/zet
```

## Usage

```bash
zet                  # open TUI
zet help             # show CLI help
zet connect prod     # open TUI and jump directly into profile session
zet run prod -- uname -a  # execute remote command and exit
zet version          # show build version
zet update           # update to latest release
zet update --check   # only check for updates
```

### TUI Keybinds

- Dashboard: `[n]` new profile, `[e]` edit profile, `[i]` import `~/.ssh/config`, `[Enter]` connect
- Global: `Ctrl+K` command palette
- Session: `[Alt+1..9]` jump tabs, `[` / `]` switch tabs, `Ctrl+W` close tab
- Tunnels: `Ctrl+T` tunnel manager, `a` add, `space` start/stop, `d` delete
- File mode: `Ctrl+F` toggle, `Tab` switch pane, `c` copy, `o` preview, `x` cancel transfer

## Under The Hood

### Tech Stack

- Language/runtime: Go
- TUI framework: Bubble Tea + Bubbles
- Styling: Lip Gloss
- SSH: `golang.org/x/crypto/ssh`
- File transfer: `github.com/pkg/sftp`
- Vault crypto: Argon2id + ChaCha20-Poly1305

### Runtime Architecture

- `cmd/zet/main.go` is the entrypoint for TUI mode and CLI commands (`help`, `version`, `update`).
- `internal/tui/root.go` is the app state machine:
  - Dashboard state for profile management
  - Session state for active SSH tabs
  - Global overlays (palette, vault unlock modal)
- `internal/tui/pages/session/session.go` owns one SSH workspace tab:
  - Terminal viewport rendering
  - SSH stream read/write loop
  - Dual-pane file mode
  - Transfer progress and cancellation channel

### Connection Flow

1. User selects a profile on dashboard.
2. Session builder assembles auth methods from profile + environment.
3. SSH connects with strict host key validation (`known_hosts` callback).
4. PTY shell starts with resize support.
5. SFTP client is attached to the same SSH client for file operations.

### Auth Method Resolution

- `auth_type=agent`: profile/default keys + agent, optional password fallback.
- `auth_type=key`: profile key path + agent, optional password fallback.
- `auth_type=password`: password/keyboard-interactive only.
- Runtime password prompt appears on auth failure and retries the connection.

### File Browser and Transfer Engine

- Local and remote panes share a unified browser model (`FileBrowser`).
- Copy action (`c`) runs direction-aware transfer:
  - Local -> Remote: upload
  - Remote -> Local: download
- Directories transfer recursively with aggregate byte progress.
- Cancellation is cooperative via a cancel channel checked inside copy loops.

### Data and Config Layout

Stored under `~/.config/zet-ssh/`:

- `profiles.json`: saved SSH profile definitions
- `theme.json`: optional color/theme overrides
- `vault.zet`: encrypted vault payload
- `known_hosts`: optional app-local host key trust file

### Update System

- `zet update` calls GitHub Releases API for latest version.
- It selects the matching `GOOS/GOARCH` tarball.
- Downloads and extracts `zet` binary.
- Replaces current executable atomically via temp file + rename.
- `ZET_SSH_AUTO_UPDATE=1` runs the same update check before launching TUI.

### Release Pipeline

- CI workflow: runs `go test ./...` and build checks on pushes/PRs.
- Release workflow (tag `v*`):
  - Cross-compiles Linux/macOS/Windows (`amd64`, `arm64`)
  - Packs `zet-<os>-<arch>.tar.gz`
  - Publishes assets to GitHub Release
- Installer script and updater both consume these release assets.

## Security

- SSH host keys are validated using:
  - `~/.config/zet-ssh/known_hosts`
  - `~/.ssh/known_hosts`
- If neither exists, connections fail closed.

## Updater / Auto Update

`zet update` checks GitHub Releases and replaces the local binary with the matching OS/arch asset.

Environment variables:

- `ZET_SSH_REPOSITORY` (default: `bonheur15/zet-ssh`)
- `ZET_SSH_AUTO_UPDATE=1` to run updater before launching TUI

Example:

```bash
export ZET_SSH_AUTO_UPDATE=1
zet
```

## Release Artifacts

Tag pushes like `v1.2.3` trigger GitHub Actions to build and publish:

- Linux: `amd64`, `arm64`
- macOS: `amd64`, `arm64`
- Windows: `amd64`, `arm64`

Assets are published as `zet-<os>-<arch>.tar.gz`.
