# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Initialized
- Set up project structure for `zet-ssh` (Go + Bubble Tea).
- Created `ARCHITECTURE.md` and Design Docs.
- Established `internal/` package structure for TUI and Core logic.

### ✨ Added
- **Profile Core:** Implemented JSON-based profile storage and management.
- **Vault Core:** Implemented secure encryption/decryption using Argon2id and ChaCha20-Poly1305.
- **SSH Core:** Implemented SSH client wrapper with PTY support.
- **SFTP Core:** Implemented SFTP client wrapper for file operations.
- **TUI Dashboard:** Improved list view with real profiles and "Add Profile" functionality.
- **Command Palette:** Added `Ctrl+K` palette for quick actions.
- **Vault UI:** Added secure master password prompt for vault unlocking.
- **Terminal Bridge:** Implemented live I/O between Bubble Tea and SSH sessions.
- **Advanced Auth:** Added support for Password and Public Key authentication in SSH core.
- **File Browser:** Created a new TUI component for directory navigation.
- **Tunnel Builder:** Added a dedicated UI page for managing SSH tunnels.
- **Session File Mode:** Added dual-pane local/remote file browser mode in the SSH session view (`Ctrl+F`).
- **Pane Transfers:** Added `c` copy action for file upload/download between local and remote panes.
- **Theme Overrides:** Added support for custom Lip Gloss color overrides via `~/.config/zet-ssh/theme.json`.
- **Transfer Progress:** Added live transfer progress bars and byte counters for upload/download.
- **Transfer Cancel:** Added cancel support during transfers (`x` in file mode).
- **File Preview:** Added `o` action to open and preview local or remote files in pane mode.
- **Session Tabs:** Added support for multiple simultaneous SSH sessions with tab switching.
- **Dashboard Editing:** Added profile editing flow (`e`) and improved form validation/save behavior.

### Changed
- **File Browser State:** Extended browser component with parent navigation, active pane highlighting, and stable path handling.
- **SFTP Core:** Upload/download now create missing parent directories where needed.
- **Profile Store:** Added upsert/update behavior for editing persisted profiles.

### Fixed
- **SSH Auth Failure:** Session connect now builds real auth methods (agent, key files, optional env password) instead of trying with an empty auth list.
- **Session Lifecycle:** Returning from session now closes SSH/SFTP resources.
- **Terminal Clear Handling:** Improved handling of clear-screen escape sequences to reduce viewport corruption after `clear`.
