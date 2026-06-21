# 🚀 HAD — Hyper Advanced Downloader

[![Go Version](https://img.shields.io/badge/Go-1.26+-00ADD8?style=flat&logo=go)](https://golang.org)
[![Platform](https://img.shields.io/badge/platform-Windows%20%7C%20Linux%20%7C%20macOS%20%7C%20ARM64-lightgrey)]()

**English** | [**فارسی**](https://github.com/batmanpriv/had/blob/main/readmeFA.md)

---

## 📋 Table of Contents

- [🎉 What's New in v3.6.0](#-whats-new-in-v360)
- [✨ Features at a Glance](#-features-at-a-glance)
- [📸 Screenshots](#-screenshots)
- [📦 Installation](#-installation)
- [🚀 Quick Start](#-quick-start)
- [🌐 Web UI (NEW)](#-web-ui-new)
- [🖥️ CLI Usage – Detailed Examples](#️-cli-usage--detailed-examples)
  - [Basic Downloads](#basic-downloads)
  - [HLS / M3U8 Streaming](#hls--m3u8-streaming)
  - [Priority Queue System](#priority-queue-system)
  - [Bandwidth Scheduling](#bandwidth-scheduling)
  - [Notifications](#notifications)
  - [Post-Processing](#post-processing)
  - [Smart Mirror Selection](#smart-mirror-selection)
  - [Capture Proxy (MITM)](#capture-proxy-mitm)
  - [Browser Extension](#browser-extension)
  - [Website Backup (Web Downloader)](#website-backup-web-downloader)
  - [FTP / SFTP](#ftp--sftp)
  - [Web Scraping](#web-scraping)
  - [Parameterized URLs](#parameterized-urls)
  - [Cookie Support](#cookie-support)
  - [Proxy Support](#proxy-support)
  - [Metalink Downloads](#metalink-downloads)
  - [RPC & REST API](#rpc--rest-api)
  - [Resume Downloads](#resume-downloads)
  - [Integrity Checks](#integrity-checks)
  - [Daemon Mode](#daemon-mode)
- [⚙️ Complete Command Line Reference](#️-complete-command-line-reference)
  - [Core Options](#core-options)
  - [HLS / Streaming Options](#hls--streaming-options)
  - [Queue & Priority Options](#queue--priority-options)
  - [Bandwidth Scheduling Options](#bandwidth-scheduling-options)
  - [Notification Options](#notification-options)
  - [Post-Processing Options](#post-processing-options)
  - [Mirror Options](#mirror-options)
  - [Capture Proxy Options](#capture-proxy-options)
  - [Web UI Options](#web-ui-options)
  - [Network Options](#network-options)
  - [Speed & Cache Options](#speed--cache-options)
  - [Integrity Check Options](#integrity-check-options)
  - [FTP/SFTP Options](#ftpsftp-options)
  - [Website Downloader Options](#website-downloader-options)
  - [Metalink Options](#metalink-options)
  - [RPC Options](#rpc-options)
  - [Parameterized URL Options](#parameterized-url-options)
  - [Scraping Options](#scraping-options)
  - [Daemon Options](#daemon-options)
- [📁 File Format Examples](#-file-format-examples)
- [❓ Frequently Asked Questions (Q&A)](#-frequently-asked-questions-qa)
  - [General Questions](#general-questions)
  - [Download Questions](#download-questions)
  - [Capture Proxy Questions](#capture-proxy-questions)
  - [Web UI Questions](#web-ui-questions)
  - [Browser Extension Questions](#browser-extension-questions)
  - [HLS / Streaming Questions](#hls--streaming-questions)
  - [RPC & API Questions](#rpc--api-questions)
  - [Troubleshooting Questions](#troubleshooting-questions)
- [📊 Performance Tips](#-performance-tips)
- [🔧 Advanced Usage](#-advanced-usage)
  - [Running as Daemon (Linux)](#running-as-daemon-linux)
  - [Using Environment Variables](#using-environment-variables)
  - [Combining with Other Tools](#combining-with-other-tools)
  - [Custom Headers](#custom-headers)
  - [Complete Automated Workflow](#complete-automated-workflow)
- [🔒 Security Note](#-security-note)
- [🛠️ Building from Source](#️-building-from-source)
- [🤝 Contributing](#-contributing)
- [🙏 Acknowledgments](#-acknowledgments)

---

## 🎉 What's New in v3.6.0

HAD v3.6.0 brings a **complete Web UI**, smarter capture proxy, better HLS support, priority queues, post-processing, notifications, and much more.

**Key highlights:**

| Feature | Description |
|---------|-------------|
| 🖥️ **Brand new Web UI** | Full graphical dashboard for managing downloads, stats, console, and settings (mobile‑friendly) |
| 🔄 **Web UI auto‑refresh** | Real‑time updates without manual reload |
| 📡 **Live log streaming** | See HAD logs directly in the browser via Server‑Sent Events |
| 🧠 **Smarter capture proxy** | Better confidence scoring, body scanning, hidden URL extraction |
| 📺 **HLS/M3U8 streaming** | Download live streams with FFmpeg or pure‑Go fallback |
| 📋 **Priority queue system** | Order downloads by importance (higher number = higher priority) |
| ⏰ **Bandwidth scheduling** | Set day/night speed limits or time‑based download windows |
| 📢 **Multi‑channel notifications** | Telegram, Discord, desktop alerts |
| 🗜️ **Post‑processing** | Auto‑extract, rename, move files after download |
| 🪞 **Smart mirror selection** | Automatically picks the fastest mirror via latency probing |
| 🍪 **Browser extension** | Proxy manager + cookie editor for Chrome/Edge/Firefox |
| 🔐 **Certificate auto‑install** | One‑click CA certificate setup for HTTPS interception |
| 🧩 **Deduplication** | Smart duplicate URL detection and filtering |
| 🎯 **Per‑file thread optimization** | Auto‑adjust threads based on file size |

---

## ✨ Features at a Glance

### Core Downloader
| Feature | Description |
|---------|-------------|
| 🧵 **Multi‑threaded downloads** | Maximize bandwidth utilization with adaptive buffering (32KB – 4MB) |
| 📡 **Multiple protocols** | HTTP, HTTPS, FTP, FTPS, SFTP with full authentication support |
| 🔄 **Resume support** | Interrupt and resume downloads seamlessly with session files |
| 🕸️ **Proxy support** | SOCKS4, SOCKS5, HTTP/HTTPS proxies with authentication |
| 📁 **Batch downloading** | Download from file lists with comments support |
| 🕷️ **Web scraping** | Extract and download links from web pages with interactive selection |
| 🔍 **Extension filtering** | Filter downloads by file extensions |
| ⚡ **Adaptive buffering** | Automatically optimizes buffer sizes based on speed (32KB‑4MB) |
| 💾 **Session saving** | Save progress every 10 seconds and resume later |
| 🎨 **Beautiful progress bars** | Real‑time visual feedback with per‑thread progress |
| 🌍 **Cross‑platform** | Windows, Linux, macOS, ARM64 |
| 🔐 **Integrity checks** | SHA256, SHA1, MD5 verification |

### HLS / M3U8 Streaming
| Feature | Description |
|---------|-------------|
| 📺 **HLS streaming** | Download M3U8 playlists and segments |
| 🎬 **FFmpeg integration** | Uses FFmpeg if available for best performance |
| 🔄 **Pure‑Go fallback** | Falls back to native Go implementation if FFmpeg is not found |
| 📡 **Live & VOD** | Supports both live streams and video‑on‑demand |

### Queue & Scheduling
| Feature | Description |
|---------|-------------|
| 📋 **Priority queues** | Download URLs with priority‑based ordering (higher number = first) |
| ⏰ **Bandwidth scheduling** | Set time windows for downloads (e.g., 00:00‑06:00) |
| 🔄 **Automatic pausing** | Downloads automatically pause outside the schedule window |

### Notifications
| Feature | Description |
|---------|-------------|
| 📢 **Telegram** | Send completion/failure notifications via Telegram bot |
| 💬 **Discord** | Send notifications via Discord webhook |
| 💻 **Desktop** | Native desktop notifications (Linux, macOS, Windows) |
| 🔔 **Multi‑channel** | Send to multiple channels simultaneously |

### Post‑Processing
| Feature | Description |
|---------|-------------|
| 🗜️ **Auto‑extract** | Extract archives (zip, tar, gz, rar, 7z, etc.) |
| 📂 **Auto‑move** | Move completed files to a specified directory |
| ✏️ **Auto‑rename** | Rename files using patterns with `{name}` and `{time}` placeholders |
| 🔗 **Chaining** | Chain multiple post‑processing actions |

### Smart Mirror Selection
| Feature | Description |
|---------|-------------|
| 🪞 **Auto‑probing** | Automatically tests all mirrors for latency |
| ⚡ **Speed ranking** | Ranks mirrors by download speed |
| 🔄 **Auto‑switch** | Switches to the fastest mirror if the primary fails |

### MITM Capture Proxy
| Feature | Description |
|---------|-------------|
| 🔒 **HTTPS interception** | Full Man‑in‑the‑Middle proxy with auto‑certificate installation |
| 🎯 **Auto‑detection** | Automatically detects videos, music, images, documents, archives |
| 🔍 **Hidden URL extraction** | Scrapes HTML, JSON, and JavaScript for buried links |
| 📊 **Confidence scoring** | 0‑100% confidence system with multi‑factor scoring |
| 📝 **Multi‑format output** | Saves captured links to both TXT and JSON |
| 🔧 **Custom extensions** | Add your own file extensions to capture |
| 🎯 **Domain filtering** | Focus on specific domains only |
| 📐 **Size filtering** | Min/max file size constraints |
| 🔄 **Auto‑download** | Option to automatically download captured files |
| 🍪 **Cookie support** | Pass cookies through the proxy |
| 🔐 **Auto‑certificate** | Automatic CA certificate installation (Windows, macOS, Linux) |
| 🧠 **Smart deduplication** | 10‑minute dedupe window with URL normalization |

### Web UI (NEW in v3.6.0)
| Feature | Description |
|---------|-------------|
| 🎨 **Dark theme** | Beautiful dark‑mode dashboard |
| 📋 **Download list** | Real‑time download list with per‑file progress |
| 📊 **Per‑thread progress** | Shows progress of each thread (when verbose mode is enabled) |
| ⚡ **Global stats** | Speed, ETA, uptime, and overall progress |
| ➕ **Add URLs** | Paste one or multiple URLs, set threads and speed limit |
| 🕷️ **Scrape** | Enter a URL and auto‑download all detected files |
| 💾 **Sessions** | View and resume saved sessions |
| 📜 **History** | See completed downloads with size, speed, duration |
| 🖥️ **Console** | Live log streaming from HAD |
| 📈 **Stats** | Real‑time speed chart, total downloaded, ETA, uptime |
| 🛠️ **Tools** | Metadata inspector, mirror tester, checksum verifier, bandwidth scheduler |
| ⚙️ **Settings** | Configure speed limit, parallel downloads, threads, output dir, RPC address, auth token |
| 📱 **Mobile‑responsive** | Works on phones and tablets |

### Website Downloader (Web Backup)
| Feature | Description |
|---------|-------------|
| 🌐 **Full site mirroring** | Crawl and backup entire websites |
| 📄 **Single page backup** | Save page with all dependencies |
| 🎯 **SPA support** | Handle hash‑based routing (`#!/` and `#!` paths) |
| 🖼️ **Asset rewriting** | Automatic URL rewriting for offline browsing |
| 🚀 **Concurrent crawling** | Configurable page and asset workers |
| 💾 **Resumable crawls** | Save and resume interrupted backups |
| 🎨 **CSS/JS processing** | Rewrite URLs in stylesheets and scripts |
| 🖼️ **Iframe support** | Download iframe content recursively |
| ⚡ **Rate limiting** | Per‑domain request throttling (configurable) |
| 🗜️ **Minification** | Optional HTML minification for smaller output |
| 🔄 **Meta‑refresh support** | Follow meta‑refresh redirects |

### Metalink Support (RFC 5854)
| Feature | Description |
|---------|-------------|
| 📦 **Version 3 & 4 support** | Full Metalink specification compliance |
| 🔄 **Mirror selection** | Automatic priority‑based mirror selection |
| ✅ **Integrity verification** | Built‑in checksum validation |
| 📊 **Piece information** | File segmentation with hash verification |

### RPC Interface
| Feature | Description |
|---------|-------------|
| 🔌 **JSON‑RPC API** | Full remote control capabilities |
| 🌐 **REST endpoints** | HTTP endpoints for status and control |
| 📊 **Real‑time monitoring** | Download progress and statistics |
| 🎮 **Dynamic control** | Pause, resume, speed limiting via API |
| 📋 **Task management** | Add, remove, and monitor download tasks |

### SFTP Support
| Feature | Description |
|---------|-------------|
| 🔑 **SSH key authentication** | RSA, ECDSA, Ed25519 support |
| 🔐 **Password authentication** | With fallback to keys |
| 📁 **Full resume capability** | Interrupt and resume SFTP transfers |
| ⏱️ **Configurable timeouts** | Connection and operation timeouts |

---

## 📸 Screenshots

| Feature | Preview |
|---------|---------|
| **Web UI – Dashboard** | ![Web UI Desktop](https://github.com/user-attachments/assets/864c5bce-d55d-4f9c-a91a-c4f30736e03a) |
| **Web UI – Mobile** | ![Web UI Mobile](https://github.com/user-attachments/assets/72c9972e-62dd-4cc1-95e3-019293ab103f) |
| **Web UI – Console** | ![Console](https://github.com/user-attachments/assets/0403b1e7-e404-4fb8-ae2e-f2316879d8a7) |
| **Web UI – Tools** | ![Tools](https://github.com/user-attachments/assets/c69ab745-f6fd-4e10-b386-7373ff39e401) |
| **Multi‑Thread Download** | ![Multi-Thread](https://github.com/user-attachments/assets/633999ce-c3da-4db5-b4be-b4714164a504) |
| **Real‑time Progress** | ![Progress](https://github.com/user-attachments/assets/2e3f4fde-4e6c-4e79-bdae-fa9094bf2993) |
| **Session Resume (JSON)** | ![Session Resume](https://github.com/user-attachments/assets/370683d3-0d54-4b9c-8e40-f1ce2f515667) |
| **MITM Capture Proxy** | ![Capture Proxy](https://github.com/user-attachments/assets/6cba171d-632c-4aef-a654-ae33c9b21b4f) |
| **JSON Export from Proxy** | ![JSON Export](https://github.com/user-attachments/assets/7f098862-e7b4-4baa-9663-b28489e9b5e3) |
| **Website Backup (Clone)** | ![Website Backup](https://github.com/user-attachments/assets/09883fe7-14d7-4045-8269-ea3c5bd5b1ae) |
| **Web Scraping** | ![Scraping](https://github.com/user-attachments/assets/fa49cd59-418d-4690-a8dc-b7a8ab2f043b) |
| **HAD Browser Extension** | ![Extension](https://github.com/user-attachments/assets/ab0ea00b-7d44-45cf-8824-eb998a6c453d) |

---

## 📦 Installation

### Using Go

```bash
go install github.com/batmanpriv/had@v3.6.0
```

### From Source

```bash
git clone https://github.com/batmanpriv/had.git
cd had
go build -o had .
```

### Pre‑built Binaries

Download the latest release from the [Releases page](https://github.com/batmanpriv/had/releases) for your platform.

---

## 🚀 Quick Start

### 1. Basic Download – Single File

```bash
./had https://example.com/file.zip
```

**What it does:**
- Downloads `file.zip` to the current directory
- Uses the default number of threads (CPU cores)
- Shows a progress bar

### 2. Download with Custom Threads

```bash
./had -t 16 https://example.com/large-file.zip
```

**What it does:**
- Uses 16 parallel threads for maximum speed
- Recommended for files larger than 1GB

### 3. Download Multiple Files

```bash
./had https://example.com/file1.zip https://example.com/file2.zip
```

**What it does:**
- Downloads both files simultaneously (up to 2 concurrent by default)
- Shows combined progress

### 4. Download from a List

```bash
./had -f urls.txt
```

**What it does:**
- Reads URLs from `urls.txt` (one per line)
- Supports comments starting with `#`

### 5. Speed Limit (1 MB/s)

```bash
./had -max-speed 1048576 https://example.com/file.zip
```

**What it does:**
- Caps download speed at 1 MB/s (1048576 bytes/sec)
- Useful for bandwidth‑limited connections

### 6. Resume an Interrupted Download

```bash
./had session_20250622_143022.json
```

**What it does:**
- Resumes from the saved session file
- Automatically created when you press Ctrl+C

---

## 🌐 Web UI (NEW)

The Web UI gives you a full graphical interface to manage downloads, view stats, and control HAD from your browser.

### Start the Web UI

```bash
./had -web-ui
```

Or use the short form:

```bash
./had webui
```

Then open your browser at:  
👉 **http://localhost:8090**

### Web UI Configuration

| Environment Variable | Description | Default |
|----------------------|-------------|---------|
| `HAD_WEB_ADDR` | Web UI bind address | `:8090` |
| `HAD_RPC_ADDR` | RPC endpoint | `http://localhost:6800` |
| `HAD_TOKEN` | Optional bearer token for authentication | (none) |

**Example with custom settings:**

```bash
HAD_WEB_ADDR=:9000 HAD_TOKEN=secret123 ./had -web-ui
```

### Web UI Features in Detail

#### Dashboard
- **Download list** – see all active downloads with file names, sizes, progress bars, and speed
- **Per‑thread progress** – when verbose mode is enabled, shows progress for each thread
- **Global stats** – total speed, ETA, uptime, active downloads
- **Overall progress** – aggregate progress bar for all downloads
- **Action buttons** – pause all, resume all, remove all

#### Add URL
- **URL input** – paste one or multiple URLs (one per line)
- **Threads** – set threads per file (1‑32)
- **Speed limit** – choose from presets or enter custom value
- **Output directory** – specify where to save files
- **Clipboard paste** – paste URLs directly from clipboard
- **Quick presets** – one‑click example URLs for testing

#### Scrape
- **URL input** – enter a webpage URL
- **Auto‑detection** – finds and downloads all media files (videos, music, archives, etc.)
- **Live log** – shows scraping progress and detected links

#### Sessions
- **View sessions** – list all saved resume sessions
- **Resume** – continue interrupted downloads
- **Delete** – remove unwanted session files

#### History
- **Completed downloads** – list of finished downloads with size, speed, duration
- **Clear** – remove all history entries

#### Console
- **Live log streaming** – see HAD logs in real‑time
- **Pause/Resume** – stop or resume log streaming
- **Clear** – clear the console output

#### Stats
- **Speed chart** – real‑time speed history graph (last 60 seconds)
- **Statistics** – speed, total downloaded, completed files, active downloads, uptime, ETA
- **Overall progress** – aggregate progress bar
- **Server info** – version, speed limit, max parallel, threads, output dir, paused status

#### Tools
- **Metadata Inspector** – fetch file info, content type, size, checksum before downloading
- **Mirror Speed Test** – test multiple mirrors and rank them by speed and latency
- **Checksum Verifier** – compute and verify MD5, SHA‑1, SHA‑256 hashes
- **Bandwidth Scheduler** – set day/night speed limits

#### Settings
- **RPC Address** – HAD RPC server endpoint
- **Auth Token** – optional bearer token for authentication
- **Auto Refresh** – refresh interval (1s, 2s, 5s, off)
- **Global Speed Limit** – set per‑second byte limit (0 = unlimited)
- **Max Parallel Downloads** – maximum simultaneous file downloads
- **Threads per File** – concurrent segments per download
- **Output Directory** – default save path

### Web UI Shortcuts

| Shortcut | Action |
|----------|--------|
| `Ctrl+K` | Focus the Add URL input |
| `Ctrl+R` | Refresh the dashboard |

---

## 🖥️ CLI Usage – Detailed Examples

### Basic Downloads

#### Download a Single File

```bash
./had https://example.com/file.zip
```

**Explanation:**
- Downloads `file.zip` to the current directory
- Uses the default number of threads (CPU cores)
- Shows a progress bar with speed and ETA

#### Download with Custom Threads

```bash
./had -t 16 https://example.com/large-file.zip
```

**Explanation:**
- Uses 16 parallel threads for maximum speed
- Recommended for files larger than 1GB
- Each thread downloads a separate segment of the file

#### Download Multiple Files

```bash
./had https://example.com/file1.zip https://example.com/file2.zip https://example.com/file3.zip
```

**Explanation:**
- Downloads all three files simultaneously
- Default concurrency is 2 (use `-u` to change)
- Shows combined progress for all downloads

#### Download from a File List

```bash
./had -f urls.txt
```

**Explanation:**
- Reads URLs from `urls.txt` (one per line)
- Supports comments starting with `#`
- Example `urls.txt`:
  ```text
  # This is a comment
  https://example.com/file1.zip
  https://example.com/file2.zip
  ftp://ftp.example.com/large-file.iso
  ```

#### Download with Speed Limit

```bash
./had -max-speed 1048576 https://example.com/file.zip
```

**Explanation:**
- Caps download speed at 1 MB/s (1048576 bytes/sec)
- Useful for bandwidth‑limited connections
- Prevents HAD from saturating your network

#### Download with Custom Output Directory

```bash
./had -o ./downloads https://example.com/file.zip
```

**Explanation:**
- Saves the file to `./downloads/` instead of the current directory
- Creates the directory if it doesn't exist

#### Download with Verbose Output

```bash
./had -v https://example.com/file.zip
```

**Explanation:**
- Shows detailed per‑thread progress
- Displays which segments are being downloaded
- Useful for debugging slow downloads

---

### HLS / M3U8 Streaming

#### Download HLS Stream (Auto‑Detect FFmpeg)

```bash
./had -hls https://example.com/stream.m3u8 -o ./videos
```

**Explanation:**
- Detects if FFmpeg is installed
- Uses FFmpeg for optimal performance if available
- Saves to `./videos/` directory
- Output format is MP4 (if FFmpeg) or TS (if pure‑Go)

#### Force Pure‑Go HLS Downloader

```bash
./had -hls https://example.com/playlist.m3u8 -t 8
```

**Explanation:**
- If FFmpeg is not found, falls back to pure‑Go implementation
- Uses 8 threads for segment downloading
- Works on systems without FFmpeg

#### Download Live HLS Stream

```bash
./had -hls "https://example.com/live/index.m3u8" -o ./recordings
```

**Explanation:**
- Downloads live streams as they become available
- Creates a TS file with all segments
- Use `-t` to control segment download concurrency

#### HLS with Notifications

```bash
./had -hls https://example.com/live.m3u8 -o ./videos \
  -notify-telegram 123456789 -notify-telegram-bot "token" \
  -post-move /completed
```

**Explanation:**
- Downloads HLS stream
- Sends Telegram notification on completion
- Moves the completed file to `/completed`

---

### Priority Queue System

#### Create a Queue File

Create `queue.txt`:

```text
# Format: URL priority (higher = processed first)
https://example.com/urgent.zip 100
https://example.com/important.zip 75
https://example.com/normal.zip 50
https://example.com/low.zip 10
https://example.com/very-low.zip 1
# Comments are supported
```

#### Download from Queue

```bash
./had -queue queue.txt
```

**Explanation:**
- Processes URLs in priority order (highest first)
- URL with priority 100 downloads before priority 50
- Priority values can be any integer (positive or negative)
- Higher priority numbers are processed first

#### Single Download with Priority

```bash
./had -priority 100 https://example.com/urgent-file.zip
```

**Explanation:**
- Assigns priority 100 to this download
- Useful when combining with other downloads in the queue
- Higher priority = processed first

#### Queue with Post‑Processing

```bash
./had -queue queue.txt -post-extract -post-move /processed
```

**Explanation:**
- Downloads all items from the queue in priority order
- Extracts archives automatically
- Moves completed files to `/processed`

---

### Bandwidth Scheduling

#### Download Only During Specific Hours

```bash
./had -schedule-from 02:00 -schedule-to 06:00 https://example.com/large-file.zip
```

**Explanation:**
- Downloads only between 02:00 and 06:00
- Automatically pauses outside the schedule window
- Useful for overnight downloads

#### Combined with Speed Limit

```bash
./had -schedule-from 23:00 -schedule-to 07:00 -max-speed 1048576 https://example.com/file.zip
```

**Explanation:**
- Downloads between 23:00 and 07:00
- Caps speed at 1 MB/s during the schedule
- Helps avoid impacting daytime bandwidth usage

#### Schedule with Multiple Downloads

```bash
./had -schedule-from 00:00 -schedule-to 06:00 -f urls.txt
```

**Explanation:**
- Downloads all URLs from `urls.txt` during the schedule window
- Automatically pauses if outside the window
- Resumes when the window opens again

---

### Notifications

#### Telegram Notification

```bash
./had -notify-telegram 123456789 -notify-telegram-bot "your_bot_token" https://example.com/file.zip
```

**Explanation:**
- Sends a notification to Telegram chat ID `123456789`
- Uses the bot token for authentication
- Notifies on download completion or failure

#### Discord Webhook Notification

```bash
./had -notify-discord "https://discord.com/api/webhooks/..." https://example.com/file.zip
```

**Explanation:**
- Sends a notification to Discord via webhook
- Creates an embed with file name and size
- Notifies on completion or failure

#### Desktop Notification

```bash
./had -notify-desktop https://example.com/file.zip
```

**Explanation:**
- Shows a native desktop notification
- Works on Linux (`notify-send`), macOS (`osascript`), and Windows (PowerShell)
- Notifies on completion or failure

#### Multiple Notification Channels

```bash
./had -notify-telegram 123456789 -notify-telegram-bot "token" \
  -notify-discord "https://discord.com/api/webhooks/..." \
  -notify-desktop https://example.com/file.zip
```

**Explanation:**
- Sends notifications to Telegram, Discord, and desktop simultaneously
- All channels receive the same notification

---

### Post‑Processing

#### Auto‑Extract Archives

```bash
./had -post-extract https://example.com/archive.zip
```

**Explanation:**
- Automatically extracts the archive after download
- Supported formats: `.zip`, `.tar`, `.gz`, `.rar`, `.7z`, `.bz2`, `.xz`
- Extracts to a directory named after the archive

#### Move Files After Download

```bash
./had -post-move /completed https://example.com/file.zip
```

**Explanation:**
- Moves the completed file to `/completed/`
- Creates the directory if it doesn't exist
- Useful for organizing downloads

#### Rename Files After Download

```bash
./had -post-rename "{name}_{time}.zip" https://example.com/file.zip
```

**Explanation:**
- Renames the file using placeholders:
  - `{name}` – original filename (without extension)
  - `{time}` – current Unix timestamp
- Example: `file.zip` → `file_1719000000.zip`

#### Chain Multiple Post‑Processing Actions

```bash
./had -post-extract -post-move /processed -post-rename "dl_{time}" https://example.com/archive.zip
```

**Explanation:**
- Extracts the archive
- Moves the extracted files to `/processed/`
- Renames the directory to `dl_1719000000`

#### Post‑Processing with Queue

```bash
./had -queue queue.txt -post-extract -post-move /media/completed
```

**Explanation:**
- Downloads all items from the queue in priority order
- Extracts archives
- Moves completed files to `/media/completed`

---

### Smart Mirror Selection

#### Auto‑Select Fastest Mirror

```bash
./had -auto-mirror -mirrors "https://mirror1.com/file.zip,https://mirror2.com/file.zip" https://primary.com/file.zip
```

**Explanation:**
- Probes all mirrors for latency
- Selects the fastest mirror automatically
- Primary URL is used as a fallback

#### Mirrors with Multiple URLs

```bash
./had -mirrors "https://us.example.com/file.zip,https://eu.example.com/file.zip,https://asia.example.com/file.zip" https://primary.com/file.zip
```

**Explanation:**
- Tests all three mirrors
- Uses the fastest one based on latency
- Automatically switches if a mirror fails

#### Mirror Selection with Fallback

```bash
./had -auto-mirror -mirrors "https://mirror1.com/file.zip,https://mirror2.com/file.zip" -retries 10 https://primary.com/file.zip
```

**Explanation:**
- Probes mirrors and selects the fastest
- Retries up to 10 times if a segment fails
- Switches mirrors on failure

---

### Capture Proxy (MITM)

#### Basic Capture Proxy

```bash
./had -capture-proxy :8085 -capture-types video,music
```

**Explanation:**
- Starts a MITM proxy on port 8085
- Captures video and music files
- Saves captured links to `captured_links.txt`

#### Capture with Custom Extensions

```bash
./had -capture-proxy :9090 -capture-types video,archive -capture-exts .webm,.mka
```

**Explanation:**
- Starts proxy on port 9090
- Captures video and archives
- Also captures `.webm` and `.mka` files

#### Auto‑Download Captured Files

```bash
./had -capture-proxy :8085 -capture-auto -capture-output ./downloads
```

**Explanation:**
- Automatically downloads captured files
- Saves files to `./downloads/`
- Uses smart threading based on file size

#### Domain Filtering

```bash
./had -capture-proxy :8085 -filter-domain example.com -capture-confidence 50
```

**Explanation:**
- Only captures URLs from `example.com`
- Confidence threshold of 50%
- Ignores links with confidence below 50

#### Body Scanning (Find Hidden Links)

```bash
./had -capture-proxy :8085 -capture-body -verbose
```

**Explanation:**
- Scans HTML, JSON, and JavaScript bodies for hidden URLs
- Finds links not visible in the page source
- May add some latency

#### Install Certificate Only

```bash
./had -install-cert
```

**Explanation:**
- Installs the CA certificate for HTTPS interception
- Required for the capture proxy to work
- Runs without starting the proxy

#### Capture Proxy with Custom Headers

```bash
./had -capture-proxy :8085 -capture-header "Authorization: Bearer token123" -capture-header "X-API-Key: abc123"
```

**Explanation:**
- Adds custom headers to all requests
- Useful for authenticated sites
- Multiple headers are supported

#### Capture with Size Filtering

```bash
./had -capture-proxy :8085 -capture-min-size 1048576 -capture-max-size 1073741824
```

**Explanation:**
- Only captures files between 1 MB and 1 GB
- Ignores very small and very large files
- Helps avoid noise

---

### Browser Extension

#### Installation

**For Chrome / Brave / Edge (Chromium‑based):**

1. Open your browser and navigate to `chrome://extensions`
2. Enable **"Developer mode"** (toggle in top‑right corner)
3. Click **"Load unpacked"** button
4. Navigate to HAD extension folder: `./extensions-had`
5. Select the folder and click **"Select Folder"**

**For Firefox:**

1. Open Firefox and navigate to `about:debugging`
2. Click on **"This Firefox"** in left sidebar
3. Click **"Load Temporary Add-on"** button
4. Navigate to `./extensions-had/` folder
5. Select the `manifest.json` file

#### Extension Features

**Proxy Management Tab:**
- **Quick presets** – one‑click proxy presets (HAD:8085, HTTP:8080, SOCKS5:1080, TOR:9050)
- **Manual config** – set host, port, and protocol
- **Test connection** – check if the proxy is reachable
- **Activate/Deactivate** – enable or disable the proxy
- **Status display** – shows uptime and active status

**Cookie Management Tab:**
- **View cookies** – see all cookies for the current site
- **Search** – filter cookies by name or value
- **Edit** – modify cookie values inline
- **Delete** – remove individual cookies
- **Copy** – copy cookies to clipboard
- **Export** – export in Header String, JSON, or Netscape format
- **Import** – import from JSON, Netscape, or header strings
- **Clear all** – remove all cookies for the current site

**Configuration Tab:**
- **Auto‑restore** – automatically restore the proxy on browser startup
- **Bypass list** – hosts that bypass the proxy (one per line)
- **Save bypass list** – save changes to the bypass list

#### Quick Proxy Setup

1. Click the HAD extension icon in your browser toolbar
2. Click one of the preset buttons (HAD:8085, HTTP:8080, SOCKS5:1080, TOR:9050)
3. Click **"ACTIVATE"** to enable the proxy
4. The status bar shows "ACTIVE" with uptime

#### Cookie Management

1. Navigate to any website
2. Click the HAD extension icon
3. Switch to the **"COOKIES"** tab
4. View all cookies for the current site
5. Click on a cookie value to expand and see full value
6. Use **EDIT**, **DELETE**, or **COPY** buttons
7. Export cookies in Header, JSON, or Netscape format
8. Import cookies from JSON, Netscape, or header strings

---

### Website Backup (Web Downloader)

#### Basic Full Site Backup

```bash
./had web -url https://example.com -mode full
```

**Explanation:**
- Crawls the entire website
- Downloads all pages, CSS, JavaScript, images
- Saves to a directory named after the domain

#### Single Page Backup with Assets

```bash
./had web -url https://example.com/about -mode single -download-external
```

**Explanation:**
- Downloads only the specified page
- Includes all page assets (CSS, JS, images)
- Also downloads external assets (CDN, etc.)

#### Backup with External CDN Assets

```bash
./had web -url https://example.com -mode full -download-external -external-domains cdn.example.com,images.example.com
```

**Explanation:**
- Downloads the full site
- Includes assets from external domains
- Only downloads from the specified external domains

#### High Performance Crawling

```bash
./had web -url https://example.com -mode full -concurrency 10 -max-pages 500
```

**Explanation:**
- Uses 10 concurrent workers
- Crawls up to 500 pages
- Much faster for large sites

#### Resume Interrupted Backup

```bash
./had web -url https://example.com -mode full -resume -output ./backup
```

**Explanation:**
- Resumes a previously interrupted backup
- Uses the saved crawl state
- Saves to `./backup`

#### SPA with Hash Routing

```bash
./had web -url https://app.example.com/#!/home -mode full -crawl-hash-routes
```

**Explanation:**
- Handles hash‑based routing (`#!/` and `#!`)
- Works with single‑page applications (SPAs)
- Crawls all hash‑based routes

#### Rate Limiting and Size Limits

```bash
./had web -url https://example.com -mode full -max-asset-size 20 -rate-limit 5
```

**Explanation:**
- Limits assets to 20 MB max
- Limits requests to 5 per second per domain
- Prevents overloading the server

---

### FTP / SFTP

#### Standard FTP Download

```bash
./had -protocol ftp ftp://example.com/file.zip
```

**Explanation:**
- Connects to FTP server
- Downloads `file.zip`
- Uses anonymous login by default

#### FTP with Credentials

```bash
./had -protocol ftp -ftp-user myuser -ftp-pass mypass ftp://example.com/file.zip
```

**Explanation:**
- Uses the provided username and password
- Connects via FTP protocol

#### FTPS (FTP over TLS)

```bash
./had -protocol ftps ftps://example.com/secure-file.zip
```

**Explanation:**
- Connects using FTPS (FTP over TLS)
- Encrypts the connection

#### SFTP with Password

```bash
./had -protocol sftp -sftp-user myuser -sftp-pass mypass sftp://example.com/file.zip
```

**Explanation:**
- Connects via SFTP
- Uses password authentication

#### SFTP with SSH Key

```bash
./had -protocol sftp -ssh-key ~/.ssh/id_rsa sftp://example.com/file.zip
```

**Explanation:**
- Uses SSH private key for authentication
- Supports RSA, ECDSA, Ed25519 keys

#### SFTP with Encrypted SSH Key

```bash
./had -protocol sftp -ssh-key ~/.ssh/id_rsa -ssh-key-pass mypassphrase sftp://example.com/file.zip
```

**Explanation:**
- Uses an encrypted SSH private key
- Provides the passphrase to decrypt the key

#### FTP Multi‑Part Download

```bash
./had -protocol ftp -ftp-multipart -ftp-parts 8 ftp://example.com/large-file.zip
```

**Explanation:**
- Splits the file into 8 parts
- Downloads each part in parallel
- Much faster for large FTP files

---

### Web Scraping

#### Basic Scraping

```bash
./had -scrape https://example.com/downloads/
```

**Explanation:**
- Extracts all downloadable links from the page
- Presents them for selection
- Downloads selected files

#### Scrape with Extension Filter

```bash
./had -scrape https://example.com/downloads/ -ex .mp4,.mp3,.zip
```

**Explanation:**
- Only shows links with the specified extensions
- Filters out other file types

#### Scrape with Custom Threads

```bash
./had -scrape https://example.com/files/ -t 16 -ex .pdf,.doc,.xls
```

**Explanation:**
- Uses 16 threads for downloads
- Only shows PDF, DOC, and XLS files

#### Scrape with Verbose Output

```bash
./had -scrape https://example.com/media/ -ex .jpg,.png,.gif -v
```

**Explanation:**
- Shows detailed per‑thread progress
- Useful for debugging

---

### Parameterized URLs

#### Simple Numeric Placeholder

```bash
./had -parameterized-url 'https://example.com/file{}.zip' -start 1 -end 50
```

**Explanation:**
- Generates URLs: `file1.zip` to `file50.zip`
- Uses `{}` as the placeholder
- Downloads all 50 files

#### Zero‑Padded Placeholders

```bash
./had -parameterized-url 'https://example.com/image{0}.jpg' -start 1 -end 100
```

**Explanation:**
- Generates `image01.jpg` to `image100.jpg`
- Zero‑padding up to 2 digits (`{0}`)
- Downloads all 100 images

#### Triple Zero‑Padded

```bash
./had -parameterized-url 'https://example.com/page{00}.html' -start 1 -end 500 -step 2
```

**Explanation:**
- Generates `page001.html` to `page500.html`
- Zero‑padding up to 3 digits (`{00}`)
- Step size of 2 (odd numbers only)

#### Custom Step Size

```bash
./had -parameterized-url 'https://example.com/chunk{}.bin' -start 0 -end 200 -step 10
```

**Explanation:**
- Generates `chunk0.bin`, `chunk10.bin`, …, `chunk200.bin`
- Step size of 10
- Useful for paginated content

---

### Cookie Support

#### Load Cookies from Netscape Format File

```bash
./had -load-cookies cookies.txt https://example.com/private-file.zip
```

**Explanation:**
- Loads cookies from a Netscape format file
- Export cookies from Firefox or Chrome
- Authenticates for private downloads

**Netscape Cookie File Format:**
```text
# Netscape HTTP Cookie File
.example.com	TRUE	/	FALSE	1735689600	SESSION	abc123def456
.example.com	TRUE	/	TRUE	1735689600	SECURE	token789
```

#### Save Cookies After Download

```bash
./had -save-cookies output.txt https://example.com/file.zip
```

**Explanation:**
- Saves cookies to a Netscape format file after download
- Useful for capturing session cookies

#### Direct Cookie String

```bash
./had -c "sessionid=abc123; user=test" https://example.com/file.zip
```

**Explanation:**
- Sends the cookie string directly
- Format: `name1=value1; name2=value2`

#### Load and Save Cookies Together

```bash
./had -load-cookies cookies.txt -save-cookies newcookies.txt https://example.com/file.zip
```

**Explanation:**
- Loads cookies from `cookies.txt`
- Downloads the file
- Saves updated cookies to `newcookies.txt`

---

### Proxy Support

#### SOCKS5 Proxy

```bash
./had -proxy socks5://127.0.0.1:1080 https://example.com/file.zip
```

**Explanation:**
- Routes traffic through a SOCKS5 proxy
- Useful for anonymity or network restrictions

#### SOCKS5 with Authentication

```bash
./had -proxy socks5://user:pass@127.0.0.1:1080 https://example.com/file.zip
```

**Explanation:**
- Uses SOCKS5 with username and password
- Authentication is passed in the proxy URL

#### SOCKS4 Proxy

```bash
./had -proxy socks4://192.168.1.1:9050 -t 16 https://example.com/file.zip
```

**Explanation:**
- Routes traffic through a SOCKS4 proxy
- Uses 16 threads for the download

#### HTTP Proxy

```bash
./had -proxy http://proxy.company.com:8080 https://example.com/file.zip
```

**Explanation:**
- Routes traffic through an HTTP proxy
- Common in corporate environments

#### HTTPS Proxy with Authentication

```bash
./had -proxy https://user:pass@proxy.company.com:8080 https://example.com/file.zip
```

**Explanation:**
- Uses HTTPS proxy with authentication
- Secure connection to the proxy

#### Environment Variables for Proxy

```bash
export HTTP_PROXY=http://proxy:8080
export HTTPS_PROXY=http://proxy:8080
export NO_PROXY=localhost,127.0.0.1
./had https://example.com/file.zip
```

**Explanation:**
- Uses environment variables for proxy configuration
- No need to specify `-proxy` flag
- `NO_PROXY` lists hosts to bypass the proxy

---

### Metalink Downloads

#### Download from Metalink URL

```bash
./had -metalink https://example.com/file.metalink
```

**Explanation:**
- Downloads the Metalink file
- Extracts mirror URLs
- Downloads from the best mirror

#### Download from Local Metalink File

```bash
./had -metalink ./downloads/ubuntu.metalink4
```

**Explanation:**
- Reads the Metalink file from disk
- Supports both version 3 and 4
- Downloads the file

#### Metalink with Custom Output Directory

```bash
./had -metalink https://example.com/file.metalink -o ./downloads
```

**Explanation:**
- Downloads the file to `./downloads/`
- Overrides the Metalink's output directory

---

### RPC & REST API

#### Start RPC Server

```bash
./had -rpc
```

**Explanation:**
- Starts the JSON‑RPC server on `localhost:6800`
- Enables remote control of HAD
- REST API is also available

#### Start RPC on Custom Address

```bash
./had -rpc -rpc-addr 0.0.0.0:6800
```

**Explanation:**
- Binds to all network interfaces
- Allows remote connections
- Useful for Web UI access

#### RPC with Downloads Directory

```bash
./had -rpc -rpc-addr localhost:6800 -o /downloads
```

**Explanation:**
- Sets the default download directory
- All RPC downloads go to `/downloads`

#### JSON‑RPC Example: Get Version

```bash
curl -X POST http://localhost:6800/jsonrpc -d '{"method":"had.version","id":1}'
```

**Response:**
```json
{
  "id": 1,
  "result": {
    "name": "HAD (Hyper Advanced Downloader)",
    "version": "3.6.0",
    "protocol": "had-rpc/2.0",
    "features": "http,https,ftp,ftps,sftp,hls,metalink,scrape,capture-proxy"
  }
}
```

#### JSON‑RPC Example: Get Global Stats

```bash
curl -X POST http://localhost:6800/jsonrpc -d '{"method":"had.getGlobalStat","id":2}'
```

**Response:**
```json
{
  "id": 2,
  "result": {
    "num_active": 2,
    "total_files": 5,
    "completed_files": 3,
    "total_size": 1073741824,
    "total_size_human": "1.0 GB",
    "total_downloaded": 536870912,
    "total_downloaded_human": "512.0 MB",
    "total_progress": 50.0,
    "paused": false,
    "speed_limit": 0,
    "uptime": "1h23m45s"
  }
}
```

#### JSON‑RPC Example: Add Download

```bash
curl -X POST http://localhost:6800/jsonrpc -d '{"method":"had.addUri","params":{"uris":["https://example.com/file.zip"]},"id":3}'
```

**Response:**
```json
{
  "id": 3,
  "result": {
    "gid": "000000018f3a2d4e"
  }
}
```

#### JSON‑RPC Example: Pause All Downloads

```bash
curl -X POST http://localhost:6800/jsonrpc -d '{"method":"had.pauseAll","id":6}'
```

**Response:**
```json
{
  "id": 6,
  "result": {
    "paused": 3
  }
}
```

#### JSON‑RPC Example: Set Speed Limit

```bash
curl -X POST http://localhost:6800/jsonrpc -d '{"method":"had.setSpeedLimit","params":{"speed":5242880},"id":7}'
```

**Response:**
```json
{
  "id": 7,
  "result": {
    "speed_limit": 5242880,
    "speed_human": "5.0 MB/s"
  }
}
```

#### JSON‑RPC Example: List All Methods

```bash
curl -X POST http://localhost:6800/jsonrpc -d '{"method":"system.listMethods","id":5}'
```

#### REST API Examples

```bash
# Get global status
curl http://localhost:6800/api/status

# Get all files
curl http://localhost:6800/api/files

# Get active tasks
curl http://localhost:6800/api/tasks

# Pause all downloads
curl http://localhost:6800/api/pause

# Resume all downloads
curl http://localhost:6800/api/resume

# Get version info
curl http://localhost:6800/api/version
```

---

### Resume Downloads

#### Resume from Saved Session

```bash
./had session_20250622_143022.json
```

**Explanation:**
- Resumes the download from the session file
- Session files are automatically saved on interrupt (Ctrl+C)
- Progress is saved every 10 seconds

#### Session File Location

Session files are saved as `{filename}.json` in the download directory.

#### Manual Resume

```bash
./had file.zip.json
```

**Explanation:**
- Resumes `file.zip` from its session file
- Works even if the original URL is the same

---

### Integrity Checks

#### Verify SHA‑256 Checksum

```bash
./had -checksum-sha256 abc123... https://example.com/file.zip
```

**Explanation:**
- Verifies the file's SHA‑256 hash after download
- Aborts if the hash doesn't match

#### Verify MD5 Checksum

```bash
./had -checksum-md5 abc123... https://example.com/file.zip
```

**Explanation:**
- Verifies the file's MD5 hash after download
- Aborts if the hash doesn't match

#### Verify SHA‑1 Checksum

```bash
./had -checksum-sha1 abc123... https://example.com/file.zip
```

**Explanation:**
- Verifies the file's SHA‑1 hash after download
- Aborts if the hash doesn't match

#### Check Integrity (Auto‑Detect)

```bash
./had -check-integrity https://example.com/file.zip
```

**Explanation:**
- Automatically detects the checksum type
- Verifies file integrity after download
- Requires checksum file in the same directory

---

### Daemon Mode

#### Start Daemon

```bash
./had -daemon -o /downloads https://example.com/bigfile.zip
```

**Explanation:**
- Runs HAD as a background daemon
- Continues running after logout
- Saves PID to `/tmp/had.pid`

#### Check Daemon Status

```bash
cat /tmp/had.pid
```

**Explanation:**
- Shows the PID of the running daemon
- Use to check if the daemon is running

#### Stop Daemon

```bash
kill $(cat /tmp/had.pid)
```

**Explanation:**
- Stops the daemon process
- Uses the PID from the PID file

#### Systemd Service

Create `/etc/systemd/system/had.service`:

```text
[Unit]
Description=HAD Downloader Service
After=network.target

[Service]
Type=simple
User=downloader
ExecStart=/usr/local/bin/had -daemon -o /downloads -rpc
Restart=on-failure

[Install]
WantedBy=multi-user.target
```

Then enable and start:

```bash
sudo systemctl enable had
sudo systemctl start had
```

---

## ⚙️ Complete Command Line Reference

### Core Options

| Option | Default | Description | Example |
|--------|---------|-------------|---------|
| `-t` | CPU cores | Number of parallel download threads per file | `-t 16` |
| `-o` | `.` | Destination directory for downloads | `-o ./downloads` |
| `-u` | `2` | Maximum simultaneous file downloads | `-u 5` |
| `-r` | `5` | Retries per segment | `-r 10` |
| `-timeout` | `30` | Network timeout in seconds | `-timeout 60` |
| `-v` | `false` | Verbose mode with per‑thread progress | `-v` |
| `-save-session` | `true` | Save session to JSON if interrupted | `-save-session false` |
| `-f` | `""` | File containing download URLs (one per line) | `-f urls.txt` |

### HLS / Streaming Options

| Option | Default | Description | Example |
|--------|---------|-------------|---------|
| `-hls` | `""` | HLS/M3U8 stream URL to download | `-hls https://example.com/stream.m3u8` |

### Queue & Priority Options

| Option | Default | Description | Example |
|--------|---------|-------------|---------|
| `-queue` | `""` | Queue file with URLs and priorities (format: url priority) | `-queue queue.txt` |
| `-priority` | `0` | Download priority for this job (higher = first) | `-priority 100` |

### Bandwidth Scheduling Options

| Option | Default | Description | Example |
|--------|---------|-------------|---------|
| `-schedule-from` | `""` | Download window start (HH:MM) | `-schedule-from 02:00` |
| `-schedule-to` | `""` | Download window end (HH:MM) | `-schedule-to 06:00` |

### Notification Options

| Option | Default | Description | Example |
|--------|---------|-------------|---------|
| `-notify-telegram` | `""` | Telegram chat ID for notifications | `-notify-telegram 123456789` |
| `-notify-telegram-bot` | `""` | Telegram bot token | `-notify-telegram-bot "token"` |
| `-notify-discord` | `""` | Discord webhook URL | `-notify-discord "https://..."` |
| `-notify-desktop` | `false` | Enable desktop notifications | `-notify-desktop` |

### Post‑Processing Options

| Option | Default | Description | Example |
|--------|---------|-------------|---------|
| `-post-extract` | `false` | Auto‑extract archive after download | `-post-extract` |
| `-post-move` | `""` | Move file to this directory after download | `-post-move /completed` |
| `-post-rename` | `""` | Rename pattern after download ({name}, {time}) | `-post-rename "{name}_{time}"` |

### Mirror Options

| Option | Default | Description | Example |
|--------|---------|-------------|---------|
| `-mirrors` | `""` | Comma‑separated mirror URLs | `-mirrors "mirror1.com, mirror2.com"` |
| `-auto-mirror` | `false` | Auto‑select fastest mirror via latency probing | `-auto-mirror` |

### Capture Proxy Options

| Option | Default | Description | Example |
|--------|---------|-------------|---------|
| `-capture-proxy` | `""` | Start MITM proxy (e.g., :8085) | `-capture-proxy :8085` |
| `-capture-types` | `video,music` | File types: video,music,image,document,archive,all | `-capture-types video,archive` |
| `-capture-exts` | `""` | Custom extensions (comma‑separated) | `-capture-exts .webm,.mka` |
| `-capture-auto` | `false` | Auto‑download captured files | `-capture-auto` |
| `-capture-output` | `captured` | Output directory for auto‑downloads | `-capture-output ./downloads` |
| `-capture-confidence` | `30` | Confidence threshold (0‑100) | `-capture-confidence 50` |
| `-capture-min-size` | `1024` | Minimum file size in bytes | `-capture-min-size 1048576` |
| `-capture-max-size` | `0` | Maximum file size (0=unlimited) | `-capture-max-size 1073741824` |
| `-capture-save` | `captured_links.txt` | File to save captured links | `-capture-save links.txt` |
| `-capture-header` | `""` | Custom HTTP headers (can be repeated) | `-capture-header "Auth: token"` |
| `-capture-cookie` | `""` | Cookie for requests | `-capture-cookie "session=abc"` |
| `-filter-domain` | `""` | Filter specific domain | `-filter-domain example.com` |
| `-filter-pattern` | `""` | URL pattern filter | `-filter-pattern "\.mp4$"` |
| `-capture-body` | `false` | Capture request/response bodies | `-capture-body` |
| `-install-cert` | `true` | Auto‑install CA certificate | `-install-cert` |

### Web UI Options

| Option | Default | Description | Example |
|--------|---------|-------------|---------|
| `-web-ui` | `false` | Start the Web UI server | `-web-ui` |

### Network Options

| Option | Description | Example |
|--------|-------------|---------|
| `-proxy` | Proxy address (socks4://, socks5://, http://) | `-proxy socks5://127.0.0.1:1080` |
| `-protocol` | Force protocol: auto, http, https, ftp, ftps, sftp | `-protocol ftp` |
| `-H` | Custom HTTP header (can be repeated) | `-H "User-Agent: MyBot"` |
| `-c` | Cookie header value | `-c "session=abc"` |
| `-load-cookies` | Load cookies from Netscape format file | `-load-cookies cookies.txt` |
| `-save-cookies` | Save cookies to file in Netscape format | `-save-cookies output.txt` |
| `-netrc` | Path to .netrc file for authentication | `-netrc ~/.netrc` |
| `-gzip` | Enable gzip/deflate encoding (default: true) | `-gzip false` |

### Speed & Cache Options

| Option | Default | Description | Example |
|--------|---------|-------------|---------|
| `-max-speed` | `0` | Maximum download speed in bytes/second (0 = unlimited) | `-max-speed 1048576` |
| `-disk-cache` | `32MB` | Disk cache size in bytes (write buffer) | `-disk-cache 64MB` |

### Integrity Check Options

| Option | Description | Example |
|--------|-------------|---------|
| `-check-integrity` | Verify file integrity after download | `-check-integrity` |
| `-checksum-sha256` | Expected SHA256 hash for integrity check | `-checksum-sha256 abc...` |
| `-checksum-md5` | Expected MD5 hash for integrity check | `-checksum-md5 abc...` |
| `-checksum-sha1` | Expected SHA1 hash for integrity check | `-checksum-sha1 abc...` |

### FTP/SFTP Options

| Option | Default | Description | Example |
|--------|---------|-------------|---------|
| `-ftp-user` | `anonymous` | FTP/SFTP username | `-ftp-user myuser` |
| `-ftp-pass` | `anonymous@example.com` | FTP/SFTP password | `-ftp-pass mypass` |
| `-ssh-user` | `""` | SSH username for SFTP | `-ssh-user myuser` |
| `-ssh-pass` | `""` | SSH password for SFTP | `-ssh-pass mypass` |
| `-ssh-key` | `""` | SSH private key file for SFTP | `-ssh-key ~/.ssh/id_rsa` |
| `-ssh-key-pass` | `""` | SSH private key passphrase | `-ssh-key-pass myphrase` |
| `-ftp-multipart` | `true` | Enable FTP multi‑part download | `-ftp-multipart false` |
| `-ftp-parts` | `0` | Number of FTP parts (0 = auto) | `-ftp-parts 8` |

### Website Downloader Options

| Option | Default | Description | Example |
|--------|---------|-------------|---------|
| `-url` | Required | Target URL to backup | `-url https://example.com` |
| `-output` | domain name | Output directory | `-output ./backup` |
| `-mode` | `single` | Crawl mode: 'single' or 'full' | `-mode full` |
| `-max-pages` | `100` | Maximum pages for full‑site mode | `-max-pages 500` |
| `-concurrency` | `5` | Number of concurrent workers | `-concurrency 10` |
| `-download-external` | `false` | Download external assets | `-download-external` |
| `-external-domains` | `""` | Comma‑separated external domains to include | `-external-domains cdn.com` |
| `-cookies` | `""` | Cookies (format: name1=value1; name2=value2) | `-cookies "session=abc"` |
| `-user-agent` | `Mozilla/5.0...` | User‑Agent header | `-user-agent "MyBot"` |
| `-timeout` | `30` | Request timeout in seconds | `-timeout 60` |
| `-retries` | `3` | Number of retries on failure | `-retries 5` |
| `-minify` | `false` | Minify HTML output | `-minify` |
| `-resume` | `false` | Resume interrupted crawl | `-resume` |
| `-rate-limit` | `10` | Requests per second per domain | `-rate-limit 5` |
| `-max-asset-size` | `50` | Maximum asset size in MB | `-max-asset-size 20` |
| `-crawl-iframes` | `true` | Download iframe content | `-crawl-iframes false` |
| `-crawl-hash-routes` | `true` | Handle hash‑based routing for SPAs | `-crawl-hash-routes` |
| `-follow-meta-refresh` | `true` | Follow meta‑refresh redirects | `-follow-meta-refresh` |

### Metalink Options

| Option | Description | Example |
|--------|-------------|---------|
| `-metalink` | Metalink URL or file path (RFC 5854) | `-metalink file.metalink` |

### RPC Options

| Option | Default | Description | Example |
|--------|---------|-------------|---------|
| `-rpc` | `false` | Enable JSON‑RPC interface | `-rpc` |
| `-rpc-addr` | `localhost:6800` | RPC server address | `-rpc-addr 0.0.0.0:6800` |

### Parameterized URL Options

| Option | Default | Description | Example |
|--------|---------|-------------|---------|
| `-parameterized-url` | `""` | URL pattern with {} as placeholder | `-parameterized-url 'file{}.zip'` |
| `-start` | `1` | Start index for parameterized URLs | `-start 0` |
| `-end` | `100` | End index for parameterized URLs | `-end 50` |
| `-step` | `1` | Step for parameterized URLs | `-step 2` |

### Scraping Options

| Option | Description | Example |
|--------|-------------|---------|
| `-scrape` | URL to scrape for downloadable links | `-scrape https://example.com` |
| `-ex` | Filter extensions (e.g., .mp4,.mp3,.zip) | `-ex .mp4,.mp3` |

### Daemon Options

| Option | Default | Description | Example |
|--------|---------|-------------|---------|
| `-daemon` | `false` | Run as daemon process in background | `-daemon` |
| `-pid-file` | `/tmp/had.pid` | PID file path for daemon mode | `-pid-file /var/run/had.pid` |

---

## 📁 File Format Examples

### URLs File (`urls.txt`)

```text
# This is a comment
https://example.com/file1.zip
https://example.com/file2.zip
ftp://ftp.example.com/large-file.iso
https://example.com/document.pdf
sftp://sftp.example.com/backup.tar.gz
```

### Queue File (`queue.txt`)

```text
# Format: URL priority (higher = processed first)
https://example.com/urgent.zip 100
https://example.com/important.zip 75
https://example.com/normal.zip 50
https://example.com/low.zip 10
# Comments are supported
```

### Captured Links JSON (`captured_links.json`)

```json
[
  {
    "url": "https://example.com/video.mp4",
    "file_type": "video",
    "extension": ".mp4",
    "size": 104857600,
    "title": "sample video",
    "source_url": "https://example.com/",
    "timestamp": "2025-06-22T10:30:00Z",
    "confidence": 85,
    "method": "GET",
    "status_code": 200,
    "content_type": "video/mp4",
    "downloaded": false
  },
  {
    "url": "https://example.com/music.mp3",
    "file_type": "music",
    "extension": ".mp3",
    "size": 5242880,
    "title": "sample song",
    "source_url": "https://example.com/",
    "timestamp": "2025-06-22T10:31:00Z",
    "confidence": 90,
    "method": "GET",
    "status_code": 200,
    "content_type": "audio/mpeg",
    "downloaded": false
  }
]
```

### Session Files

Session files are auto‑saved as `{filename}.json`. To resume:

```bash
./had file.zip.json
```

### .netrc File Format

```text
machine example.com
login myusername
password mysecretpass

machine github.com
login mytoken
password ghp_xxxxxxxxxxxx

default
login anonymous
password user@example.com
```

### Netscape Cookies File Format

```text
# Netscape HTTP Cookie File
.example.com	TRUE	/	FALSE	1735689600	SESSION	abc123def456
.example.com	TRUE	/	TRUE	1735689600	SECURE	token789
```

---

## ❓ Frequently Asked Questions (Q&A)

### General Questions

**Q: What is HAD?**  
A: HAD (Hyper Advanced Downloader) is a powerful, multi‑threaded downloader that supports HTTP/HTTPS, FTP/FTPS, SFTP, Metalink, MITM capture proxy, proxy connections, smart resume capabilities, JSON‑RPC interface, complete website backup, HLS streaming support, queue management, and extensive post‑processing.

**Q: What platforms does HAD support?**  
A: HAD supports Windows, Linux, macOS, and ARM64 (including Raspberry Pi).

**Q: How do I install HAD?**  
A: You can install HAD using `go install github.com/batmanpriv/had@v3.6.0`, build from source, or download pre‑built binaries from the releases page.

**Q: How do I update HAD?**  
A: If installed via Go, run `go install github.com/batmanpriv/had@v3.6.0` again. If built from source, pull the latest changes and rebuild.

**Q: Is HAD free?**  
A: Yes, HAD is open source and free to use under the MIT license.

### Download Questions

**Q: How can I speed up downloads?**  
A: Use the `-t` flag to increase threads (e.g., `-t 16`). For very large files, you can use up to 32 threads. Also, make sure the server supports range requests.

**Q: How do I resume an interrupted download?**  
A: HAD automatically saves session files every 10 seconds. To resume, simply run `./had file.zip.json` or use the `-resume` flag in the web downloader.

**Q: Why is my download stuck or slow?**  
A: Try increasing the timeout with `-timeout 60`. If using a proxy, check your proxy settings. Also, some servers may not support range requests, which limits HAD to single‑thread downloads.

**Q: Can I download multiple files at once?**  
A: Yes, use the `-u` flag to set the number of concurrent downloads (e.g., `-u 5`).

**Q: How do I download from FTP?**  
A: Use `-protocol ftp` and provide credentials with `-ftp-user` and `-ftp-pass`. For example: `./had -protocol ftp -ftp-user myuser -ftp-pass mypass ftp://example.com/file.zip`.

**Q: How do I download from SFTP?**  
A: Use `-protocol sftp` and authenticate with either a password (`-sftp-user`, `-sftp-pass`) or an SSH key (`-ssh-key`).

**Q: Can I set a speed limit?**  
A: Yes, use `-max-speed` with the speed in bytes per second. For example, `-max-speed 1048576` limits to 1 MB/s.

**Q: How do I download from a list of URLs?**  
A: Create a text file with one URL per line and use `-f filename.txt`.

**Q: Can I filter by file extension?**  
A: Yes, use the `-ex` flag with extensions separated by commas (e.g., `-ex .mp4,.mp3,.zip`).

### Capture Proxy Questions

**Q: What is the capture proxy?**  
A: The capture proxy is a MITM (Man‑in‑The‑Middle) proxy that intercepts HTTP/HTTPS traffic and automatically detects and saves download links (videos, music, images, etc.) as you browse.

**Q: How do I start the capture proxy?**  
A: Run `./had -capture-proxy :8085`. Then configure your browser to use `localhost:8085` as a proxy.

**Q: Why is the capture proxy not capturing anything?**  
A: Make sure the CA certificate is installed (`./had -install-cert`). Also, check that your browser is properly configured to use the proxy and that you're visiting HTTPS sites (you may need to accept certificate warnings).

**Q: How do I install the CA certificate?**  
A: Run `./had -install-cert`. If auto‑installation fails, follow the manual instructions displayed in the terminal.

**Q: Can I filter by domain?**  
A: Yes, use `-filter-domain example.com` to only capture links from that domain.

**Q: How does confidence scoring work?**  
A: Confidence scoring (0‑100%) is based on multiple signals: file extension, content‑type, URL patterns, headers, referer, and more. Higher confidence means the proxy is more certain the link is a downloadable file.

**Q: Can I auto‑download captured files?**  
A: Yes, use `-capture-auto` and optionally `-capture-output` to specify the directory.

**Q: What file types does the capture proxy detect?**  
A: By default, it detects video, music, image, document, and archive files. You can customize this with `-capture-types` and `-capture-exts`.

### Web UI Questions

**Q: How do I start the Web UI?**  
A: Run `./had -web-ui` or `./had webui`. Then open `http://localhost:8090` in your browser.

**Q: Can I change the Web UI port?**  
A: Yes, set the `HAD_WEB_ADDR` environment variable (e.g., `HAD_WEB_ADDR=:9000 ./had -web-ui`).

**Q: Can I secure the Web UI?**  
A: Yes, set the `HAD_TOKEN` environment variable to enable bearer token authentication.

**Q: Why can't I connect to the Web UI?**  
A: Make sure HAD is running with `-web-ui`. Check if the port is free and not blocked by a firewall. Also, ensure the RPC server is running (it starts automatically with the Web UI).

**Q: Does the Web UI work on mobile?**  
A: Yes, the Web UI is fully responsive and works on phones and tablets.

**Q: Can I use the Web UI remotely?**  
A: Yes, start HAD with `-rpc-addr 0.0.0.0:6800` and access the Web UI from another device. Make sure to secure it with a token.

**Q: How do I refresh the Web UI?**  
A: The Web UI auto‑refreshes every 2 seconds by default. You can change the interval in the Settings tab.

### Browser Extension Questions

**Q: How do I install the browser extension?**  
A: Load the `extensions-had` folder as an unpacked extension in Chrome/Edge (Developer Mode) or Firefox (about:debugging).

**Q: What does the extension do?**  
A: The extension provides a proxy manager (quick presets, activation, bypass list) and a cookie editor (view, edit, delete, export, import cookies for the current site).

**Q: How do I use the proxy presets?**  
A: Click the extension icon, then click one of the preset buttons (HAD:8085, HTTP:8080, SOCKS5:1080, TOR:9050). Then click "ACTIVATE".

**Q: How do I view cookies?**  
A: Navigate to any website, click the extension icon, and switch to the "COOKIES" tab. You'll see all cookies for the current site.

**Q: Can I edit cookies?**  
A: Yes, click the "EDIT" button next to any cookie, modify the value, and click "SAVE".

**Q: How do I export cookies?**  
A: In the "COOKIES" tab, select a format (Header, JSON, or Netscape) and click "EXPORT".

**Q: How do I import cookies?**  
A: Paste JSON, Netscape, or header string data into the import textarea and click "IMPORT & APPLY".

### HLS / Streaming Questions

**Q: What is HLS?**  
A: HLS (HTTP Live Streaming) is a streaming protocol used by many video platforms. HAD can download HLS streams and save them as MP4 or TS files.

**Q: How do I download an HLS stream?**  
A: Use `-hls` with the M3U8 playlist URL: `./had -hls https://example.com/stream.m3u8`.

**Q: Do I need FFmpeg?**  
A: No, HAD has a pure‑Go fallback. However, FFmpeg provides better performance and produces MP4 output instead of TS.

**Q: Can I download live streams?**  
A: Yes, HAD supports live HLS streams. The download continues as new segments become available.

**Q: Why is my HLS download failing?**  
A: Check if the M3U8 URL is accessible. Try with `-v` for debug output. Ensure the playlist is valid and contains segment URLs.

**Q: How do I specify the output format?**  
A: With FFmpeg, the output is automatically MP4. With pure‑Go, the output is TS. You can change the extension manually after download.

### RPC & API Questions

**Q: How do I start the RPC server?**  
A: Use `-rpc`. By default, it listens on `localhost:6800`.

**Q: What is the RPC protocol?**  
A: HAD uses JSON‑RPC 2.0 over HTTP. There are also REST endpoints for common operations.

**Q: How do I add a download via RPC?**  
A: Send a POST request to `/jsonrpc` with `{"method":"had.addUri","params":{"uris":["https://example.com/file.zip"]},"id":1}`.

**Q: Can I pause downloads via RPC?**  
A: Yes, use `had.pauseAll` or `had.pause` with a GID.

**Q: How do I get the status of a download?**  
A: Use `had.tellStatus` with the GID, or `had.tellAllStatus` for all downloads.

**Q: What methods are available?**  
A: Send `{"method":"system.listMethods","id":1}` to get a list of all methods.

### Troubleshooting Questions

**Q: HAD crashes on startup?**  
A: Check if you have the required Go version (1.26+). Ensure all dependencies are installed. Try rebuilding from source.

**Q: "permission denied" when saving files?**  
A: Make sure you have write permissions to the output directory. Use `-o` to specify a writable directory.

**Q: "connection refused" error?**  
A: Check if the server is reachable. If using a proxy, verify the proxy address and port. Increase the timeout with `-timeout 60`.

**Q: "invalid checksum" error?**  
A: The downloaded file's hash doesn't match the expected value. The file may be corrupted or the checksum is incorrect.

**Q: "too many open files" error?**  
A: Reduce the number of concurrent downloads with `-u` or the number of threads with `-t`.

**Q: "no space left on device"?**  
A: Free up disk space or use `-o` to save to a different drive with more space.

**Q: "certificate verify failed"?**  
A: For HTTPS sites, make sure the CA certificate is installed (`./had -install-cert`). For self‑signed certificates, use `-insecure`.

**Q: HLS download says "no segments found"?**  
A: The M3U8 playlist might be empty or require authentication. Try accessing the URL in a browser to verify it works.

**Q: The Web UI shows "RPC unreachable"?**  
A: Make sure the RPC server is running. Check the `HAD_RPC_ADDR` environment variable. Ensure the RPC address is correct.

**Q: The capture proxy is slow?**  
A: Disable body scanning (`-capture-body false`) or reduce the confidence threshold. Also, consider using a faster machine.

**Q: The browser extension doesn't show cookies?**  
A: Refresh the page. Click the "REFRESH" button in the extension. Make sure you're on the correct site.

---

## 📊 Performance Tips

| Scenario | Recommendation |
|----------|----------------|
| **Large files (1GB+)** | Use `-t 16` or `-t 32` for maximum speed |
| **Many small files** | Use `-u 20` to increase parallel downloads |
| **Slow connection** | Reduce threads to `2-4` and increase `-timeout 60` |
| **High bandwidth (100+ Mbps)** | Increase threads to `32-64` for maximum throughput |
| **Rate‑limited sites** | Use `-rate-limit 5` in the web downloader to avoid being blocked |
| **Capture proxy** | Use `-capture-confidence 30` for a good balance of accuracy and speed |
| **HLS streams** | Install FFmpeg for the best performance and MP4 output |
| **Scheduled downloads** | Use `-schedule-from` and `-schedule-to` to avoid peak hours |
| **Post‑processing** | Chain commands with `-post-extract` and `-post-move` |
| **SFTP transfers** | Use SSH keys instead of passwords for better performance |
| **Metalink downloads** | Let HAD auto‑select the best mirrors |
| **Disk space limited** | Reduce `-disk-cache` to 8MB or lower |
| **Many concurrent downloads** | Use `-u 5-10` to avoid connection limits |

---

## 🔧 Advanced Usage

### Running as Daemon (Linux)

```bash
# Start daemon
./had -daemon -o /downloads https://example.com/bigfile.zip

# Check status
cat /tmp/had.pid

# Stop daemon
kill $(cat /tmp/had.pid)

# Systemd service
sudo cat > /etc/systemd/system/had.service << EOF
[Unit]
Description=HAD Downloader Service
After=network.target

[Service]
Type=simple
User=downloader
ExecStart=/usr/local/bin/had -daemon -o /downloads -rpc
Restart=on-failure

[Install]
WantedBy=multi-user.target
EOF

sudo systemctl enable had
sudo systemctl start had
```

### Using Environment Variables

```bash
# Set proxy via environment
export HTTP_PROXY=http://proxy:8080
export HTTPS_PROXY=http://proxy:8080
export NO_PROXY=localhost,127.0.0.1

# Set RPC address
export HAD_RPC_ADDR=http://localhost:6800

# Set Web UI address
export HAD_WEB_ADDR=:8090

# Set auth token
export HAD_TOKEN=secret123

# Run with environment
./had https://example.com/file.zip
```

### Combining with Other Tools

```bash
# Download and extract
./had https://example.com/archive.zip && unzip archive.zip

# Download and verify signature
./had https://example.com/file.iso && sha256sum -c file.iso.sha256

# Download list from another program output
cat urls.txt | xargs -n1 ./had

# Scheduled downloads with cron
0 2 * * * /usr/local/bin/had https://example.com/daily-backup.zip -o /backups

# Download and move to specific directory
./had -post-move /media/videos https://example.com/video.mp4
```

### Custom Headers

```bash
# Multiple custom headers
./had -H "Authorization: Bearer token123" -H "X-Custom-Header: value" https://api.example.com/file.zip

# User-Agent override
./had -H "User-Agent: MyCustomBot/1.0" https://example.com/file.zip

# Referer header
./had -H "Referer: https://google.com" https://example.com/file.zip
```

### Capture Proxy with Custom Headers

```bash
# Add authentication headers to capture proxy
./had -capture-proxy :8085 -capture-header "Authorization: Bearer token123" -capture-header "X-API-Key: abc123"

# Pass cookies through proxy
./had -capture-proxy :8085 -capture-cookie "sessionid=abc123; user=test"
```

### Complete Automated Workflow

```bash
# Download from queue with scheduling, notifications, and post‑processing
./had \
  -queue queue.txt \
  -schedule-from 00:00 \
  -schedule-to 06:00 \
  -max-speed 1048576 \
  -post-extract \
  -post-move /media/completed \
  -post-rename "{name}_{time}" \
  -notify-telegram 123456789 \
  -notify-telegram-bot "your_bot_token" \
  -notify-desktop
```

---

## 🔒 Security Note

The capture proxy uses a self‑signed CA certificate to intercept HTTPS traffic. This certificate must be trusted by your system for HTTPS interception to work. The `-install-cert` flag attempts to install it automatically, but you may need to do it manually depending on your system permissions.

**Important:** Only use this tool on networks and websites you own or have permission to test. Intercepting HTTPS traffic without authorization may violate terms of service or laws in your jurisdiction.

### Certificate Installation Troubleshooting

| Problem | Solution |
|---------|----------|
| **Certificate warning in browser** | Reinstall certificate as Trusted Root |
| **Extension not capturing** | Check proxy settings (localhost:8085) |
| **"ERR_PROXY_CONNECTION_FAILED"** | Ensure HAD is running with `-capture-proxy` |
| **Firefox shows "Connection not secure"** | Manually import `had.crt` to Firefox certificate store |
| **Automatic installation fails** | Run as administrator/root or use manual method |

---

## 🛠️ Building from Source

### Prerequisites

- Go 1.26 or higher
- GCC (for Windows builds)
- FFmpeg (optional, for HLS downloads – fallback to pure‑Go)

### Build Commands

```bash
# Linux/macOS
go build -o had main.go

# Windows
GOOS=windows GOARCH=amd64 go build -o had.exe main.go

# Linux ARM64
GOOS=linux GOARCH=arm64 go build -o had-linux-arm64 main.go

# With optimizations (smaller binary)
go build -ldflags="-s -w" -o had main.go

# Cross‑compile all platforms
GOOS=linux GOARCH=amd64 go build -o had-linux-amd64 main.go
GOOS=linux GOARCH=386 go build -o had-linux-386 main.go
GOOS=linux GOARCH=arm64 go build -o had-linux-arm64 main.go
GOOS=windows GOARCH=amd64 go build -o had-windows-amd64.exe main.go
GOOS=windows GOARCH=386 go build -o had-windows-386.exe main.go
GOOS=darwin GOARCH=amd64 go build -o had-darwin-amd64 main.go
GOOS=darwin GOARCH=arm64 go build -o had-darwin-arm64 main.go
```

---

## 🤝 Contributing

Contributions are welcome! Please feel free to submit a Pull Request.

1. Fork the repository
2. Create your feature branch (`git checkout -b feature/AmazingFeature`)
3. Commit your changes (`git commit -m 'Add some AmazingFeature'`)
4. Push to the branch (`git push origin feature/AmazingFeature`)
5. Open a Pull Request

### Development Setup

```bash
git clone https://github.com/batmanpriv/had.git
cd had
go mod download
go build -o had main.go
./had -v https://example.com/test.zip
```

---

## 🙏 Acknowledgments

- Thanks to all contributors and users who reported issues
- Built with ❤️ using Go
- Special thanks to the goproxy library for MITM proxy capabilities

---

**⭐ Star this repository if you find it useful!**

[Report Bug](https://github.com/batmanpriv/had/issues) · [Request Feature](https://github.com/batmanpriv/had/issues) · [View Releases](https://github.com/batmanpriv/had/releases)
