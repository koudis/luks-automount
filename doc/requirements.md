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

- **FR-12** Before any code path performs any `umount` / unmount operation,
  including **lazy unmount** (`MNT_DETACH`), the worker MUST check whether any
  process is still using the mount point. This applies to manual `lock`, daemon
  auto-lock on removal, and any cleanup unmount before re-mounting. The check
  MUST be performed immediately before the unmount attempt; earlier state checks
  MUST NOT be treated as sufficient. If one or more processes are using the
  mount point, the operation MUST stop before unmounting and before closing the
  dm-crypt mapping. The worker MUST return a structured busy response containing
  the blocking processes. The user-side process MUST show a GNOME graphical
  warning listing those processes and asking the user to close them and retry.
  If no blocking processes are found and a registered disk was removed, the
  daemon MUST perform a **lazy unmount** (`MNT_DETACH`) followed by closing the
  dm-crypt mapping. If the disk was still tracked as mounted at removal time,
  the daemon MUST log a `WARN`-level message indicating the device was removed
  while mounted.
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
| `lock <name>` | user | One-shot: unmount and close a specific disk. If the mount point is busy, show a GNOME warning listing the blocking processes. |
| `run` | user | Long-running daemon: detect and auto-unlock on plug-in, auto-lock on removal. It is intended to run as a systemd user service. If the mount point is busy during auto-lock, show a GNOME warning from the user service. Only one instance may run at a time (lock file). |
| `install` | user + sudo | Install the binary to `/usr/local/bin`, write the sudoers drop-in, and write and enable the systemd user unit. |
| `uninstall` | user + sudo | Disable and remove the systemd user unit, remove the sudoers drop-in, and remove the installed binary. |
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
- **FR-22** `install` MUST ask for confirmation before installing the current
  binary to `/usr/local/bin/luks-automount` with root privileges.
- **FR-23** `install` MUST ask for confirmation before writing a systemd user
  unit file to `~/.config/systemd/user/luks-automount.service`. The unit MUST
  contain an `[Install]` section with `WantedBy=default.target` and
  `ExecStart=/usr/local/bin/luks-automount run`. After writing the unit, it
  MUST call `systemctl --user daemon-reload` and
  `systemctl --user enable --now luks-automount`.
- **FR-24** `install` MUST ask for confirmation before writing
  `/etc/sudoers.d/luks-automount` with root privileges. The rule MUST allow the
  current user to run `/usr/local/bin/luks-automount worker` as root without a
  password prompt. The sudoers content MUST be validated with `visudo` before
  installation.
- **FR-25** `uninstall` MUST ask for confirmation before disabling and
  removing `~/.config/systemd/user/luks-automount.service`. It MUST call
  `systemctl --user disable --now luks-automount`, remove the unit file, and
  then call `systemctl --user daemon-reload`. If the unit file does not
  exist, it MUST log `WARN` and continue.
- **FR-26** `uninstall` MUST ask for confirmation before removing
  `/etc/sudoers.d/luks-automount` with root privileges.
- **FR-27** `uninstall` MUST ask for confirmation before removing
  `/usr/local/bin/luks-automount` with root privileges.

---

## 3. Non-Functional Requirements

- **NFR-01** The program MUST be written in Go with no CGO.
- **NFR-02** Privilege escalation MUST use `sudo` to re-execute the same
  binary with the hidden `worker` subcommand. If the caller has no terminal,
  the worker invocation MUST pass `-n` so `sudo` fails instead of prompting.
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
  systemd user instance. The generated unit file MUST include
  `After=graphical-session.target` and `PartOf=graphical-session.target` so
  the daemon runs in the user's graphical session. User-facing dialogs MUST be
  launched only by user-side processes, including the `lock` command and the
  `run` user service. The privileged `worker` process MUST NOT access D-Bus,
  the display server, or GUI tools.
- **NFR-09** The supported filesystem types are: `ext4`, `btrfs`, `xfs`,
  `vfat`, `exfat`, `ntfs`. Any other value MUST be rejected.
- **NFR-10** Mount options MUST always include `nosuid,nodev` prepended
  before any user-supplied options. User-supplied options MUST NOT override
  or remove these forced options.
- **NFR-11** `cryptsetup` MUST be available to the privileged worker for
  LUKS identification, unlocking, and closing.
- **NFR-12** A GNOME-compatible graphical dialog utility, such as `zenity`,
  SHOULD be used for mount-point-busy warnings when a graphical session is
  available. If no graphical session or dialog utility is available, the
  operation MUST still fail safely and the same blocking-process details MUST
  be returned to the caller or written to the daemon log.

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
- GNOME warning dialogs are also shown by the **user process only**. The
  `worker` process MUST report structured data and MUST NOT open windows.

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
{ "ok": false, "code": "mount_point_busy", "message": "<reason>",
	  "mount_users": [
	    { "pid": 1234, "name": "nautilus", "cmdline": "nautilus" }
	  ] }
```

For `read_uuid` the `message` field carries the UUID string on success. The
`code` field is optional and carries a machine-readable error code when the
caller must handle an error specially. `mount_users` is present only for
`code = "mount_point_busy"`.

- One worker process is spawned per operation and exits immediately after
  writing the response.
- Exit codes: `0` on `{ok:true}`, `1` on `{ok:false}` (operation error),
  `2` on protocol / JSON parse error.
- For `unmount_and_close`, the worker MUST inspect `/proc` immediately before
  every unmount attempt when the requested mount point is mounted from the
  expected mapper. A process counts as a mount user when its `cwd`, `root`,
  `exe`, an open file descriptor, or a memory-mapped file resolves to the mount
  point or below it. The worker MUST include at least PID and process name for
  each discovered process, and SHOULD include the command line when available.
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
│   ├── dialog/
│   │   └── dialog.go            # user-side GNOME warnings
│   ├── luks/
│   │   └── identify.go          # ReadUUID(devPath) → string
│   ├── monitor/
│   │   └── monitor.go           # Netlink listener → chan Event
│   ├── worker/
│   │   ├── request.go           # Request JSON type
│   │   ├── response.go          # Response JSON type
│   │   ├── client.go            # user-side: spawn sudo worker, pipe passphrase
│   │   ├── mount_users.go       # root-side: inspect mount-point users via /proc
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
| Mount point busy during `lock` | Abort before unmount/close; show GNOME warning listing blocking processes |
| Mount point busy during daemon auto-lock | Abort before unmount/close; the `run` user service shows GNOME warning listing blocking processes and logs the failure |
| `remove` while disk is mounted | Exit with error; user must run `lock <name>` first |
| Disk already unlocked / mounted | Skip the already-done step; remaining steps proceed |
| Disk already closed / unmounted | Skip the already-done step; remaining steps proceed |
| `chown` fails on read-only mount | Log `WARN`; mount operation still reports success |
| Device removed while mounted | Log `WARN`; perform the mandatory pre-unmount busy check; if not busy, proceed with lazy unmount + LUKS close; if busy, follow mount-point-busy behaviour |
| Duplicate entry on `add` | Exit with error listing which field is duplicated |
| Second `run` instance started | Exit with error: "daemon is already running" |
| `install` step declined | Skip the declined step and continue with the next step |
| `uninstall` step declined | Skip the declined step and continue with the next step |
| `uninstall` when unit file is missing | Log `WARN` and continue |
| sudo denied / no TTY | Log `ERROR`; daemon stays alive |
| Netlink socket error | Reconnect with exponential back-off |
