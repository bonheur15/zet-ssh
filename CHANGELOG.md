# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### 🚀 Initialized
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
