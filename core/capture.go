package core

import (
	"bytes"
	"compress/gzip"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/elazarl/goproxy"
	"github.com/mattn/go-colorable"
)

func init() {
	_ = colorable.NewColorable(os.Stdout)
}

type FileType string

const (
	TypeVideo    FileType = "video"
	TypeMusic    FileType = "music"
	TypeImage    FileType = "image"
	TypeDocument FileType = "document"
	TypeArchive  FileType = "archive"
	TypeAll      FileType = "all"
)

var ExtensionGroups = map[FileType][]string{
	TypeVideo: {
		".mp4", ".mkv", ".avi", ".mov", ".wmv", ".flv", ".webm", ".m4v",
		".mpg", ".mpeg", ".m2ts", ".mts", ".ts", ".m3u8", ".mpd", ".iso",
		".vob", ".3gp", ".ogv", ".ogg", ".qt", ".rm", ".rmvb", ".asf",
		".divx", ".xvid", ".264", ".265", ".hevc",
	},
	TypeMusic: {
		".mp3", ".wav", ".flac", ".aac", ".ogg", ".m4a", ".opus", ".wma",
		".alac", ".ape", ".dsd", ".dff", ".dsf", ".mid", ".midi", ".ra",
		".voc", ".vox", ".aiff", ".au", ".snd", ".amr", ".awb", ".weba",
	},
	TypeImage: {
		".jpg", ".jpeg", ".png", ".gif", ".bmp", ".svg", ".webp", ".ico",
		".tiff", ".tif", ".raw", ".cr2", ".nef", ".arw", ".dng", ".heif",
		".heic", ".jfif", ".pjpeg", ".pjp", ".avif", ".apng", ".cur",
	},
	TypeDocument: {
		".pdf", ".doc", ".docx", ".xls", ".xlsx", ".ppt", ".pptx", ".txt",
		".rtf", ".odt", ".ods", ".odp", ".odg", ".csv", ".json", ".xml",
		".md", ".epub", ".mobi", ".azw", ".azw3", ".cbr", ".cbz", ".ps",
		".tex", ".log", ".ini", ".cfg", ".conf", ".yaml", ".yml",
	},
	TypeArchive: {
		".zip", ".rar", ".7z", ".tar", ".gz", ".bz2", ".xz", ".tgz",
		".tbz2", ".txz", ".zst", ".lzma", ".lz", ".lzh", ".cab", ".arj",
		".deb", ".rpm", ".pkg", ".msi", ".apk", ".ipa", ".jar", ".war",
	},
}

var contentTypeMappings = map[string]FileType{
	"video/mp4":             TypeVideo,
	"video/webm":            TypeVideo,
	"video/ogg":             TypeVideo,
	"video/x-matroska":      TypeVideo,
	"video/quicktime":       TypeVideo,
	"video/x-msvideo":       TypeVideo,
	"video/x-flv":           TypeVideo,
	"video/mpeg":            TypeVideo,
	"application/x-mpegurl": TypeVideo,
	"application/vnd.apple.mpegurl": TypeVideo,
	"audio/mpeg":            TypeMusic,
	"audio/mp4":             TypeMusic,
	"audio/ogg":             TypeMusic,
	"audio/flac":            TypeMusic,
	"audio/wav":             TypeMusic,
	"audio/webm":            TypeMusic,
	"audio/aac":             TypeMusic,
	"audio/opus":            TypeMusic,
	"image/jpeg":            TypeImage,
	"image/png":             TypeImage,
	"image/gif":             TypeImage,
	"image/webp":            TypeImage,
	"image/svg+xml":         TypeImage,
	"application/pdf":       TypeDocument,
	"application/zip":       TypeArchive,
	"application/x-rar-compressed": TypeArchive,
	"application/x-7z-compressed":  TypeArchive,
	"application/x-tar":    TypeArchive,
	"application/gzip":     TypeArchive,
}

type CaptureConfig struct {
	Port             string
	FileTypes        []FileType
	CustomExtensions []string
	Headers          map[string]string
	Cookie           string
	AutoDownload     bool
	OutputDir        string
	MinFileSize      int64
	MaxFileSize      int64
	ConfidenceLevel  int
	SaveToFile       string
	Verbose          bool
	CaptureBody      bool
	FilterDomain     string
	FilterPattern    string
	DedupeWindow     time.Duration
	MaxCaptured      int
}

type CapturedItem struct {
	URL          string            `json:"url"`
	FileType     FileType          `json:"file_type"`
	Extension    string            `json:"extension"`
	Size         int64             `json:"size"`
	Title        string            `json:"title"`
	SourceURL    string            `json:"source_url"`
	Timestamp    time.Time         `json:"timestamp"`
	Confidence   int               `json:"confidence"`
	Method       string            `json:"method"`
	StatusCode   int               `json:"status_code"`
	ContentType  string            `json:"content_type"`
	Headers      map[string]string `json:"headers,omitempty"`
	Downloaded   bool              `json:"downloaded"`
	DownloadPath string            `json:"download_path,omitempty"`
}

