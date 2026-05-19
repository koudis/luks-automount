# luks-automount — Formal Requirements

## 1. Overview

`luks-automount` is a Go program that automatically unlocks and mounts
LUKS-encrypted external USB SSD drives when they are plugged in, and
unmounts and closes them when they are removed. Multiple drives are
supported. Passphrases are stored in GNOME Keyring.

---

## 2. Functional Requirements

### 2.1 Device Detection

- **FR-01** The daemon MUST detect USB block device insertion and removal
  via the kernel Netlink UEvent socket (no polling).
- **FR-02** Only events with `DEVTYPE=disk` MUST be processed; partition
  events (`DEVTYPE=partition`) MUST be ignored.
- **FR-03** On startup the daemon MUST scan `/sys/block` for already-plugged
  disks and unlock/mount any that are registered in the configuration.
- **FR-04** Each detected disk MUST be identified by its **LUKS UUID** read
  from the LUKS header. Serial numbers and device node names (`/dev/sdX`)
  MUST NOT be used as the primary identifier.

### 2.2 Unlocking and Mounting

- **FR-05** When a registered disk is detected the daemon MUST unlock the
  LUKS volume and mount the filesystem without any user interaction
  (unattended unlock).
- **FR-06** The passphrase MUST be retrieved from GNOME Keyring before
  unlock. If the keyring is locked, the standard GNOME Keyring unlock
  dialog MUST appear.
- **FR-07** If no keyring entry exists for a disk, the daemon MUST log the
  condition and skip the disk. Interactive fallback is available only in
  the `unlock` one-shot subcommand.
- **FR-08** The mount point MUST already exist before mounting. The program
  MUST NOT create mount points automatically.
- **FR-09** All mount points MUST reside under `/mnt`. A valid mount point
  satisfies `strings.HasPrefix(p, "/mnt/")` on the cleaned path; bare `/mnt`
  and paths containing `..` MUST be rejected at configuration time and by the
  worker at runtime.
- **FR-10** After mounting a non-FAT-family filesystem, the root directory of
  the mounted filesystem MUST be `chown`-ed to the UID/GID of the user who
  owns the daemon process. If `chown` fails (e.g., read-only mount), the
  failure MUST be logged as a warning and the mount operation MUST still
  succeed.
- **FR-11** For FAT/exFAT/NTFS filesystems `uid=<uid>,gid=<gid>` MUST be
  injected into the mount options instead of calling `chown`.

### 2.3 Unmounting and Locking

- **FR-12** When a registered disk is removed, the daemon MUST perform a
  **lazy unmount** (`MNT_DETACH`) followed by closing the dm-crypt mapping.
  If the disk was still tracked as mounted at removal time, the daemon MUST
  log a `WARN`-level message indicating the device was removed while mounted.
- **FR-13** On daemon shutdown (SIGINT / SIGTERM) the daemon MUST stop
  accepting new events, wait up to 10 seconds for in-flight operations to
  complete, then exit. It MUST NOT auto-unmount mounted disks.

### 2.4 Multi-Disk Support

- **FR-14** Any number of LUKS disks MAY be registered. Each is identified
  by a unique logical `name`.
- **FR-15** Two or more registered disks MAY be plugged in simultaneously;
  each MUST be handled independently and concurrently.
- **FR-16** Duplicate plug-in events for the same device MUST be idempotent
  (if the mapper already exists, skip unlock; if already mounted, skip mount).

### 2.5 CLI Subcommands

| Subcommand | Privilege | Description |
|---|---|---|
| `add <name>` | user + one sudo | Register a new disk interactively and store its passphrase in GNOME Keyring. Reads the LUKS UUID via the `read_uuid` worker op. |
| `remove <name>` | user | Delete the config entry and the Keyring entry for `<name>`. Refuses if the disk is currently mounted; user must run `lock <name>` first. |
| `list` | user | Show all registered disks. Columns: `NAME`, `UUID`, `DEV`, `UNLOCKED`, `MOUNTED`. |
| `unlock <name>` | user | One-shot: unlock and mount a specific already-plugged disk. Falls back to interactive passphrase prompt if Keyring entry is missing. |
| `lock <name>` | user | One-shot: unmount and close a specific disk. |
| `run` | user | Long-running daemon: detect and auto-unlock on plug-in, auto-lock on removal. Only one instance may run at a time (lock file). |
| `install-service` | user | Write a systemd user unit file and enable it so the daemon starts automatically at login. |
| `uninstall-service` | user | Stop and disable the systemd user service and remove the unit file. |
| `worker` | root (sudo) | **Internal, hidden from help.** Performs a single privileged operation driven by a JSON message on stdin. |

