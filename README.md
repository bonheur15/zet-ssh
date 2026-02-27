# Zet-SSH

A modern keyboard-driven SSH workspace in your terminal: profiles, secure auth, multi-session tabs, file transfer, and release-driven updates.

## Features

- Profile manager with create/edit and `~/.ssh/config` import
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
curl -fsSL https://raw.githubusercontent.com/bonheur/zet-ssh-4/main/scripts/install.sh | sh
```

Install a specific version tag:

```bash
curl -fsSL https://raw.githubusercontent.com/bonheur/zet-ssh-4/main/scripts/install.sh | sh -s -- v0.1.0
```

## Build From Source

```bash
go build -o zet ./cmd/zet
```

## Usage

```bash
zet                  # open TUI
zet help             # show CLI help
zet version          # show build version
zet update           # update to latest release
zet update --check   # only check for updates
```

### TUI Keybinds

- Dashboard: `[n]` new profile, `[e]` edit profile, `[i]` import `~/.ssh/config`, `[Enter]` connect
- Global: `Ctrl+K` command palette
- Session: `[Alt+1..9]` jump tabs, `[` / `]` switch tabs, `Ctrl+W` close tab
- File mode: `Ctrl+F` toggle, `Tab` switch pane, `c` copy, `o` preview, `x` cancel transfer

## Security

- SSH host keys are validated using:
  - `~/.config/zet-ssh/known_hosts`
  - `~/.ssh/known_hosts`
- If neither exists, connections fail closed.

## Updater / Auto Update

`zet update` checks GitHub Releases and replaces the local binary with the matching OS/arch asset.

Environment variables:

- `ZET_SSH_REPOSITORY` (default: `bonheur/zet-ssh-4`)
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