type RequestLog struct {
	Method      string            `json:"method"`
	URL         string            `json:"url"`
	Host        string            `json:"host"`
	Path        string            `json:"path"`
	QueryParams map[string]string `json:"query_params,omitempty"`
	Headers     map[string]string `json:"headers,omitempty"`
	Timestamp   time.Time         `json:"timestamp"`
}

type scoreSignal struct {
	points int
	reason string
}

type CaptureProxy struct {
	config      *CaptureConfig
	proxy       *goproxy.ProxyHttpServer
	captured    []CapturedItem
	seenURLs    map[string]time.Time
	requestLogs []RequestLog
	mu          sync.RWMutex
	urlFilters  []*regexp.Regexp
	httpClient  *http.Client
}

func NewCaptureProxy(config *CaptureConfig) *CaptureProxy {
	if config == nil {
		config = &CaptureConfig{
			Port:             ":8085",
			FileTypes:        []FileType{TypeVideo, TypeMusic},
			CustomExtensions: []string{},
			Headers:          make(map[string]string),
			AutoDownload:     false,
			OutputDir:        "captured",
			MinFileSize:      1024,
			MaxFileSize:      0,
			ConfidenceLevel:  40,
			SaveToFile:       "",
			Verbose:          true,
			CaptureBody:      false,
			DedupeWindow:     10 * time.Minute,
			MaxCaptured:      10000,
		}
	}

	if config.DedupeWindow == 0 {
		config.DedupeWindow = 10 * time.Minute
	}
	if config.MaxCaptured == 0 {
		config.MaxCaptured = 10000
	}

	os.MkdirAll(config.OutputDir, 0755)

	proxy := goproxy.NewProxyHttpServer()
	proxy.Verbose = false
	proxy.Tr = &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}

	cp := &CaptureProxy{
		config:      config,
		proxy:       proxy,
		captured:    make([]CapturedItem, 0),
		seenURLs:    make(map[string]time.Time),
		requestLogs: make([]RequestLog, 0),
		urlFilters:  make([]*regexp.Regexp, 0),
		httpClient: &http.Client{
			Transport: &http.Transport{
				TLSClientConfig:   &tls.Config{InsecureSkipVerify: true},
				DisableKeepAlives: true,
			},
			Timeout: 8 * time.Second,
		},
	}

	if config.FilterPattern != "" {
		if re, err := regexp.Compile(config.FilterPattern); err == nil {
			cp.urlFilters = append(cp.urlFilters, re)
		}
	}

	cp.setupHandlers()
	return cp
}

func (cp *CaptureProxy) setupHandlers() {
	cp.proxy.OnRequest().HandleConnect(goproxy.AlwaysMitm)

	cp.proxy.OnRequest().DoFunc(func(r *http.Request, ctx *goproxy.ProxyCtx) (*http.Request, *http.Response) {
		for k, v := range cp.config.Headers {
			r.Header.Set(k, v)
		}
		if cp.config.Cookie != "" {
			r.Header.Set("Cookie", cp.config.Cookie)
		}

		cp.logRequest(r)

		if cp.config.FilterDomain != "" && !strings.Contains(r.Host, cp.config.FilterDomain) {
			return r, nil
		}

		if len(cp.urlFilters) > 0 {
			matched := false
			for _, filter := range cp.urlFilters {
				if filter.MatchString(r.URL.String()) {
					matched = true
					break
				}
			}
			if !matched {
				return r, nil
			}
		}

		if cp.config.CaptureBody && (r.Method == "POST" || r.Method == "PUT") {
			body, _ := io.ReadAll(r.Body)
			r.Body = io.NopCloser(bytes.NewReader(body))
			cp.scanBodyForURLs(string(body), r)
		}

		cp.evaluateRequest(r.URL.String(), r, "")
		return r, nil
	})

	cp.proxy.OnResponse().DoFunc(func(resp *http.Response, ctx *goproxy.ProxyCtx) *http.Response {
		if resp == nil || resp.Request == nil {
			return resp
		}

		rawURL := resp.Request.URL.String()
		contentType := resp.Header.Get("Content-Type")
		contentLen := resp.ContentLength

		cp.logResponse(resp)
		cp.evaluateRequest(rawURL, resp.Request, contentType)

		if cp.config.CaptureBody {
			var body []byte
			var err error

			if strings.Contains(resp.Header.Get("Content-Encoding"), "gzip") {
				gr, gerr := gzip.NewReader(resp.Body)
				if gerr == nil {
					body, err = io.ReadAll(gr)
					gr.Close()
				}
			} else {
				body, err = io.ReadAll(resp.Body)
			}

			if err == nil {
				resp.Body = io.NopCloser(bytes.NewReader(body))
				ct := strings.ToLower(contentType)
				if strings.Contains(ct, "json") || strings.Contains(ct, "javascript") ||
					strings.Contains(ct, "text/html") || strings.Contains(ct, "text/plain") {
					cp.scanBodyForURLs(string(body), resp.Request)
				}
			}
		}

		cp.scanResponseHeaders(resp.Header, rawURL, contentType, contentLen)
		return resp
	})
}