- **FR-17** `add` MUST list currently plugged LUKS disks for the user to
  choose from. UUID discovery is done via the `read_uuid` worker op (one
  sudo prompt). The UUID MUST be stored in the configuration.
- **FR-18** `add` MUST prompt for the passphrase without terminal echo and
  store it in GNOME Keyring. The passphrase MUST NOT be written to the
  config file or any log.
- **FR-19** `add` MUST validate that the provided `mount_point` satisfies
  FR-09 (path under `/mnt/`, no `..`). It MUST also fail with a clear error
  if the directory does not exist, printing the exact `sudo mkdir` command
  needed.
- **FR-20** `add` MUST reject duplicate `name`, `luks_uuid`, or
  `mapper_name` values across all existing config entries.
- **FR-21** The `name` field MUST match `[a-zA-Z0-9_-]+` (same charset as
  `mapper_name`). It is used as both the Keyring account key and the default
  `mapper_name` if not overridden.
- **FR-22** `install-service` MUST write a systemd user unit file to
  `~/.config/systemd/user/luks-automount.service` containing an
  `[Install]` section with `WantedBy=default.target` so the service starts
  at login. The `ExecStart` value MUST be the absolute path of the running
  binary followed by `run`.
- **FR-23** `install-service` MUST call `systemctl --user daemon-reload`
  and `systemctl --user enable --now luks-automount` after writing the unit
  file.
- **FR-24** `install-service` MUST fail with a clear error if the unit file
  already exists, unless a `--force` flag is passed, in which case the
  existing file is overwritten and the service is restarted.
- **FR-25** `uninstall-service` MUST call
  `systemctl --user disable --now luks-automount` and then remove the unit
  file. It MUST succeed (with a warning) if the unit file does not exist.

---

## 3. Non-Functional Requirements

- **NFR-01** The program MUST be written in Go with no CGO.
- **NFR-02** Privilege escalation MUST use plain `sudo` to re-execute the
  same binary with the hidden `worker` subcommand.
- **NFR-03** The passphrase MUST be transmitted from the user process to the
  worker exclusively via **stdin** (a pipe). It MUST NOT appear on the
  command line, in environment variables, or in any log.
- **NFR-04** Passphrase bytes MUST be zeroed in memory immediately after use.
- **NFR-05** The configuration file MUST be created with permissions `0600`.
- **NFR-06** Each registered disk MUST be processed in its own goroutine.
  A per-disk mutex keyed by **LUKS UUID** MUST prevent concurrent operations
  on the same device.
- **NFR-07** Only one instance of `run` MAY execute at a time. A lock file
  at `~/.local/state/luks-automount/run.lock` MUST be acquired with `flock`
  on startup; if already held the process MUST exit with an error.
- **NFR-08** The `run` subcommand is intended to be launched by the
  systemd user instance, which guarantees that `DBUS_SESSION_BUS_ADDRESS`
  and `XDG_RUNTIME_DIR` are set. The generated unit file MUST include
  `After=graphical-session.target` so the daemon starts only after the
  GNOME session is fully initialised.
- **NFR-09** The supported filesystem types are: `ext4`, `btrfs`, `xfs`,
  `vfat`, `exfat`, `ntfs`. Any other value MUST be rejected.
- **NFR-10** Mount options MUST always include `nosuid,nodev` prepended
  before any user-supplied options. User-supplied options MUST NOT override
  or remove these forced options.

---

## 4. Configuration

### 4.1 Location

`~/.config/luks-automount/config.toml`

### 4.2 Schema

```toml
[[disk]]
name            = "backup-ssd"       # unique logical name; also the Keyring account key
luks_uuid       = "a1b2c3d4-..."     # LUKS header UUID — primary match key
mapper_name     = "backup-ssd"       # /dev/mapper/<mapper_name>
mount_point     = "/mnt/backup"      # must be under /mnt and must already exist
filesystem_type = "ext4"
mount_options   = "noatime"          # optional, comma-separated
```

---

## 5. GNOME Keyring Integration

- Each disk has one Keyring entry: `service = "luks-automount"`,
  `account = <disk.name>`.
- `remove <name>` MUST delete the Keyring entry in addition to the config
  entry.
- The Keyring is accessed by the **user process only**. The `worker` process
  MUST NOT access D-Bus or the Keyring.

---

## 6. Privileged Worker Protocol

