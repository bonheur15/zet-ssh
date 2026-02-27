# TODO

## High Priority
- [ ] Research best Go TUI terminal emulator widget (consider `charmbracelet/x/term` or `creack/pty` integration).
- [ ] Implement robust error handling for the Vault (ensure memory zeroing where possible).
- [x] Add recursive directory transfers (upload/download folders in SFTP pane mode).
- [x] Add host-key verification with `known_hosts` instead of insecure callback.
- [x] Add explicit auth fields in profile form (auth type, key path, vault refs, password source).

## Future / Nice to Have
- [x] Add support for importing `~/.ssh/config`.
- [ ] Implement "Read Only" mode for specific connections.
- [x] Add support for SSH Agent forwarding.
- [ ] Add an in-app theme editor to write `theme.json` without manual file edits.
- [ ] Replace basic ANSI stripping with full VT terminal emulation to perfectly handle commands like `clear`.
- [ ] Add checksums/signatures for release artifacts and updater verification.
- [ ] Implement SSH string paste import (`ssh user@host -p 22`) directly in dashboard.
- [ ] Replace tunnel-builder placeholder with real local/remote/dynamic forwarding runtime + health indicators.
- [ ] Implement command palette actions (execute selected actions, not just list items).
- [ ] Add transparent command log pane with collapsible UI and JSONL persistence.
- [ ] Theme changer