func (cp *CaptureProxy) evaluateRequest(rawURL string, r *http.Request, contentType string) {
	if rawURL == "" {
		return
	}

	signals := cp.scoreURL(rawURL, contentType, r)
	totalScore := 0
	for _, s := range signals {
		totalScore += s.points
	}
	if totalScore > 100 {
		totalScore = 100
	}
	if totalScore < 0 {
		totalScore = 0
	}

	if totalScore < cp.config.ConfidenceLevel {
		return
	}

	detectedType, detectedExt := cp.detectTypeFromSignals(rawURL, contentType)
	if detectedType == "" {
		return
	}

	if !cp.isFileTypeAllowed(detectedType) {
		return
	}

	if cp.config.Verbose {
		cp.logAdvanced(r, signals, totalScore)
	}

	cp.capture(rawURL, r, detectedType, detectedExt, contentType, totalScore)
}

func (cp *CaptureProxy) scoreURL(rawURL, contentType string, r *http.Request) []scoreSignal {
	var signals []scoreSignal

	parsedURL, err := url.Parse(rawURL)
	if err != nil {
		return signals
	}

	pathLower := strings.ToLower(parsedURL.Path)
	urlLower := strings.ToLower(rawURL)
	queryLower := strings.ToLower(parsedURL.RawQuery)

	ext := strings.ToLower(filepath.Ext(strings.Split(parsedURL.Path, "?")[0]))
	for _, exts := range ExtensionGroups {
		for _, e := range exts {
			if ext == e {
				signals = append(signals, scoreSignal{45, "known extension: " + e})
				goto doneExt
			}
		}
	}
	for _, ce := range cp.config.CustomExtensions {
		if ext == ce {
			signals = append(signals, scoreSignal{45, "custom extension: " + ce})
			goto doneExt
		}
	}
doneExt:

	if contentType != "" {
		ctBase := strings.ToLower(strings.Split(contentType, ";")[0])
		ctBase = strings.TrimSpace(ctBase)
		if _, ok := contentTypeMappings[ctBase]; ok {
			signals = append(signals, scoreSignal{40, "content-type: " + ctBase})
		} else if strings.HasPrefix(ctBase, "video/") {
			signals = append(signals, scoreSignal{40, "content-type video/*"})
		} else if strings.HasPrefix(ctBase, "audio/") {
			signals = append(signals, scoreSignal{40, "content-type audio/*"})
		} else if strings.HasPrefix(ctBase, "image/") {
			signals = append(signals, scoreSignal{20, "content-type image/*"})
		} else if ctBase == "application/octet-stream" {
			signals = append(signals, scoreSignal{15, "octet-stream"})
		}
	}

	mediaPathKeywords := []string{
		"/video/", "/videos/", "/media/", "/stream/", "/play/", "/watch/",
		"/audio/", "/music/", "/tracks/", "/songs/", "/podcast/",
		"/hls/", "/dash/", "/vod/", "/live/", "/content/",
		"/download/", "/dl/", "/get/", "/file/", "/files/",
		"/upload/", "/uploads/", "/storage/", "/cdn/",
	}
	for _, kw := range mediaPathKeywords {
		if strings.Contains(pathLower, kw) {
			signals = append(signals, scoreSignal{10, "path keyword: " + kw})
			break
		}
	}

	queryKeywords := []string{"video", "audio", "file", "stream", "media", "download", "src", "url", "path", "source"}
	for _, kw := range queryKeywords {
		if strings.Contains(queryLower, kw+"=") {
			signals = append(signals, scoreSignal{8, "query param: " + kw})
			break
		}
	}

	cdnHosts := []string{
		"cdn.", "media.", "static.", "storage.", "assets.", "content.",
		"stream.", "video.", "audio.", "files.", "download.", "dl.",
		"s3.", "cloudfront.", "akamai.", "fastly.", "cloudflare.",
		"r2.", "blob.core.windows.net", "googleapis.com/storage",
	}
	hostLower := strings.ToLower(parsedURL.Host)
	for _, cdn := range cdnHosts {
		if strings.Contains(hostLower, cdn) {
			signals = append(signals, scoreSignal{12, "cdn host: " + cdn})
			break
		}
	}

	streamPatterns := []string{
		"m3u8", "mpd", "chunklist", "segment", ".ts?", "playlist",
		"manifest", "index.php?v=", "token=", "expires=", "sig=",
	}
	for _, pat := range streamPatterns {
		if strings.Contains(urlLower, pat) {
			signals = append(signals, scoreSignal{15, "stream pattern: " + pat})
			break
		}
	}

	if r != nil {
		referer := r.Header.Get("Referer")
		if referer != "" {
			refLower := strings.ToLower(referer)
			mediaReferers := []string{"youtube", "vimeo", "twitch", "soundcloud", "spotify",
				"netflix", "amazon", "player", "watch", "video", "stream"}
			for _, mref := range mediaReferers {
				if strings.Contains(refLower, mref) {
					signals = append(signals, scoreSignal{8, "media referer: " + mref})
					break
				}
			}
		}

		accept := r.Header.Get("Accept")
		if strings.Contains(accept, "video/") || strings.Contains(accept, "audio/") {
			signals = append(signals, scoreSignal{12, "accept header media"})
		}

		rangeHdr := r.Header.Get("Range")
		if rangeHdr != "" {
			signals = append(signals, scoreSignal{10, "range request"})
		}
	}

	noisePatterns := []string{
		".css", ".js", ".woff", ".woff2", ".ttf", ".eot",
		"/favicon", "/robots.txt", "/sitemap",
		"google-analytics", "googletagmanager", "facebook.com/tr",
		"analytics", "pixel", "beacon", "tracker",
	}
	for _, noise := range noisePatterns {
		if strings.Contains(urlLower, noise) {
			signals = append(signals, scoreSignal{-50, "noise: " + noise})
			break
		}
	}

	if strings.HasSuffix(pathLower, "/") || pathLower == "" {
		signals = append(signals, scoreSignal{-20, "directory/root path"})
	}

	return signals
}