Communication between the user process and the `worker` subprocess is via
**stdin/stdout** using JSON.

### Request (written to worker stdin)

```jsonc
// unlock_and_mount — passphrase is sent as a second line after the JSON
{ "op": "unlock_and_mount", "dev": "/dev/sdb", "mapper": "backup-ssd",
  "mount_point": "/mnt/backup", "fs": "ext4", "options": "noatime",
  "uid": 1000, "gid": 1000 }
<passphrase line>

// unmount_and_close
{ "op": "unmount_and_close", "mapper": "backup-ssd", "mount_point": "/mnt/backup" }

// read_uuid — used by `add` to discover the LUKS UUID without requiring
// the user to be in the `disk` group
{ "op": "read_uuid", "dev": "/dev/sdb" }
```

### Response (worker writes to stdout)

```json
{ "ok": true,  "message": "" }
{ "ok": false, "message": "<reason>" }
```

For `read_uuid` the `message` field carries the UUID string on success.

- One worker process is spawned per operation and exits immediately after
  writing the response.
- Exit codes: `0` on `{ok:true}`, `1` on `{ok:false}` (operation error),
  `2` on protocol / JSON parse error.
- The worker MUST validate all inputs before acting:
  - `op` is one of `unlock_and_mount`, `unmount_and_close`, `read_uuid`.
  - `mount_point` satisfies `strings.HasPrefix(cleaned, "/mnt/")` and
    contains no `..` components.
  - `mapper` matches `[a-zA-Z0-9_-]+`.
  - `dev` matches `^/dev/[a-z]+$`.
  - `fs` is in the supported filesystem whitelist (NFR-09).
  - User-supplied `options` MUST NOT contain `suid`, `dev`, or `exec`
    tokens; these are silently dropped and a warning is logged.

---

## 7. Logging

- **LG-01** All log output MUST go to **both** stderr and a rotating log
  file at `~/.local/state/luks-automount/luks-automount.log`.
- **LG-02** Log rotation settings: max size **10 MB**, max **5** backup
  files, max age **30 days**, compressed backups.
- **LG-03** The passphrase MUST NEVER appear in any log message.
- **LG-04** Log entries MUST include a UTC timestamp, severity level, and a
  short message. Structured logging MUST be used: **text** format on stderr,
  **JSON** format in the rotating log file.

---

## 8. Component Layout

```
luks-automount/
├── cmd/luks-automount/
│   └── main.go                  # cobra root + subcommand wiring
├── internal/
│   ├── config/
│   │   ├── config.go            # Config struct (slice of Disk), Load / Save
│   │   └── disk.go              # Disk struct
│   ├── keyring/
│   │   └── keyring.go           # Get / Set / Delete passphrase by disk name
│   ├── logging/
│   │   └── logging.go           # slog setup: stderr + rotating file
│   ├── luks/
│   │   └── identify.go          # ReadUUID(devPath) → string
│   ├── monitor/
│   │   └── monitor.go           # Netlink listener → chan Event
│   ├── worker/
│   │   ├── protocol.go          # Request / Response JSON types
│   │   ├── client.go            # user-side: spawn sudo worker, pipe passphrase
│   │   └── server.go            # root-side: read stdin, run op, write response
│   └── engine/
│       └── engine.go            # orchestrates monitor + config + keyring + worker
└── doc/
    └── requirements.md
```

---

## 9. Failure Handling

| Condition | Behaviour |
|---|---|
| Keyring entry missing | Daemon logs `WARN` and skips; `unlock` prompts interactively |
| Wrong passphrase | Log `ERROR`; do NOT auto-delete the Keyring entry |
| Mount point does not exist | Log `ERROR` and abort; `add` prints `sudo mkdir` hint |
| Mount point outside `/mnt` | Rejected at `add` time and by the worker at runtime |
| `remove` while disk is mounted | Exit with error; user must run `lock <name>` first |
| Disk already unlocked / mounted | Log `INFO` and skip that step; remaining steps proceed |
| `chown` fails on read-only mount | Log `WARN`; mount operation still reports success |
| Device removed while mounted | Log `WARN`; proceed with lazy unmount + LUKS close |
| Duplicate entry on `add` | Exit with error listing which field is duplicated |
| Second `run` instance started | Exit with error: "daemon is already running" |
| `install-service` when unit exists | Exit with error unless `--force` is passed |
| `uninstall-service` when unit missing | Log `WARN` and exit 0 |
| sudo denied / no TTY | Log `ERROR`; daemon stays alive |
| Netlink socket error | Reconnect with exponential back-off |
