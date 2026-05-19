# luks-automount

Automatically unlocks and mounts whole-disk LUKS-encrypted USB SSDs when plugged in, and unmounts and locks them on removal. Passphrases are stored in GNOME Keyring. Privileged operations run through a short-lived worker subprocess invoked via `sudo`.

## Requirements

- Linux with kernel UEvent support
- `sudo` configured to allow the user to run `luks-automount worker` as root
- GNOME Keyring (or a compatible Secret Service implementation)
- `cryptsetup` installed
- Mount points created under `/mnt/` before registering a disk

## Build

```sh
go build -o luks-automount ./cmd/luks-automount
```

Install to a directory on your PATH, e.g.:

```sh
sudo cp luks-automount /usr/local/bin/
```

## Usage

### Register a disk

Plug in the USB drive, then:

```sh
luks-automount add myusb
```

Prompts for device selection, mount point (must exist under `/mnt/`), filesystem type, mount options, and passphrase. The passphrase is stored in GNOME Keyring and never written to disk or logs.

### List registered disks

```sh
luks-automount list
```

### Manually unlock / lock

```sh
luks-automount unlock myusb
luks-automount lock myusb
```

### Run the daemon

```sh
luks-automount run
```

Watches for plug/unplug events and unlocks or locks registered disks automatically. Only one instance may run at a time.

### Install as a user systemd service

```sh
luks-automount install-service
```

Writes `~/.config/systemd/user/luks-automount.service` and enables it immediately. Use `--force` to overwrite an existing unit.

```sh
luks-automount uninstall-service
```

### Remove a disk registration

```sh
luks-automount lock myusb          # unmount first if mounted
luks-automount remove myusb
```

## sudo configuration

Add a rule so the user can run the worker without a password prompt during daemon operation:

```
# /etc/sudoers.d/luks-automount
youruser ALL=(root) NOPASSWD: /usr/local/bin/luks-automount worker
```

## Maintenance

### Changing requirements

Edit `doc/requirements.md` first. All functional decisions are recorded there. Review for contradictions before touching any code.

### Changing code

The main packages and their responsibilities:

| Package | Responsibility |
|---|---|
| `internal/config` | TOML config, disk struct, validation |
| `internal/keyring` | GNOME Keyring passphrase storage |
| `internal/logging` | slog fan-out: stderr (text) + rotating JSON file |
| `internal/monitor` | Netlink UEvent listener |
| `internal/luks` | LUKS UUID read, unlock, lock |
| `internal/worker` | Privileged worker protocol (server + client) |
| `internal/engine` | Daemon orchestrator, per-disk state machine |
| `cmd/luks-automount` | CLI (cobra subcommands) |

### Running tests

```sh
go test ./...
```

The integration smoke test requires root and a kernel with loop-device support (bare metal or a VM, not a container):

```sh
go test -tags integration -c -o /tmp/worker.test ./internal/worker/
sudo /tmp/worker.test -test.run TestSmoke_WorkerProtocol -test.v
```