func (cp *CaptureProxy) detectTypeFromSignals(rawURL, contentType string) (FileType, string) {
	parsedURL, err := url.Parse(rawURL)
	if err != nil {
		return "", ""
	}

	ext := strings.ToLower(filepath.Ext(strings.Split(parsedURL.Path, "?")[0]))

	for ft, exts := range ExtensionGroups {
		for _, e := range exts {
			if ext == e {
				return ft, e
			}
		}
	}

	for _, ce := range cp.config.CustomExtensions {
		if ext == ce {
			return TypeAll, ce
		}
	}

	if contentType != "" {
		ctBase := strings.ToLower(strings.Split(contentType, ";")[0])
		ctBase = strings.TrimSpace(ctBase)
		if ft, ok := contentTypeMappings[ctBase]; ok {
			return ft, ext
		}
		if strings.HasPrefix(ctBase, "video/") {
			return TypeVideo, ext
		}
		if strings.HasPrefix(ctBase, "audio/") {
			return TypeMusic, ext
		}
	}

	urlLower := strings.ToLower(rawURL)
	urlKeywords := map[FileType][]string{
		TypeVideo: {"video", "movie", "film", "stream", "vod", "hls", "dash", "m3u8", "mpd", "watch", "play"},
		TypeMusic: {"music", "audio", "song", "track", "album", "podcast", "radio"},
		TypeImage: {"image", "photo", "picture", "gallery", "img"},
		TypeDocument: {"document", "pdf", "ebook"},
		TypeArchive: {"archive", "package", "bundle", "release"},
	}

	for ft, kws := range urlKeywords {
		for _, kw := range kws {
			if strings.Contains(urlLower, "/"+kw+"/") ||
				strings.Contains(urlLower, "/"+kw+"?") ||
				strings.HasSuffix(urlLower, "/"+kw) {
				return ft, ext
			}
		}
	}

	return "", ""
}

func (cp *CaptureProxy) isFileTypeAllowed(ft FileType) bool {
	for _, allowed := range cp.config.FileTypes {
		if allowed == ft || allowed == TypeAll {
			return true
		}
	}
	return false
}

func (cp *CaptureProxy) isDuplicate(rawURL string) bool {
	cp.mu.RLock()
	defer cp.mu.RUnlock()

	normalizedURL := normalizeURL(rawURL)

	if ts, exists := cp.seenURLs[normalizedURL]; exists {
		if time.Since(ts) < cp.config.DedupeWindow {
			return true
		}
	}
	return false
}

