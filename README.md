# Supervisor

[![Go Report Card](https://goreportcard.com/badge/github.com/glasslabs/supervisor)](https://goreportcard.com/report/github.com/glasslabs/supervisor)
[![Build Status](https://github.com/glasslabs/supervisor/actions/workflows/agent.yml/badge.svg)](https://github.com/glasslabs/supervisor/actions)
[![GitHub license](https://img.shields.io/badge/license-MIT-blue.svg)](https://raw.githubusercontent.com/glasslabs/supervisor/main/LICENCE)

Management agent for [GlassOS](https://github.com/glasslabs/os). It supervises the `glass`
process under [cage](https://github.com/cage-kiosk/cage) and exposes an HTTP API for OTA
updates, log access, configuration management, and asset uploads.

## Table of Contents

- [Installation](#installation)
  - [Building from Source](#building-from-source)
- [Usage](#usage)
  - [Options](#options)
- [HTTP API](#http-api)
  - [Glass Process](#glass-process)
    - [GET /glass/status](#get-glassstatus)
    - [GET /glass/logs](#get-glasslogs)
    - [POST /glass/restart](#post-glassrestart)
    - [POST /glass/update](#post-glassupdate)
    - [GET /glass/config](#get-glassconfig)
    - [POST /glass/config](#post-glassconfig)
    - [POST /glass/secrets](#post-glasssecrets)
    - [GET /glass/assets](#get-glassassets)
    - [GET /glass/assets/{name}](#get-glassassetsname)
    - [POST /glass/assets/{name}](#post-glassassetsname)
    - [DELETE /glass/assets/{name}](#delete-glassassetsname)
  - [Operating System](#operating-system)
    - [POST /os/update](#post-osupdate)
    - [GET /os/status](#get-osstatus)
    - [POST /os/reboot](#post-osreboot)
- [Common Operations](#common-operations)

## Installation

`supervisor` is built into [GlassOS](https://github.com/glasslabs/os) images and runs
automatically as a systemd service. Refer to the GlassOS repository for pre-built images.

### Building from Source

Go 1.26 or later is required.

```shell
go build -o supervisor ./cmd/supervisor
```

## Usage

```shell
supervisor [options]
```

### Options

**`--addr` ADDR** *(default: `:80`)*

HTTP listen address for the management API and web UI.

**`--glass-bin` PATH** *(default: `/usr/lib/glass/glass`)*

Path to the `glass` binary that `supervisor` supervises. OTA updates replace this file.

**`--data-dir` PATH** *(default: `/data`)*

Root of the persistent data directory. The agent reads and writes the following
sub-directories:

| Path | Purpose |
|---|---|
| `<data-dir>/config/config.yaml` | Looking Glass configuration file |
| `<data-dir>/config/secrets.yaml` | Looking Glass secrets file |
| `<data-dir>/assets/` | Static assets served to modules |
| `<data-dir>/modules/` | Downloaded WASM module cache |

**`--log.level` LEVEL** *(default: `info`)*

Minimum log level. Supported values: `trace`, `debug`, `info`, `warn`, `error`.

## HTTP API

All endpoints are served on the address given by `--addr` (default port 80).
Responses use `application/json` unless noted otherwise.

### Glass Process

#### GET /glass/status

Returns a JSON snapshot of the supervised `glass` process.

**Response** `200 OK`

```json
{
  "pid": 1234,
  "restarts": 2,
  "uptime": "5m32s"
}
```

| Field | Type | Description |
|---|---|---|
| `pid` | integer | OS process ID of the running `glass` process. `0` if not yet started. |
| `restarts` | integer | Number of times the process has been restarted since agent startup. |
| `uptime` | string | Duration since the process last started. Empty string if the process is not running. |

---

#### GET /glass/logs

Returns the last 2000 lines of `glass` stdout/stderr as `text/plain`.

**Query parameters**

| Parameter | Description |
|---|---|
| `follow=true` | Stream new log lines in real time until the client disconnects. |

**Response** `200 OK` — `text/plain; charset=utf-8`

```
2024/01/15 10:30:00 Starting glass...
2024/01/15 10:30:01 Module loaded: simple-clock
```

---

#### POST /glass/restart

Sends SIGTERM to the running `glass` process. The supervision loop restarts it automatically.

**Response** `204 No Content`

---

#### POST /glass/update

Downloads a new `glass` binary, verifies its SHA-256 checksum, replaces `--glass-bin`, and
restarts the process. The download is aborted if the checksum does not match.

Accepts `application/gzip`, `application/zip`, or an uncompressed binary. For zip archives
the first file in the archive is used.

**Request body** `application/json`

```json
{
  "url": "https://example.com/glass-linux-arm64.zip",
  "sha256": "e3b0c44298fc1c149afb..."
}
```

| Field | Required | Description |
|---|---|---|
| `url` | ✓ | URL of the binary or compressed archive to download. |
| `sha256` | ✓ | Expected lowercase hex-encoded SHA-256 of the downloaded binary. |

**Response** `204 No Content` on success.

---

#### GET /glass/config

Returns the current `config.yaml` as `text/plain`.

**Response** `200 OK` — `text/plain; charset=utf-8`  
**Response** `404 Not Found` — if no config has been uploaded yet.

---

#### POST /glass/config

Replaces `config.yaml` with the request body. The file is written atomically.
Restart Glass to apply the new configuration.

**Request body** Raw YAML.

**Response** `204 No Content`

---

#### POST /glass/secrets

Replaces `secrets.yaml` with the request body. The file is written atomically.
Restart Glass to apply the new secrets.

**Request body** Raw YAML.

**Response** `204 No Content`

---

#### GET /glass/assets

Returns a JSON array of filenames currently stored in the assets directory.

**Response** `200 OK`

```json
["background.jpg", "logo.svg"]
```

---

#### GET /glass/assets/{name}

Downloads the named asset file. Supports HTTP range requests and conditional GET via
`Last-Modified`.

**Response** `200 OK` — file contents with appropriate `Content-Type`.  
**Response** `404 Not Found` — if the asset does not exist.

---

#### POST /glass/assets/{name}

Uploads a file to the assets directory, replacing any existing file with the same name.
The file is written atomically.

**Request body** Raw file bytes.

**Response** `204 No Content`

---

#### DELETE /glass/assets/{name}

Deletes the named asset file. Returns `204 No Content` even if the file does not exist.

**Response** `204 No Content`

---

### Operating System

These endpoints require [RAUC](https://rauc.io) and
[systemd-logind](https://www.freedesktop.org/wiki/Software/systemd/) to be reachable on the
system D-Bus. They are only meaningful when running on GlassOS.

#### POST /os/update

Downloads a RAUC bundle from the given URL and installs it. The request blocks until
installation completes. Reboot the device to activate the new OS image.

**Request body** `application/json`

```json
{
  "url": "https://example.com/glassos-v1.2.3-rpi4.raucb"
}
```

| Field | Required | Description |
|---|---|---|
| `url` | ✓ | URL of the `.raucb` bundle to download and install. |

**Response** `204 No Content` on success.

---

#### GET /os/status

Returns the RAUC slot status for the running system.

**Response** `200 OK`

```json
{
  "compatible": "glassos-rpi4",
  "variant": "",
  "booted": "system0",
  "slots": [
    {
      "name": "system0",
      "class": "system",
      "device": "/dev/mmcblk0p4",
      "type": "erofs",
      "bootname": "system0",
      "state": "booted",
      "sha256": "abc123...",
      "size": 268435456
    },
    {
      "name": "system1",
      "class": "system",
      "device": "/dev/mmcblk0p5",
      "type": "erofs",
      "bootname": "system1",
      "state": "inactive"
    }
  ]
}
```

| Field | Description |
|---|---|
| `compatible` | RAUC system compatibility string. |
| `variant` | RAUC system variant string. Empty if not set. |
| `booted` | Name of the currently active boot slot. |
| `slots` | Array of all configured RAUC slots and their states. |

---

#### POST /os/reboot

Triggers a graceful system reboot via systemd-logind. The `204 No Content` response is sent
before the reboot begins.

**Response** `204 No Content`

---

## Common Operations

### Check Glass process status

```shell
curl http://glass.local/glass/status
```

### View logs

```shell
curl http://glass.local/glass/logs

# Stream live
curl http://glass.local/glass/logs?follow=true
```

### Restart Glass

```shell
curl -X POST http://glass.local/glass/restart
```

### Update Glass binary (OTA)

```shell
curl -X POST http://glass.local/glass/update \
  -H 'Content-Type: application/json' \
  -d '{"url":"https://github.com/glasslabs/looking-glass/releases/download/v1.2.3/glass-v1.2.3-linux-arm64-wayland.zip","sha256":"<hex>"}'
```

### Upload configuration

```shell
curl -X POST http://glass.local/glass/config --data-binary @config.yaml
curl -X POST http://glass.local/glass/restart
```

### View current configuration

```shell
curl http://glass.local/glass/config
```

### Upload secrets

```shell
curl -X POST http://glass.local/glass/secrets --data-binary @secrets.yaml
curl -X POST http://glass.local/glass/restart
```

### Upload an asset

```shell
curl -X POST http://glass.local/glass/assets/background.jpg --data-binary @background.jpg
```

### List assets

```shell
curl http://glass.local/glass/assets
```

### Download an asset

```shell
curl http://glass.local/glass/assets/background.jpg -o background.jpg
```

### Delete an asset

```shell
curl -X DELETE http://glass.local/glass/assets/background.jpg
```

### OS update (RAUC)

```shell
curl -X POST http://glass.local/os/update \
  -H 'Content-Type: application/json' \
  -d '{"url":"https://github.com/glasslabs/os/releases/download/v1.2.3/glassos-v1.2.3-rpi4.raucb"}'

# Then reboot to activate the new image
curl -X POST http://glass.local/os/reboot
```

### Check OS slot status

```shell
curl http://glass.local/os/status
```
