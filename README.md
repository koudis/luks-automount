# luks-automount

Automatically unlocks and mounts whole-disk LUKS-encrypted USB SSDs when plugged in, and unmounts and locks them on removal. Passphrases are stored in GNOME Keyring. Privileged operations run through a short-lived worker subprocess invoked via `sudo`.

## Requirements

- Linux with device-mapper, `/dev/mapper`, block-device UEvent support, and mount support
- A graphical user session with GNOME Keyring or another compatible Secret Service implementation
- `sudo` installed and configured to allow the user to run `luks-automount worker` as root
- Mount points created under `/mnt/` before registering a disk

### Runtime requirements

- The hidden `worker` subcommand must be executable through `sudo`
- The user session must provide access to the Secret Service API so stored passphrases can be read
- `install` and `uninstall` require a user `systemd` instance
- The program validates mount points under `/mnt/`; bare `/mnt` and paths containing `..` are rejected

### Build requirements

- Go 1.26 or newer
- A Linux system matching the runtime requirements above

### External tools and services

- Required for normal operation:
  - `sudo`
  - `cryptsetup`
  - `systemctl` for `install` and `uninstall`
  - GNOME Keyring or another Secret Service provider
- Required for the integration smoke test:
  - `dd`
  - `losetup`
  - `mkfs.ext4`
  - root privileges
  - a host or VM with working loop devices

## Build

```sh
go build -o luks-automount ./cmd/luks-automount
```

Install the tool, user service, and sudoers rule:

```sh
./luks-automount install
```

The installer asks for confirmation before each step. Root access is requested through `sudo` only for installing `/usr/local/bin/luks-automount` and `/etc/sudoers.d/luks-automount`.

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

### Install

The user service cannot answer `sudo` password prompts. Run the installer from the built binary before enabling normal daemon use.

```sh
luks-automount install
```

The command can install the binary to `/usr/local/bin/luks-automount`, write the sudoers drop-in, write `~/.config/systemd/user/luks-automount.service`, and enable it immediately.

```sh
luks-automount uninstall
```

The command can disable and remove `~/.config/systemd/user/luks-automount.service`, remove `/etc/sudoers.d/luks-automount`, and remove `/usr/local/bin/luks-automount`.

### Remove a disk registration

```sh
luks-automount lock myusb          # unmount first if mounted
luks-automount remove myusb
```

## sudo configuration

The installer writes this rule so the user service can run the worker without a password prompt:

```
# /etc/sudoers.d/luks-automount
youruser ALL=(root) NOPASSWD: /usr/local/bin/luks-automount worker
```

If you manage sudoers manually, validate changes with `visudo` and keep the path equal to `/usr/local/bin/luks-automount`.

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

## License

This project is licensed under the Apache License 2.0. See `LICENSE`.