func normalizeURL(rawURL string) string {
	parsedURL, err := url.Parse(rawURL)
	if err != nil {
		return rawURL
	}

	parsedURL.Fragment = ""

	q := parsedURL.Query()
	noiseParams := []string{"_t", "_ts", "timestamp", "nocache", "cb", "random", "rand", "t"}
	for _, p := range noiseParams {
		q.Del(p)
	}

	keys := make([]string, 0, len(q))
	for k := range q {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	parsedURL.RawQuery = q.Encode()
	return strings.ToLower(parsedURL.String())
}

func (cp *CaptureProxy) capture(rawURL string, r *http.Request, fileType FileType, ext, contentType string, confidence int) {
	if cp.isDuplicate(rawURL) {
		return
	}

	size := int64(-1)
	if cp.config.MinFileSize > 0 || cp.config.MaxFileSize > 0 || cp.config.AutoDownload {
		size = cp.probeFileSize(rawURL, r)
	}

	if cp.config.MinFileSize > 0 && size >= 0 && size < cp.config.MinFileSize {
		return
	}
	if cp.config.MaxFileSize > 0 && size > cp.config.MaxFileSize {
		return
	}

	title := cp.extractTitle(rawURL, func() string {
		if r != nil {
			return r.Header.Get("Referer")
		}
		return ""
	}())

	item := CapturedItem{
		URL:         rawURL,
		FileType:    fileType,
		Extension:   ext,
		Size:        size,
		Title:       title,
		Timestamp:   time.Now(),
		Confidence:  confidence,
		ContentType: contentType,
	}

	if r != nil {
		item.Method = r.Method
		item.SourceURL = r.Header.Get("Referer")
	}

	cp.mu.Lock()
	normalizedURL := normalizeURL(rawURL)
	cp.seenURLs[normalizedURL] = time.Now()
	if len(cp.captured) < cp.config.MaxCaptured {
		cp.captured = append(cp.captured, item)
	}
	capturedLen := len(cp.captured)
	cp.mu.Unlock()

	cp.displayCapturedItem(item)

	if cp.config.SaveToFile != "" {
		cp.saveToFile(item)
	}

	if cp.config.AutoDownload && confidence >= cp.config.ConfidenceLevel {
		go cp.triggerDownload(item)
	}

	_ = capturedLen
}

func (cp *CaptureProxy) probeFileSize(rawURL string, r *http.Request) int64 {
	req, err := http.NewRequest("HEAD", rawURL, nil)
	if err != nil {
		return -1
	}

	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")

	if r != nil {
		if auth := r.Header.Get("Authorization"); auth != "" {
			req.Header.Set("Authorization", auth)
		}
		if cookie := r.Header.Get("Cookie"); cookie != "" {
			req.Header.Set("Cookie", cookie)
		}
	}

	for k, v := range cp.config.Headers {
		req.Header.Set(k, v)
	}
	if cp.config.Cookie != "" {
		req.Header.Set("Cookie", cp.config.Cookie)
	}

	resp, err := cp.httpClient.Do(req)
	if err != nil {
		return -1
	}
	defer resp.Body.Close()

	if resp.ContentLength > 0 {
		return resp.ContentLength
	}

	if cr := resp.Header.Get("Content-Range"); cr != "" {
		parts := strings.Split(cr, "/")
		if len(parts) == 2 {
			if s, e := strconv.ParseInt(strings.TrimSpace(parts[1]), 10, 64); e == nil {
				return s
			}
		}
	}

	return -1
}

func (cp *CaptureProxy) scanBodyForURLs(body string, r *http.Request) {
	patterns := []*regexp.Regexp{
		regexp.MustCompile(`(?i)"(?:url|src|source|file|video_url|audio_url|media_url|stream_url|download_url|href|link|path|manifest_url|hls_url|dash_url)"\s*:\s*"(https?://[^"]+)"`),
		regexp.MustCompile(`(?i)'(?:url|src|source|file)'\s*:\s*'(https?://[^']+)'`),
		regexp.MustCompile(`(?i)(?:url|src|href|source)\s*=\s*["'](https?://[^"'>\s]+)["']`),
		regexp.MustCompile(`(?i)<(?:source|video|audio|track|embed)[^>]+src=["'](https?://[^"']+)["']`),
		regexp.MustCompile(`(?i)(https?://[^\s"'<>]+\.(?:mp4|mkv|webm|m3u8|mpd|mp3|flac|wav|aac|ogg|m4a|pdf|zip|rar|7z)(?:\?[^\s"'<>]*)?)`),
	}

	seen := make(map[string]bool)
	for _, re := range patterns {
		matches := re.FindAllStringSubmatch(body, -1)
		for _, match := range matches {
			if len(match) < 2 {
				continue
			}
			foundURL := match[1]
			foundURL = strings.TrimSpace(foundURL)
			if seen[foundURL] {
				continue
			}
			seen[foundURL] = true

			if !strings.HasPrefix(foundURL, "http") {
				if strings.HasPrefix(foundURL, "//") {
					foundURL = "https:" + foundURL
				} else if strings.HasPrefix(foundURL, "/") && r != nil {
					foundURL = r.URL.Scheme + "://" + r.Host + foundURL
				} else {
					continue
				}
			}

			if _, err := url.Parse(foundURL); err != nil {
				continue
			}

			if cp.config.Verbose {
				log.Printf("\033[35m[BODY-SCAN]\033[0m Found: %s", truncate(foundURL, 80))
			}

			cp.evaluateRequest(foundURL, r, "")
		}
	}
}

func (cp *CaptureProxy) scanResponseHeaders(headers http.Header, rawURL, contentType string, contentLen int64) {
	if loc := headers.Get("Location"); loc != "" {
		if cp.config.Verbose {
			log.Printf("\033[36m[REDIRECT]\033[0m %s", truncate(loc, 80))
		}
		cp.evaluateRequest(loc, nil, "")
	}

	if cd := headers.Get("Content-Disposition"); cd != "" {
		if strings.Contains(cd, "attachment") || strings.Contains(cd, "filename=") {
			if cp.config.Verbose {
				log.Printf("\033[36m[DISPOSITION]\033[0m %s from %s", cd, truncate(rawURL, 60))
			}
			cp.evaluateRequest(rawURL, nil, contentType)
		}
	}

	for _, hdr := range []string{"X-Accel-Redirect", "X-Sendfile"} {
		if val := headers.Get(hdr); val != "" {
			if cp.config.Verbose {
				log.Printf("\033[36m[ACCEL-REDIRECT]\033[0m %s: %s", hdr, truncate(val, 80))
			}
			cp.evaluateRequest(val, nil, contentType)
		}
	}
}

func (cp *CaptureProxy) triggerDownload(item CapturedItem) {
	log.Printf("\033[36m[AUTO-DL]\033[0m Queuing: %s (%s)\n", item.Title, cp.formatSize(item.Size))

	os.MkdirAll(cp.config.OutputDir, 0755)

	oldOutDir := outDir
	oldNumThreads := numThreads
	oldMaxParallel := maxParallel
	oldVerbose := verbose
	oldRetries := retries
	oldTimeoutSec := timeoutSec
	oldEnableGzip := enableGzip

	outDir = cp.config.OutputDir
	maxParallel = 1
	verbose = cp.config.Verbose
	retries = 3
	timeoutSec = 30
	enableGzip = true

	numThreads = determineThreadsBySize(item.Size)

	for k, v := range cp.config.Headers {
		headers = append(headers, k+": "+v)
	}
	if cp.config.Cookie != "" {
		cookie = cp.config.Cookie
	}

	log.Printf("\033[33m[AUTO-DL]\033[0m threads=%d size=%s url=%s\n",
		numThreads, cp.formatSize(item.Size), truncate(item.URL, 60))

	gs := NewGlobalStatus()
	client := createHTTPClient()

	fileName := filepath.Base(strings.Split(item.URL, "?")[0])
	if fileName == "" || fileName == "/" || fileName == "." {
		fileName = fmt.Sprintf("captured_%d%s", time.Now().Unix(), item.Extension)
	}

	gs.addFile(fileName, item.Size)

	downloadSingle(item.URL, client, gs)

	cp.mu.Lock()
	for i := range cp.captured {
		if cp.captured[i].URL == item.URL {
			cp.captured[i].Downloaded = true
			cp.captured[i].DownloadPath = filepath.Join(cp.config.OutputDir, fileName)
			break
		}
	}
	cp.mu.Unlock()

	outDir = oldOutDir
	numThreads = oldNumThreads
	maxParallel = oldMaxParallel
	verbose = oldVerbose
	retries = oldRetries
	timeoutSec = oldTimeoutSec
	enableGzip = oldEnableGzip
}

func (cp *CaptureProxy) logRequest(r *http.Request) {
	logEntry := RequestLog{
		Method:      r.Method,
		URL:         r.URL.String(),
		Host:        r.Host,
		Path:        r.URL.Path,
		QueryParams: make(map[string]string),
		Headers:     make(map[string]string),
		Timestamp:   time.Now(),
	}

	for k, v := range r.URL.Query() {
		if len(v) > 0 {
			logEntry.QueryParams[k] = v[0]
		}
	}
	for k, v := range r.Header {
		if len(v) > 0 {
			logEntry.Headers[k] = v[0]
		}
	}

	cp.mu.Lock()
	cp.requestLogs = append(cp.requestLogs, logEntry)
	if len(cp.requestLogs) > 5000 {
		cp.requestLogs = cp.requestLogs[500:]
	}
	cp.mu.Unlock()
}

func (cp *CaptureProxy) logAdvanced(r *http.Request, signals []scoreSignal, totalScore int) {
	if r == nil {
		return
	}

	methodColor := "\033[36m"
	switch r.Method {
	case "GET":
		methodColor = "\033[32m"
	case "POST":
		methodColor = "\033[33m"
	case "PUT":
		methodColor = "\033[34m"
	case "DELETE":
		methodColor = "\033[31m"
	}

	fmt.Printf("\n%s[%s]\033[0m \033[1m%s\033[0m", methodColor, r.Method, r.Host)
	if r.URL.Path != "/" && r.URL.Path != "" {
		fmt.Printf("\033[90m%s\033[0m", r.URL.Path)
	}

	scoreColor := "\033[32m"
	if totalScore < 60 {
		scoreColor = "\033[33m"
	}
	if totalScore < 40 {
		scoreColor = "\033[31m"
	}
	fmt.Printf(" %s[score:%d]\033[0m", scoreColor, totalScore)

	if cp.config.Verbose && len(signals) > 0 {
		fmt.Printf("\n  \033[90mSignals:\033[0m ")
		for i, s := range signals {
			if i > 0 {
				fmt.Printf(" | ")
			}
			if s.points > 0 {
				fmt.Printf("\033[32m+%d %s\033[0m", s.points, s.reason)
			} else {
				fmt.Printf("\033[31m%d %s\033[0m", s.points, s.reason)
			}
		}
	}
	fmt.Println()
}

func (cp *CaptureProxy) logResponse(resp *http.Response) {
	if !cp.config.Verbose {
		return
	}
	statusColor := "\033[32m"
	if resp.StatusCode >= 400 {
		statusColor = "\033[31m"
	} else if resp.StatusCode >= 300 {
		statusColor = "\033[33m"
	}
	fmt.Printf("  \033[90m→\033[0m [%s%d\033[0m] \033[90m%s\033[0m",
		statusColor, resp.StatusCode, http.StatusText(resp.StatusCode))
	if ct := resp.Header.Get("Content-Type"); ct != "" {
		fmt.Printf(" \033[90m%s\033[0m", strings.Split(ct, ";")[0])
	}
	if cl := resp.Header.Get("Content-Length"); cl != "" {
		fmt.Printf(" \033[90m(%s)\033[0m", cl)
	}
	fmt.Println()
}

func (cp *CaptureProxy) displayCapturedItem(item CapturedItem) {
	color := "\033[32m"
	if item.Confidence < 60 {
		color = "\033[33m"
	}
	if item.Confidence < 40 {
		color = "\033[31m"
	}

	icon := "📁"
	switch item.FileType {
	case TypeVideo:
		icon = "🎬"
	case TypeMusic:
		icon = "🎵"
	case TypeImage:
		icon = "🖼️"
	case TypeDocument:
		icon = "📄"
	case TypeArchive:
		icon = "🗜️"
	}

	fmt.Printf("\n%s \033[36m[%s]\033[0m %s%d%%\033[0m | %s",
		icon, strings.ToUpper(string(item.FileType)),
		color, item.Confidence,
		cp.formatSize(item.Size))

	if item.Title != "" && item.Title != "unknown" {
		fmt.Printf(" | \033[33m%s\033[0m", item.Title)
	}
	if item.ContentType != "" {
		ctBase := strings.Split(item.ContentType, ";")[0]
		fmt.Printf(" | \033[90m%s\033[0m", strings.TrimSpace(ctBase))
	}
	if item.Method != "" {
		fmt.Printf(" | \033[90m%s\033[0m", item.Method)
	}
	fmt.Printf("\n  \033[90m%s\033[0m\n", item.URL)
}

func (cp *CaptureProxy) extractTitle(rawURL, referer string) string {
	parsedURL, err := url.Parse(rawURL)
	if err != nil {
		return "unknown"
	}

	pathParts := strings.Split(parsedURL.Path, "/")
	for i := len(pathParts) - 1; i >= 0; i-- {
		part := pathParts[i]
		if part == "" {
			continue
		}

		if idx := strings.LastIndex(part, "."); idx > 0 {
			part = part[:idx]
		}

		part, _ = url.PathUnescape(part)
		part = strings.ReplaceAll(part, "-", " ")
		part = strings.ReplaceAll(part, "_", " ")
		part = strings.ReplaceAll(part, "+", " ")
		part = strings.TrimSpace(part)

		if len(part) > 3 && len(part) < 200 && !isGarbageTitle(part) {
			return part
		}
	}

	q := parsedURL.Query()
	for _, key := range []string{"title", "name", "filename", "file", "n", "t"} {
		if v := q.Get(key); v != "" && len(v) > 3 && len(v) < 200 {
			return v
		}
	}

	if referer != "" {
		refParsed, err := url.Parse(referer)
		if err == nil {
			refParts := strings.Split(refParsed.Path, "/")
			for i := len(refParts) - 1; i >= 0; i-- {
				part := strings.TrimSpace(refParts[i])
				part = strings.ReplaceAll(part, "-", " ")
				part = strings.ReplaceAll(part, "_", " ")
				if len(part) > 3 && len(part) < 200 && !strings.Contains(part, ".") && !isGarbageTitle(part) {
					return part
				}
			}
		}
	}

	return "unknown"
}

func isGarbageTitle(title string) bool {
	garbageWords := []string{
		"index", "default", "main", "home", "api", "v1", "v2", "v3",
		"static", "assets", "dist", "build", "chunk", "bundle",
		"undefined", "null", "none", "unknown",
	}
	titleLower := strings.ToLower(strings.TrimSpace(title))
	for _, g := range garbageWords {
		if titleLower == g {
			return true
		}
	}

	allDigits := true
	for _, c := range titleLower {
		if c < '0' || c > '9' {
			allDigits = false
			break
		}
	}
	if allDigits && len(titleLower) > 6 {
		return true
	}

	return false
}

func (cp *CaptureProxy) saveToFile(item CapturedItem) {
	txtFile := cp.config.SaveToFile
	f, err := os.OpenFile(txtFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return
	}
	defer f.Close()

	titlePart := ""
	if item.Title != "unknown" && item.Title != "" {
		titlePart = item.Title + " | "
	}

	line := fmt.Sprintf("[%s] [%s] %s%d%% | %s | %s | %s\n",
		item.Timestamp.Format("2006-01-02 15:04:05"),
		strings.ToUpper(string(item.FileType)),
		titlePart,
		item.Confidence,
		cp.formatSize(item.Size),
		item.Method,
		item.URL)

	f.WriteString(line)

	jsonFile := strings.TrimSuffix(txtFile, filepath.Ext(txtFile)) + ".json"
	cp.appendToJSON(jsonFile, item)
}

func (cp *CaptureProxy) appendToJSON(jsonFile string, item CapturedItem) {
	var items []CapturedItem
	if data, err := os.ReadFile(jsonFile); err == nil {
		json.Unmarshal(data, &items)
	}
	items = append(items, item)
	if data, err := json.MarshalIndent(items, "", "  "); err == nil {
		os.WriteFile(jsonFile, data, 0644)
	}
}

func (cp *CaptureProxy) formatSize(size int64) string {
	if size <= 0 {
		return "unknown"
	}
	if size < 1024 {
		return fmt.Sprintf("%dB", size)
	}
	if size < 1024*1024 {
		return fmt.Sprintf("%.1fKB", float64(size)/1024)
	}
	if size < 1024*1024*1024 {
		return fmt.Sprintf("%.1fMB", float64(size)/(1024*1024))
	}
	return fmt.Sprintf("%.2fGB", float64(size)/(1024*1024*1024))
}

func (cp *CaptureProxy) GetStats() map[string]interface{} {
	cp.mu.RLock()
	defer cp.mu.RUnlock()

	stats := make(map[string]interface{})
	stats["total_captured"] = len(cp.captured)
	stats["total_requests"] = len(cp.requestLogs)
	stats["unique_urls"] = len(cp.seenURLs)

	byType := make(map[FileType]int)
	downloaded := 0
	for _, item := range cp.captured {
		byType[item.FileType]++
		if item.Downloaded {
			downloaded++
		}
	}
	stats["by_type"] = byType
	stats["downloaded"] = downloaded
	return stats
}

func (cp *CaptureProxy) GetCapturedItems() []CapturedItem {
	cp.mu.RLock()
	defer cp.mu.RUnlock()
	result := make([]CapturedItem, len(cp.captured))
	copy(result, cp.captured)
	return result
}

func (cp *CaptureProxy) GetRequestLogs() []RequestLog {
	cp.mu.RLock()
	defer cp.mu.RUnlock()
	result := make([]RequestLog, len(cp.requestLogs))
	copy(result, cp.requestLogs)
	return result
}

func (cp *CaptureProxy) ExportLogs(format, filename string) error {
	cp.mu.RLock()
	defer cp.mu.RUnlock()

	f, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer f.Close()

	switch format {
	case "json":
		enc := json.NewEncoder(f)
		enc.SetIndent("", "  ")
		return enc.Encode(cp.requestLogs)
	case "csv":
		f.WriteString("Timestamp,Method,URL,Host,Path\n")
		for _, entry := range cp.requestLogs {
			fmt.Fprintf(f, "%s,%s,%s,%s,%s\n",
				entry.Timestamp.Format("2006-01-02 15:04:05"),
				entry.Method, entry.URL, entry.Host, entry.Path)
		}
	}
	return nil
}

func (cp *CaptureProxy) Start() error {
	port := cp.config.Port
	if !strings.HasPrefix(port, ":") {
		port = ":" + port
	}

	fmt.Printf("\033[36m╔════════════════════════════════════════════════════════════════╗\033[0m\n")
	fmt.Printf("\033[36m║         SMART CAPTURE PROXY — INTELLIGENT MODE v3.0           ║\033[0m\n")
	fmt.Printf("\033[36m╚════════════════════════════════════════════════════════════════╝\033[0m\n\n")

	cleanPort := strings.TrimPrefix(port, ":")
	fmt.Printf("\033[32m✓\033[0m Proxy: \033[33mlocalhost:%s\033[0m\n", cleanPort)
	fmt.Printf("\033[32m✓\033[0m Capturing: ")
	for i, ft := range cp.config.FileTypes {
		if i > 0 {
			fmt.Printf(", ")
		}
		fmt.Printf("\033[33m%s\033[0m", ft)
	}
	fmt.Printf("\n")
	fmt.Printf("\033[32m✓\033[0m Confidence threshold: \033[33m%d%%\033[0m\n", cp.config.ConfidenceLevel)
	fmt.Printf("\033[32m✓\033[0m Dedupe window: \033[33m%s\033[0m\n", cp.config.DedupeWindow)

	if cp.config.AutoDownload {
		fmt.Printf("\033[32m✓\033[0m Auto-download: \033[33menabled\033[0m\n")
		fmt.Printf("\033[32m✓\033[0m Output dir: \033[33m%s\033[0m\n", cp.config.OutputDir)
	}

	if cp.config.SaveToFile != "" {
		fmt.Printf("\033[32m✓\033[0m Save to: \033[33m%s\033[0m\n", cp.config.SaveToFile)
	}
	if cp.config.FilterDomain != "" {
		fmt.Printf("\033[32m✓\033[0m Domain filter: \033[33m%s\033[0m\n", cp.config.FilterDomain)
	}
	if cp.config.FilterPattern != "" {
		fmt.Printf("\033[32m✓\033[0m URL pattern: \033[33m%s\033[0m\n", cp.config.FilterPattern)
	}
	if cp.config.CaptureBody {
		fmt.Printf("\033[33m⚠\033[0m Body scan: \033[33menabled\033[0m (may add latency)\n")
	}

	fmt.Printf("\n\033[90mFoxyProxy config:\033[0m HTTP Proxy → localhost:%s\n", cleanPort)
	fmt.Printf("\033[33m🎯 Waiting for traffic...\033[0m\n\n")

	return http.ListenAndServe(port, cp.proxy)
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}