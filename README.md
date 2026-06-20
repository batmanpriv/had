# 🚀 HAD - Hyper Advanced Downloader

[![Go Version](https://img.shields.io/badge/Go-1.26+-00ADD8?style=flat&logo=go)](https://golang.org)
[![Platform](https://img.shields.io/badge/platform-Windows%20%7C%20Linux%20%7C%20macOS%20%7C%20ARM64-lightgrey)]()

**English** | [**فارسی**](https://github.com/batmanpriv/had/blob/main/readmeFA.md)

---

A powerful, multi-threaded downloader with support for **HTTP/HTTPS**, **FTP/FTPS**, **SFTP**, **Metalink**, **MITM Capture Proxy**, **proxy connections**, **smart resume capabilities**, **JSON-RPC interface**, **complete website backup tool**, **HLS streaming support**, **queue management**, and **extensive post-processing**. Built for speed and reliability.

## 🎉 New in v3.3.2

HAD v3.3.2 introduces major new features and significant improvements:

### ✨ Core Enhancements
- 📺 **HLS/M3U8 Streaming Support** - Download HTTP Live Streams with built-in or FFmpeg backend
- 📋 **Priority Queue System** - Manage downloads with priority-based queues
- 🔄 **Bandwidth Scheduling** - Set time-based download windows (e.g., only at night)
- 📢 **Multi-channel Notifications** - Telegram, Discord, and Desktop notifications
- 🪞 **Smart Mirror Selection** - Auto-select fastest mirror via latency probing
- 🗜️ **Post-Processing** - Auto-extract archives, rename, and move files after download
- 🧩 **Deduplication** - Smart duplicate URL detection and filtering
- 🎯 **Per-file Thread Optimization** - Auto-adjust threads based on file size

### 🔧 Browser Extension Overhaul
- 🍪 **Full Cookie Management** - View, edit, delete, import/export cookies per site
- 🎯 **Site-Specific Cookies** - Automatically shows cookies for the current tab
- 📋 **Multi-format Export** - Export cookies as Header String, JSON, or Netscape format
- 🔄 **Import Support** - Import cookies from JSON, Netscape, or header strings
- 🔍 **Cookie Search** - Search and filter cookies by name or value
- ⚡ **Proxy Presets** - Quick presets for common proxy configurations
- 🏃 **Auto-Restore** - Automatically restore proxy on browser startup

### 🎯 Smart Capture Proxy Updates
- 🧠 **Enhanced Detection** - Better confidence scoring and hidden URL extraction
- 🔍 **Body Scanning** - Deep inspection of HTML, JSON, and JavaScript for hidden links
- 📊 **Advanced Signals** - Multi-factor URL scoring (path, query, headers, referer)
- 🎯 **Better Deduplication** - Intelligent normalization to avoid duplicates
- 📝 **Richer Metadata** - Captures method, status code, content-type, and more

## ✨ Features

### Core Downloader
- 🧵 **Multi-threaded downloads** - Maximize bandwidth utilization with adaptive buffering (32KB-4MB)
- 📡 **Multiple protocols** - HTTP, HTTPS, FTP, FTPS, SFTP with full authentication support
- 🔄 **Resume support** - Interrupt and resume downloads seamlessly with session files
- 🕸️ **Proxy support** - SOCKS4, SOCKS5, HTTP/HTTPS proxies with authentication
- 📁 **Batch downloading** - Download from file lists with comments support
- 🕷️ **Web scraping** - Extract and download links from web pages with interactive selection
- 🔍 **Extension filtering** - Filter downloads by file extensions
- ⚡ **Adaptive buffering** - Automatically optimizes buffer sizes based on speed (32KB-4MB)
- 💾 **Session saving** - Save progress every 10 seconds and resume later
- 🎨 **Beautiful progress bars** - Real-time visual feedback with per-thread progress
- 🌍 **Cross-platform** - Windows, Linux, macOS, ARM64
- 🔐 **Integrity checks** - SHA256, SHA1, MD5 verification
- 📺 **HLS streaming** - Download M3U8 playlists and segments (FFmpeg or pure-Go)
- 📋 **Priority queues** - Download URLs with priority-based ordering
- ⏰ **Bandwidth scheduling** - Set time windows for downloads (e.g., 00:00-06:00)
- 📢 **Notifications** - Telegram, Discord, and desktop notifications on completion/failure

### MITM Capture Proxy
- 🔒 **HTTPS interception** - Full Man-in-the-Middle proxy capabilities with auto-certificate installation
- 🎯 **Auto-detection** - Automatically detects videos, music, images, documents, archives
- 🔍 **Hidden URL extraction** - Scrapes HTML, JSON, and JavaScript for buried links
- 📊 **Confidence scoring** - 0-100% confidence system with multi-factor scoring
- 📝 **Multi-format output** - Saves captured links to both TXT and JSON
- 🔧 **Custom extensions** - Add your own file extensions to capture
- 🎯 **Domain filtering** - Focus on specific domains only
- 📐 **Size filtering** - Min/max file size constraints
- 🔄 **Auto-download** - Option to automatically download captured files
- 🍪 **Cookie support** - Pass cookies through the proxy
- 🔐 **Auto-certificate** - Automatic CA certificate installation (Windows, macOS, Linux)
- 🧠 **Smart deduplication** - 10-minute dedupe window with URL normalization

### Download from Captured JSON
- 📦 **Batch download** - Download all captured files with one command
- ⚡ **Smart threading** - Auto-adjusts threads based on file size (1-8 threads)
- 🔄 **Concurrent downloads** - Download multiple files simultaneously
- 📊 **Unified progress** - Track all downloads in a single beautiful interface
- 💾 **Resume support** - Interrupted downloads can be resumed
- 🧹 **Filename sanitization** - Automatically cleans titles for safe filenames

### Website Downloader (Web Backup)
- 🌐 **Full site mirroring** - Crawl and backup entire websites
- 📄 **Single page backup** - Save page with all dependencies
- 🎯 **SPA support** - Handle hash-based routing (#!/ and #! paths)
- 🖼️ **Asset rewriting** - Automatic URL rewriting for offline browsing
- 🚀 **Concurrent crawling** - Configurable page and asset workers
- 💾 **Resumable crawls** - Save and resume interrupted backups
- 🎨 **CSS/JS processing** - Rewrite URLs in stylesheets and scripts
- 🖼️ **Iframe support** - Download iframe content recursively
- ⚡ **Rate limiting** - Per-domain request throttling (configurable)
- 🗜️ **Minification** - Optional HTML minification for smaller output
- 🔄 **Meta-refresh support** - Follow meta-refresh redirects

### Metalink Support (RFC 5854)
- 📦 **Version 3 & 4 support** - Full Metalink specification compliance
- 🔄 **Mirror selection** - Automatic priority-based mirror selection
- ✅ **Integrity verification** - Built-in checksum validation
- 📊 **Piece information** - File segmentation with hash verification

### RPC Interface
- 🔌 **JSON-RPC API** - Full remote control capabilities
- 🌐 **REST endpoints** - HTTP endpoints for status and control
- 📊 **Real-time monitoring** - Download progress and statistics
- 🎮 **Dynamic control** - Pause, resume, speed limiting via API
- 📋 **Task management** - Add, remove, and monitor download tasks

### SFTP Support
- 🔑 **SSH key authentication** - RSA, ECDSA, Ed25519 support
- 🔐 **Password authentication** - With fallback to keys
- 📁 **Full resume capability** - Interrupt and resume SFTP transfers
- ⏱️ **Configurable timeouts** - Connection and operation timeouts

## 📸 Screenshots

| Feature | Preview |
|---------|---------|
| **Multi-Thread Download** | ![Multi-Thread](https://github.com/user-attachments/assets/633999ce-c3da-4db5-b4be-b4714164a504) |
| **Real-time Progress** | ![Progress](https://github.com/user-attachments/assets/2e3f4fde-4e6c-4e79-bdae-fa9094bf2993) |
| **Session Resume (JSON)** | ![Session Resume](https://github.com/user-attachments/assets/370683d3-0d54-4b9c-8e40-f1ce2f515667) |
| **MITM Capture Proxy** | ![Capture Proxy](https://github.com/user-attachments/assets/6cba171d-632c-4aef-a654-ae33c9b21b4f) |
| **JSON Export from Proxy** | ![JSON Export](https://github.com/user-attachments/assets/7f098862-e7b4-4baa-9663-b28489e9b5e3) |
| **Website Backup (Clone)** | ![Website Backup](https://github.com/user-attachments/assets/09883fe7-14d7-4045-8269-ea3c5bd5b1ae) |
| **Web Scraping** | ![Scraping](https://github.com/user-attachments/assets/fa49cd59-418d-4690-a8dc-b7a8ab2f043b) |
| **HAD Browser Extension** | ![Extension](https://github.com/user-attachments/assets/ab0ea00b-7d44-45cf-8824-eb998a6c453d) |

## 📦 Installation

### Go Installation

```bash
go install github.com/batmanpriv/had@3.3.2
```

### From Source

```bash
git clone https://github.com/batmanpriv/had.git
cd had
go build -o had .
```

## 🚀 Quick Start

### 📺 HLS/M3U8 Streaming Download (NEW)

Download HTTP Live Streams directly:

```bash
# Download HLS stream with FFmpeg (recommended)
./had -hls https://example.com/stream.m3u8 -o ./videos

# Download HLS stream (auto-detects FFmpeg, falls back to pure-Go)
./had -hls https://example.com/playlist.m3u8 -t 8

# Download HLS from a live stream
./had -hls "https://example.com/live/index.m3u8" -o ./recordings
```

### 📋 Priority Queue System (NEW)

Manage downloads with priority-based queues:

```bash
# Download from queue file (higher priority = higher number)
./had -queue queue.txt

# Queue file format:
# https://example.com/important.zip 100
# https://example.com/normal.zip 50
# https://example.com/low.zip 10

# Add priority to single download
./had -priority 100 https://example.com/urgent-file.zip
```

### ⏰ Bandwidth Scheduling (NEW)

Schedule downloads during specific time windows:

```bash
# Download only between 02:00 and 06:00
./had -schedule-from 02:00 -schedule-to 06:00 https://example.com/large-file.zip

# Combined with speed limit during schedule
./had -schedule-from 23:00 -schedule-to 07:00 -max-speed 1048576 https://example.com/file.zip
```

### 📢 Notifications (NEW)

Get notified when downloads complete:

```bash
# Telegram notification
./had -notify-telegram 123456789 -notify-telegram-bot "your_bot_token" https://example.com/file.zip

# Discord webhook
./had -notify-discord "https://discord.com/api/webhooks/..." https://example.com/file.zip

# Desktop notification (Linux/macOS/Windows)
./had -notify-desktop https://example.com/file.zip

# Multiple notifications
./had -notify-telegram ... -notify-discord ... -notify-desktop https://example.com/file.zip
```

### 🗜️ Post-Processing (NEW)

Automatically process downloads after completion:

```bash
# Auto-extract archives after download
./had -post-extract https://example.com/archive.zip

# Move to specific directory after download
./had -post-move /completed https://example.com/file.zip

# Rename using pattern ({name}, {time} placeholders)
./had -post-rename "{name}_{time}.zip" https://example.com/file.zip

# Chain post-processing
./had -post-extract -post-move /processed -post-rename "dl_{time}" https://example.com/archive.zip
```

### 🪞 Smart Mirror Selection (NEW)

Automatically select the fastest mirror:

```bash
# Auto-select fastest mirror
./had -auto-mirror -mirrors "https://mirror1.com/file.zip,https://mirror2.com/file.zip" https://primary.com/file.zip

# Will probe all mirrors and use the one with lowest latency
```

### MITM Capture Proxy

Capture download links while browsing:

```bash
# Start capture proxy on port 8085
./had -capture-proxy :8085 -capture-types video,music

# Capture with custom extensions
./had -capture-proxy :9090 -capture-types video,archive -capture-exts .webm,.mka

# Auto-download captured files
./had -capture-proxy :8085 -capture-auto -capture-output ./downloads

# Filter by domain and confidence
./had -capture-proxy :8085 -filter-domain example.com -capture-confidence 50

# Enable body scanning (finds hidden links)
./had -capture-proxy :8085 -capture-body -verbose

# Install certificate only
./had -install-cert
```

### Browser Extension (NEW)

HAD includes a redesigned browser extension for easier proxy management and cookie editing.

#### Features
- 🍪 **Cookie Manager** - View, edit, delete cookies for the current site
- 📋 **Export/Import** - Export cookies in Header, JSON, or Netscape format
- ⚡ **Quick Presets** - One-click proxy presets (HAD, HTTP, SOCKS5, TOR)
- 🏃 **Auto-Restore** - Proxy automatically reconnects on browser start
- 🔍 **Search & Filter** - Find cookies by name or value

#### Installation
1. Open `chrome://extensions` (Chromium) or `about:debugging` (Firefox)
2. Enable Developer Mode
3. Load unpacked from `./extensions-had/`

### Download from Captured JSON

After capturing links, download everything with one command:

```bash
# Download all captured files
./had -download-json captured_links.json

# Custom output with concurrent downloads
./had -download-json captured_links.json -o ./videos -u 5

# High performance (8 threads per file, 4 concurrent)
./had -download-json captured_links.json -t 8 -u 4
```

### Basic Downloads

```bash
# Download a single file
./had https://example.com/file.zip

# Download with 16 threads
./had -t 16 https://example.com/large-file.zip

# Download multiple files
./had https://example.com/file1.zip https://example.com/file2.zip

# Download from file list
./had -f urls.txt

# Download with speed limit (1MB/s)
./had -max-speed 1048576 https://example.com/file.zip

# Download with checksum verification
./had -checksum-sha256 abc123... https://example.com/file.zip

# Download with custom headers
./had -H "Authorization: Bearer token123" https://example.com/file.zip
```

### Website Backup

```bash
# Basic full site backup
./had web -url https://example.com -mode full

# Backup entire website to specific directory
./had web -url https://example.com -mode full -output ./backup

# Backup single page with all assets
./had web -url https://example.com/about -mode single -download-external

# Backup with external CDN assets
./had web -url https://example.com -mode full -download-external -external-domains cdn.example.com,images.example.com

# High performance crawling (10 concurrent workers)
./had web -url https://example.com -mode full -concurrency 10 -max-pages 500

# Resume interrupted backup
./had web -url https://example.com -mode full -resume -output ./backup

# SPA with hash routing support
./had web -url https://app.example.com/#!/home -mode full -crawl-hash-routes

# Limit asset size and rate
./had web -url https://example.com -mode full -max-asset-size 20 -rate-limit 5
```

### Metalink Downloads

```bash
# Download from Metalink URL
./had -metalink https://example.com/file.metalink

# Download from local Metalink file
./had -metalink ./downloads/ubuntu.metalink4

# Metalink with custom output directory
./had -metalink https://example.com/file.metalink -o ./downloads
```

### RPC Server Mode

```bash
# Start RPC server on default port
./had -rpc

# Start RPC server on custom address
./had -rpc -rpc-addr 0.0.0.0:6800

# RPC with downloads directory
./had -rpc -rpc-addr localhost:6800 -o /downloads
```

**RPC Example Requests:**

```bash
# Get version info
curl -X POST http://localhost:6800/jsonrpc -d '{"method":"had.version","id":1}'

# Get global statistics
curl -X POST http://localhost:6800/jsonrpc -d '{"method":"had.getGlobalStat","id":2}'

# Add download URL
curl -X POST http://localhost:6800/jsonrpc -d '{"method":"had.addUri","params":{"uris":["https://example.com/file.zip"]},"id":3}'

# Get all files status
curl -X POST http://localhost:6800/jsonrpc -d '{"method":"had.tellAllStatus","id":4}'

# List all available methods
curl -X POST http://localhost:6800/jsonrpc -d '{"method":"system.listMethods","id":5}'

# Pause all downloads
curl -X POST http://localhost:6800/jsonrpc -d '{"method":"had.pauseAll","id":6}'

# Set speed limit to 5MB/s
curl -X POST http://localhost:6800/jsonrpc -d '{"method":"had.setSpeedLimit","params":{"speed":5242880},"id":7}'

# Shutdown had
curl -X POST http://localhost:6800/jsonrpc -d '{"method":"had.shutdown","id":8}'
```

**REST API Endpoints:**

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

# Version info
curl http://localhost:6800/api/version
```

### 🔐 Proxy Downloads

```bash
# SOCKS5 proxy
./had -proxy socks5://127.0.0.1:1080 https://example.com/file.zip

# SOCKS5 with authentication
./had -proxy socks5://user:pass@127.0.0.1:1080 https://example.com/file.zip

# SOCKS4 proxy with custom threads
./had -proxy socks4://192.168.1.1:9050 -t 16 https://example.com/file.zip

# HTTP proxy
./had -proxy http://proxy.company.com:8080 https://example.com/file.zip

# HTTPS proxy with auth
./had -proxy https://user:pass@proxy.company.com:8080 https://example.com/file.zip
```

### 📡 FTP/SFTP Downloads

```bash
# Standard FTP
./had -protocol ftp ftp://example.com/file.zip

# FTP with custom credentials
./had -protocol ftp -ftp-user myuser -ftp-pass mypass ftp://example.com/file.zip

# FTPS (FTP over TLS)
./had -protocol ftps ftps://example.com/secure-file.zip

# SFTP with password
./had -protocol sftp -sftp-user myuser -sftp-pass mypass sftp://example.com/file.zip

# SFTP with SSH key
./had -protocol sftp -ssh-key ~/.ssh/id_rsa sftp://example.com/file.zip

# SFTP with encrypted SSH key
./had -protocol sftp -ssh-key ~/.ssh/id_rsa -ssh-key-pass mypassphrase sftp://example.com/file.zip

# FTP multi-part download (auto-detects file size)
./had -protocol ftp -ftp-multipart -ftp-parts 8 ftp://example.com/large-file.zip
```

### 🕷️ Web Scraping

```bash
# Extract and download all files from a page
./had -scrape https://example.com/downloads/

# Filter by extensions
./had -scrape https://example.com/downloads/ -ex .mp4,.mp3,.zip

# Scrape with custom threads
./had -scrape https://example.com/files/ -t 16 -ex .pdf,.doc,.xls

# Scrape and download with progress
./had -scrape https://example.com/media/ -ex .jpg,.png,.gif -v
```

### 🔄 Parameterized URLs

```bash
# Simple numeric placeholder
./had -parameterized-url 'https://example.com/file{}.zip' -start 1 -end 50

# Zero-padded placeholders
./had -parameterized-url 'https://example.com/image{0}.jpg' -start 1 -end 100

# Triple zero-padded
./had -parameterized-url 'https://example.com/page{00}.html' -start 1 -end 500 -step 2

# Custom step size
./had -parameterized-url 'https://example.com/chunk{}.bin' -start 0 -end 200 -step 10
```

### 🔄 Resume Downloads

```bash
# Resume from saved session
./had session_20231215_143022.json

# Session auto-saves on interrupt (Ctrl+C)
# Progress saves every 10 seconds automatically
```

### 🍪 Cookie Support

```bash
# Load cookies from Netscape format file (Firefox/Chrome export)
./had -load-cookies cookies.txt https://example.com/private-file.zip

# Save cookies after download
./had -save-cookies output.txt https://example.com/file.zip

# Direct cookie string
./had -c "sessionid=abc123; user=test" https://example.com/file.zip

# Load and save cookies
./had -load-cookies cookies.txt -save-cookies newcookies.txt https://example.com/file.zip
```

### 🔐 NetRC Authentication

```bash
# Use .netrc file for authentication
./had -netrc ~/.netrc https://example.com/private/file.zip

# .netrc file format:
# machine example.com login myuser password mypass
# default login anonymous password user@example.com
```

## ⚙️ Command Line Options

### Core Options

| Option | Default | Description |
|--------|---------|-------------|
| `-t` | CPU cores | Number of parallel download threads per file (auto-adjusts for small files) |
| `-o` | `.` | Destination directory for downloads |
| `-u` | `2` | Maximum simultaneous file downloads |
| `-r` | `5` | Retries per segment |
| `-timeout` | `30` | Network timeout in seconds |
| `-v` | `false` | Verbose mode with per-thread progress |
| `-save-session` | `true` | Save session to JSON if interrupted |
| `-f` | `""` | File containing download URLs (one per line) |

### HLS/Streaming Options (NEW)

| Option | Default | Description |
|--------|---------|-------------|
| `-hls` | `""` | HLS/M3U8 stream URL to download |
| FFmpeg | Auto-detected | Uses FFmpeg if available, falls back to pure-Go |

### Queue Options (NEW)

| Option | Default | Description |
|--------|---------|-------------|
| `-queue` | `""` | Queue file with URLs and priorities (format: url priority) |
| `-priority` | `0` | Download priority for this job (higher = first) |

### Bandwidth Options (NEW)

| Option | Default | Description |
|--------|---------|-------------|
| `-schedule-from` | `""` | Download window start (HH:MM) |
| `-schedule-to` | `""` | Download window end (HH:MM) |

### Notification Options (NEW)

| Option | Default | Description |
|--------|---------|-------------|
| `-notify-telegram` | `""` | Telegram chat ID for notifications |
| `-notify-telegram-bot` | `""` | Telegram bot token |
| `-notify-discord` | `""` | Discord webhook URL |
| `-notify-desktop` | `false` | Enable desktop notifications |

### Post-Processing Options (NEW)

| Option | Default | Description |
|--------|---------|-------------|
| `-post-extract` | `false` | Auto-extract archive after download (zip, tar, gz, rar, 7z) |
| `-post-move` | `""` | Move file to this directory after download |
| `-post-rename` | `""` | Rename pattern after download ({name}, {time} placeholders) |

### Mirror Options (NEW)

| Option | Default | Description |
|--------|---------|-------------|
| `-mirrors` | `""` | Comma-separated mirror URLs |
| `-auto-mirror` | `false` | Auto-select fastest mirror via latency probing |

### Capture Proxy Options

| Option | Default | Description |
|--------|---------|-------------|
| `-capture-proxy` | `""` | Start MITM proxy (e.g., :8085) |
| `-capture-types` | `video,music` | File types: video,music,image,document,archive,all |
| `-capture-exts` | `""` | Custom extensions (comma-separated) |
| `-capture-auto` | `false` | Auto-download captured files |
| `-capture-output` | `captured` | Output directory for auto-downloads |
| `-capture-confidence` | `30` | Confidence threshold (0-100) |
| `-capture-min-size` | `1024` | Minimum file size in bytes |
| `-capture-max-size` | `0` | Maximum file size (0=unlimited) |
| `-capture-save` | `captured_links.txt` | File to save captured links |
| `-capture-header` | `""` | Custom HTTP headers (can be repeated) |
| `-capture-cookie` | `""` | Cookie for requests |
| `-filter-domain` | `""` | Filter specific domain |
| `-filter-pattern` | `""` | URL pattern filter |
| `-capture-body` | `false` | Capture request/response bodies |
| `-install-cert` | `true` | Auto-install CA certificate |

### Download from JSON Options

| Option | Default | Description |
|--------|---------|-------------|
| `-download-json` | `""` | Download all files from captured JSON file |
| `-u` | `3` | Max concurrent downloads (when using -download-json) |
| `-o` | `captured_downloads` | Output directory |
| `-t` | Auto | Threads per file (auto-adjusted by size) |

### Network Options

| Option | Description |
|--------|-------------|
| `-proxy` | Proxy address (socks4://, socks5://, http://) |
| `-protocol` | Force protocol: auto, http, https, ftp, ftps, sftp |
| `-H` | Custom HTTP header (can be repeated) |
| `-c` | Cookie header value |
| `-load-cookies` | Load cookies from Netscape format file |
| `-save-cookies` | Save cookies to file in Netscape format |
| `-netrc` | Path to .netrc file for authentication |
| `-gzip` | Enable gzip/deflate encoding (default: true) |

### Speed & Cache Options

| Option | Default | Description |
|--------|---------|-------------|
| `-max-speed` | `0` | Maximum download speed in bytes/second (0 = unlimited) |
| `-disk-cache` | `32MB` | Disk cache size in bytes (write buffer) |

### Integrity Check Options

| Option | Description |
|--------|-------------|
| `-check-integrity` | Verify file integrity after download |
| `-checksum-sha256` | Expected SHA256 hash for integrity check |
| `-checksum-md5` | Expected MD5 hash for integrity check |
| `-checksum-sha1` | Expected SHA1 hash for integrity check |

### FTP/SFTP Options

| Option | Default | Description |
|--------|---------|-------------|
| `-ftp-user` | `anonymous` | FTP/SFTP username |
| `-ftp-pass` | `anonymous@example.com` | FTP/SFTP password |
| `-ssh-user` | `""` | SSH username for SFTP |
| `-ssh-pass` | `""` | SSH password for SFTP |
| `-ssh-key` | `""` | SSH private key file for SFTP |
| `-ssh-key-pass` | `""` | SSH private key passphrase |
| `-ftp-multipart` | `true` | Enable FTP multi-part download |
| `-ftp-parts` | `0` | Number of FTP parts (0 = auto) |

### Website Downloader Options

| Option | Default | Description |
|--------|---------|-------------|
| `-url` | Required | Target URL to backup |
| `-output` | domain name | Output directory |
| `-mode` | `single` | Crawl mode: 'single' or 'full' |
| `-max-pages` | `100` | Maximum pages for full-site mode |
| `-concurrency` | `5` | Number of concurrent workers |
| `-download-external` | `false` | Download external assets |
| `-external-domains` | `""` | Comma-separated external domains to include |
| `-cookies` | `""` | Cookies (format: name1=value1; name2=value2) |
| `-user-agent` | `Mozilla/5.0...` | User-Agent header |
| `-timeout` | `30` | Request timeout in seconds |
| `-retries` | `3` | Number of retries on failure |
| `-minify` | `false` | Minify HTML output |
| `-resume` | `false` | Resume interrupted crawl |
| `-rate-limit` | `10` | Requests per second per domain |
| `-max-asset-size` | `50` | Maximum asset size in MB |
| `-crawl-iframes` | `true` | Download iframe content |
| `-crawl-hash-routes` | `true` | Handle hash-based routing for SPAs |
| `-follow-meta-refresh` | `true` | Follow meta-refresh redirects |

### Metalink Options

| Option | Description |
|--------|-------------|
| `-metalink` | Metalink URL or file path (RFC 5854) |

### RPC Options

| Option | Default | Description |
|--------|---------|-------------|
| `-rpc` | `false` | Enable JSON-RPC interface |
| `-rpc-addr` | `localhost:6800` | RPC server address |

### Parameterized URL Options

| Option | Default | Description |
|--------|---------|-------------|
| `-parameterized-url` | `""` | URL pattern with {} as placeholder |
| `-start` | `1` | Start index for parameterized URLs |
| `-end` | `100` | End index for parameterized URLs |
| `-step` | `1` | Step for parameterized URLs |

### Scraping Options

| Option | Description |
|--------|-------------|
| `-scrape` | URL to scrape for downloadable links |
| `-ex` | Filter extensions (e.g., .mp4,.mp3,.zip) |

### Daemon Options

| Option | Default | Description |
|--------|---------|-------------|
| `-daemon` | `false` | Run as daemon process in background |
| `-pid-file` | `/tmp/had.pid` | PID file path for daemon mode |

## 📝 File Format Examples

### URLs File (urls.txt)

```text
# This is a comment
https://example.com/file1.zip
https://example.com/file2.zip
ftp://ftp.example.com/large-file.iso
https://example.com/document.pdf
sftp://sftp.example.com/backup.tar.gz
```

### Queue File (queue.txt) (NEW)

```text
# Format: URL priority (higher = processed first)
https://example.com/urgent.zip 100
https://example.com/important.zip 75
https://example.com/normal.zip 50
https://example.com/low.zip 10
# Comments are supported
```

### Captured Links JSON (captured_links.json)

```json
[
  {
    "url": "https://example.com/video.mp4",
    "file_type": "video",
    "extension": ".mp4",
    "size": 104857600,
    "title": "sample video",
    "source_url": "https://example.com/",
    "timestamp": "2024-01-15T10:30:00Z",
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
    "timestamp": "2024-01-15T10:31:00Z",
    "confidence": 90,
    "method": "GET",
    "status_code": 200,
    "content_type": "audio/mpeg",
    "downloaded": false
  }
]
```

### Session Files

Session files are auto-saved as `{filename}.json` and `{filename}.progress`. To resume:

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

## 🎯 Workflow Examples

### Complete Download Automation (NEW)

```bash
# Full automated workflow with scheduling, notifications, and post-processing
./had \
  -schedule-from 02:00 \
  -schedule-to 06:00 \
  -max-speed 5242880 \
  -post-extract \
  -post-move /processed \
  -notify-telegram 123456789 \
  -notify-telegram-bot "your_bot_token" \
  -notify-desktop \
  -queue queue.txt
```

### HLS Streaming with Notifications (NEW)

```bash
# Download HLS stream and get notified
./had -hls https://example.com/live.m3u8 -o ./videos \
  -notify-telegram 123456789 -notify-telegram-bot "token" \
  -post-move /completed
```

### Capture Proxy with Auto-Download

**Step 1: Start the capture proxy**

```bash
./had -capture-proxy :8085 -capture-types video -capture-body -verbose -capture-auto -capture-output ./captured
```

**Step 2: Configure your browser**

Set FoxyProxy or system proxy to `localhost:8085`

**Step 3: Browse normally**

The proxy captures all video links automatically and saves them to `captured_links.txt` and `captured_links.json`

**Step 4: Download all captured files**

```bash
./had -download-json captured_links.json -o ./videos -u 5 -t 8
```

### Browser Extension - Quick Proxy (NEW)

1. Click the HAD extension icon in your browser toolbar
2. Click one of the preset buttons (HAD:8085, HTTP:8080, SOCKS5:1080, TOR:9050)
3. Click **"ACTIVATE"** to enable proxy
4. The status bar shows "ACTIVE" with uptime

### Browser Extension - Cookie Management (NEW)

1. Navigate to any website
2. Click the HAD extension icon
3. Switch to the **"COOKIES"** tab
4. View all cookies for the current site
5. Click on a cookie value to expand and see full value
6. Use **EDIT**, **DELETE**, or **COPY** buttons
7. Export cookies in Header, JSON, or Netscape format
8. Import cookies from JSON, Netscape, or header strings

### Browser Extension - Bypass List (NEW)

1. Click the HAD extension icon
2. Switch to the **"CONFIG"** tab
3. Edit the bypass list (hosts that bypass the proxy)
4. Click **"SAVE BYPASS LIST"**

## 🎨 Output Preview

### Normal Download Progress

```text
══════════════════════════════════════════════════════════════════
                      DOWNLOAD STATUS
══════════════════════════════════════════════════════════════════
⬇️ 1. large-file.zip ████████████████████░░░░░░░░░░░░ 65.2%  1.2GB/1.8GB  ⚡ 12.5 MB/s  ETA: 45s
⬇️ 2. document.pdf   ████████████████████████████░░░░ 82.1%  8.2MB/10.0MB  ⚡ 2.3 MB/s  ETA: 2s
⏳ 3. image.jpg      ░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░ 0.0%  0B/5.2MB       ⏳ Waiting
──────────────────────────────────────────────────────────────────
Avg Speed: 14.8 MB/s  Instant: 12.5 MB/s  Active: 2
Files: 1/3  Downloaded: 1.2GB / 1.8GB (65.2%)  Elapsed: 45.2s
Remaining: 45s  Left: 0.6GB
```

### Capture Proxy Output

```text
╔════════════════════════════════════════════════════════════════╗
║              CAPTURE PROXY - ADVANCED MODE v3.3.2             ║
╚════════════════════════════════════════════════════════════════╝

✓ Proxy: :8085
✓ Capturing: video, music
✓ Save file: captured_links.txt
⚠ Body capture: enabled (may slow down proxy)

Configure FoxyProxy:
  • HTTP Proxy: localhost:8085

🎯 Waiting for traffic...

[HIDDEN] Found in body: https://cdn.example.com/video.mp4
🎬 [VIDEO]  | 85% | unknown | sample video | GET
  https://cdn.example.com/video.mp4
```

### Verbose Mode Output (Per-thread Progress)

```text
     └─ Threads Progress [6/8 completed]:
        T1: ✅ [██████████] Complete
        T2: ✅ [██████████] Complete
        T3: ⬇️ [████████░░] Downloading (85.3%)
        T4: ⬇️ [██████░░░░] Downloading (62.1%)
        T5: ✅ [██████████] Complete
        T6: ✅ [██████████] Complete
        T7: ✅ [██████████] Complete
        T8: ⏳ [░░░░░░░░░░] Waiting
```

### Website Downloader Output

```text
[*] Mode: Full Site (max 100 pages)
[*] Full-site mode: crawling example.com
[1] https://example.com/
[2] https://example.com/about
[3] https://example.com/contact
  [↓] style.css
  [↓] script.js
  [↓] logo.png
  [↓] font.woff2

[✓] Backup completed in 12.5s
    Pages: 15 | Assets: 87 | Size: 24.32 MB
```

### RPC Server Output

```text
[INFO] Starting JSON-RPC server on localhost:6800
[INFO] Available methods:
  - had.addUri
  - had.addUrls
  - had.remove
  - had.removeAll
  - had.tellStatus
  - had.tellAllStatus
  - had.getGlobalStat
  - had.getFiles
  - had.pause
  - had.pauseAll
  - had.resume
  - had.resumeAll
  - had.setSpeedLimit
  - had.getSpeedLimit
  - had.setMaxParallel
  - had.setThreads
  - had.scrape
  - had.shutdown
  - had.version
  - system.listMethods
```

### HLS Download Progress (NEW)

```text
📺 HLS via ffmpeg: https://example.com/stream.m3u8
   Downloading HLS stream...
   Processing segments: 45/67
```

## 🔧 Manual Extension Installation & Certificate Setup

### Extension for Capturing (extension-had)

HAD includes a redesigned browser extension for easier capture proxy integration. The extension files are located in the `extensions-had/` folder.

#### Manual Installation

**For Chrome / Brave / Edge (Chromium-based):**

1. Open your browser and navigate to `chrome://extensions`
2. Enable **"Developer mode"** (toggle in top-right corner)
3. Click **"Load unpacked"** button
4. Navigate to HAD extension folder: `./extensions-had`
5. Select the folder and click **"Select Folder"**
6. The HAD extension should now appear in your extensions list

**For Firefox:**

1. Open Firefox and navigate to `about:debugging`
2. Click on **"This Firefox"** in left sidebar
3. Click **"Load Temporary Add-on"** button
4. Navigate to `./extensions-had/` folder
5. Select the `manifest.json` file
6. **Note:** Firefox loads extensions temporarily. For permanent installation, you'll need to package and sign the extension

#### Extension Features (NEW)

1. **Proxy Management Tab**
   - Quick presets for common proxy configurations
   - Host, Port, and Scheme configuration
   - Test connection button
   - Activate/Deactivate controls
   - Status display with uptime

2. **Cookie Management Tab**
   - View all cookies for the current site
   - Edit cookie values inline
   - Delete individual cookies
   - Copy cookies to clipboard
   - Export in Header, JSON, or Netscape format
   - Import from JSON, Netscape, or header strings
   - Search and filter cookies
   - Clear all cookies for the site

3. **Configuration Tab**
   - Auto-restore proxy on browser startup
   - Bypass list management

### CA Certificate Installation

For HTTPS interception to work properly, you must install HAD's Certificate Authority (CA) certificate on your system.

#### Automatic Installation (Recommended)

```bash
# Install certificate automatically
./had -install-cert

# Or run with capture proxy (auto-installs if needed)
./had -capture-proxy :8085
```

#### Manual Installation

When automatic installation fails, follow these steps:

**Step 1: Generate the certificate**

```bash
# Run HAD once to generate the certificate
./had -capture-proxy :8085
# Certificate will be created as 'had.crt' in the current directory
# Press Ctrl+C to stop after generation
```

**Step 2: Install certificate on your OS**

**Windows:**
1. Double-click `had.crt` file
2. Click **"Install Certificate"**
3. Select **"Local Machine"** (requires admin privileges)
4. Choose **"Place all certificates in the following store"**
5. Click **"Browse"** and select **"Trusted Root Certification Authorities"**
6. Click **"OK"** → **"Next"** → **"Finish"**
7. Restart your browser

**Linux (Ubuntu/Debian):**

```bash
# Copy certificate to system trust store
sudo cp had.crt /usr/local/share/ca-certificates/had.crt
sudo update-ca-certificates

# For Firefox (manual)
# Go to about:preferences#privacy → View Certificates → Import → Select had.crt
# Check "Trust this CA to identify websites"
```

**Linux (RHEL/Fedora):**

```bash
sudo cp had.crt /etc/pki/ca-trust/source/anchors/
sudo update-ca-trust
```

**macOS:**

```bash
# Add to system keychain
sudo security add-trusted-cert -d -r trustRoot -k /Library/Keychains/System.keychain had.crt

# Or double-click had.crt → Add to Keychain → Trust → Always Trust
```

**Step 3: Verify installation**

```bash
# Start capture proxy
./had -capture-proxy :8085

# Configure browser to use localhost:8085 as HTTP/HTTPS proxy
# Visit https://example.com - you should NOT see certificate warnings
```

### Troubleshooting Certificate Issues

| Problem | Solution |
|---------|----------|
| Certificate warning in browser | Reinstall certificate as Trusted Root |
| Extension not capturing | Check proxy settings (localhost:8085) |
| "ERR_PROXY_CONNECTION_FAILED" | Ensure HAD is running with `-capture-proxy` |
| Firefox shows "Connection not secure" | Manually import `had.crt` to Firefox certificate store |
| Automatic installation fails | Run as administrator/root or use manual method |

## 🛠️ Building from Source

### Prerequisites

- Go 1.26 or higher
- GCC (for Windows builds)
- FFmpeg (optional, for HLS downloads - fallback to pure-Go)

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

# Cross-compile all platforms
GOOS=linux GOARCH=amd64 go build -o had-linux-amd64 main.go
GOOS=linux GOARCH=386 go build -o had-linux-386 main.go
GOOS=linux GOARCH=arm64 go build -o had-linux-arm64 main.go
GOOS=windows GOARCH=amd64 go build -o had-windows-amd64.exe main.go
GOOS=windows GOARCH=386 go build -o had-windows-386.exe main.go
GOOS=darwin GOARCH=amd64 go build -o had-darwin-amd64 main.go
GOOS=darwin GOARCH=arm64 go build -o had-darwin-arm64 main.go
```

## 🐛 Troubleshooting

### Common Issues

| Question | Solution |
|----------|----------|
| **Q: Capture proxy not capturing anything?** | • Install CA certificate: `./had -install-cert`<br>• Check browser proxy settings (localhost:port)<br>• Ensure HTTPS sites show certificate warning (accept it)<br>• Use `-capture-body` to see hidden links |
| **Q: Download from JSON not working?** | • Verify captured_links.json exists<br>• Check URLs are valid in the JSON file<br>• Try with `-v` flag for debug output<br>• Ensure output directory is writable |
| **Q: Certificate installation fails?** | • Run as administrator/root<br>• Follow manual instructions displayed<br>• Check if had.crt file was created<br>• Disable antivirus temporarily |
| **Q: Slow download speeds?** | • Increase thread count: `-t 16`<br>• Check if server supports range requests<br>• Try FTP multi-part for FTP files<br>• Reduce proxy latency if using proxy |
| **Q: Proxy not working?** | • Verify proxy format: `socks5://host:port`<br>• Ensure proxy is reachable<br>• Try without authentication first<br>• Check firewall settings |
| **Q: Resume not working?** | • Ensure server supports `Accept-Ranges: bytes`<br>• Check if session files exist in download directory<br>• Manual resume: `./had session_file.json`<br>• Some servers don't support resume |
| **Q: Website crawl hangs?** | • Use `-resume` flag to continue interrupted crawl<br>• Reduce concurrency: `-concurrency 3`<br>• Increase timeout: `-timeout 60`<br>• Add rate limiting: `-rate-limit 5` |
| **Q: Assets not downloading?** | • Enable `-download-external` for external resources<br>• Add external domains with `-external-domains`<br>• Check `-max-asset-size` limit<br>• Verify network connectivity to CDN |
| **Q: SFTP connection fails?** | • Verify SSH key permissions (600)<br>• Check if server supports SFTP (not just SSH)<br>• Try password authentication first<br>• Increase timeout: `-timeout 60` |
| **Q: Metalink not working?** | • Verify file is valid XML/RFC 5854 compliant<br>• Check if URLs in metalink are accessible<br>• Try downloading individual URLs directly<br>• Validate checksums if provided |
| **Q: RPC server not responding?** | • Check if port is open: `netstat -an | grep 6800`<br>• Verify firewall allows the port<br>• Use `localhost` instead of `0.0.0.0` for testing<br>• Check logs for binding errors |
| **Q: Daemon mode not starting?** | • Verify PID file directory is writable<br>• Check if another instance is running<br>• Review system logs for errors<br>• Try running without daemon mode first |
| **Q: HLS download not working?** | • Ensure FFmpeg is installed for best results<br>• Check if URL is accessible<br>• Try with `-v` flag for debug output<br>• Verify M3U8 playlist is valid |
| **Q: Queue not processing?** | • Check queue file format (URL priority)<br>• Ensure priority is a valid number<br>• Verify file path is correct<br>• Check for duplicate entries |
| **Q: Notifications not working?** | • Verify Telegram bot token and chat ID<br>• Check Discord webhook URL format<br>• Ensure network connectivity<br>• Try with desktop notifications enabled |
| **Q: Extension not showing cookies?** | • Refresh the page<br>• Click "REFRESH" button in extension<br>• Ensure you're on the correct site<br>• Check if cookies exist in browser storage |

## 📊 Performance Tips

1. **For large files (1GB+):** Use `-t` flag with 8-16 threads
2. **For many small files:** Use `-u` flag to increase parallel downloads (10-20)
3. **For slow connections:** Reduce threads to 2-4 and increase timeout to 60s
4. **For capture proxy:** Use `-capture-confidence 30` to balance accuracy and speed
5. **For batch downloads from JSON:** Use `-u 5` and `-t auto` (auto-adjusts by size)
6. **For website backup:** Start with `-mode single` to test, then use `-mode full`
7. **For rate-limited sites:** Use `-rate-limit` to avoid being blocked (5-10 req/s)
8. **For high bandwidth connections:** Increase threads to 32-64 for maximum speed
9. **For limited disk space:** Reduce `-disk-cache` to 8MB or lower
10. **For many concurrent downloads:** Use `-u` with 5-10 to avoid connection limits
11. **For SFTP transfers:** Use SSH keys instead of passwords for better performance
12. **For Metalink downloads:** Let had auto-select best mirrors
13. **For HLS streams:** Ensure FFmpeg is installed for optimal performance
14. **For scheduled downloads:** Use `-schedule-from` and `-schedule-to` to avoid peak hours
15. **For post-processing:** Chain commands with `-post-extract` and `-post-move`

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
Description=had Downloader Service
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
```

### Custom Headers Example

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

### Complete Automated Workflow (NEW)

```bash
# Download from queue with scheduling, notifications, and post-processing
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

## 🔒 Security Note

The capture proxy uses a self-signed CA certificate to intercept HTTPS traffic. This certificate must be trusted by your system for HTTPS interception to work. The `-install-cert` flag attempts to install it automatically, but you may need to do it manually depending on your system permissions.

**Important:** Only use this tool on networks and websites you own or have permission to test. Intercepting HTTPS traffic without authorization may violate terms of service or laws in your jurisdiction.

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

## 🙏 Acknowledgments

- Thanks to all contributors and users who reported issues
- Built with ❤️ using Go
- Special thanks to the goproxy library for MITM proxy capabilities

---

**⭐ Star this repository if you find it useful!**

[Report Bug](https://github.com/batmanpriv/had/issues) · [Request Feature](https://github.com/batmanpriv/had/issues) · [View Releases](https://github.com/batmanpriv/had/releases)
