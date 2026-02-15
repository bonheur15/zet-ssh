# Zet-SSH Architecture (Go / TUI)

## Stack
- **Language:** Go (Golang)
- **UI Framework:** Bubble Tea (Elm architecture for TUI)
- **Styling:** Lip Gloss (CSS-like styling for terminal)
- **SSH/SFTP Backend:** Pure Go (`golang.org/x/crypto/ssh`, `github.com/pkg/sftp`)
- **Crypto:** Go Standard Library + `golang.org/x/crypto` (Argon2id, ChaCha20-Poly1305)

## Core Components
- **Bubble Tea Model:** Central state machine handling UI updates and events.
- **Vault Service:** Encrypted secrets store with auto-lock functionality.
- **Profiles Store:** JSON-based persistence for connection details.
- **Session Manager:** Manages active SSH clients, PTY sizing, and signal propagation.
- **TUI/Terminal Emulator:** Embedded terminal widget (e.g., `viewport` or `term`) to render remote shells within the app.
- **SFTP Client:** Internal client for the TUI file manager pane.
- **Tunnel Manager:** Pure Go implementation of local/remote/dynamic forwarding.
- **Command Log:** In-memory buffer of executed operations, flushed to disk/UI.

## Data Files
Stored under `~/.config/zet-ssh/` (XDG Base Directory compliant):
- `profiles.json` (Connection definitions, tags)
- `snippets.json` (Command library)
- `workspaces.json` (Layouts, active tabs state)
- `commandlog.jsonl` (Audit log)
- `vault.zet` (Encrypted binary file)

## UI Layout (TUI)
- **Header:** Workspace name, active profile status, breadcrumbs.
- **Sidebar (Collapsible):**
  - **Tabs:** Active sessions list.
  - **Profiles:** Tree view of saved connections.
  - **File Browser:** Local/Remote file tree toggle.
- **Main View (Panes):**
  - **Terminal Pane:** The active SSH session interaction area.
  - **Split View:** Support for side-by-side terminals or File Manager.
- **Footer:** Status bar (connection health, latency), Command Input (for internal Zet-SSH commands), Key hints.

## Security
- **Memory Safety:** Go's managed memory model.
- **Encryption:** Vault uses XChaCha20-Poly1305 / AES-256-GCM. key derivation via Argon2id.
- **Zeroization:** Sensitive memory (passwords/keys) cleared from memory when vault locks (where possible via Go).
- **Redaction:** UI logs automatically mask detected secrets/keys.
- **Host Keys:** Strict checking enforced via `known_hosts`.
