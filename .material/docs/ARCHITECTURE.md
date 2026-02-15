# Zet-SSH Architecture (Qt 6 / Linux)

## Stack
- Qt 6 + QML (Qt Quick Controls 2)
- C++17 backend services exposed to QML
- External system tools for SSH/SFTP/tunnels (`ssh`, `sftp`, `scp`, `ssh-keygen`)
- Encrypted vault using OpenSSL AES-256-GCM + PBKDF2-HMAC-SHA256

## Core components
- `Vault` (encrypted secrets store, auto-lock)
- `ProfilesModel` (saved connections, tags, validation)
- `SnippetsModel` (command library, variables)
- `WorkspaceStore` (tabs, tunnels, file roots, layouts)
- `CommandLogModel` (transparent command log + redaction)
- `SshSessionManager` (spawned SSH sessions)
- `SftpClient` (file ops via `sftp`/`scp`)
- `TunnelManager` (local/remote/SOCKS forwarding)
- `KnownHostsManager` (view/remove entries)

## Data files
Stored under `~/.config/zet-ssh/`:
- `profiles.json`
- `snippets.json`
- `workspaces.json`
- `commandlog.jsonl`
- `vault.zet` (encrypted)

## UI layout
- Top bar: workspace + search + quick connect
- Left sidebar: tabs, quick actions, palette
- Center: terminal sessions
- Right panel: profiles / file browser
- Bottom panel: command log

## Security
- Strict host key checking by default
- Encrypted vault required for secrets
- Redaction enabled in command logs
- Dangerous command confirmation rules
