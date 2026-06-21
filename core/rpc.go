package core

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

type DownloadTask struct {
	GID      string
	URLs     []string
	Status   string
	Added    time.Time
	Started  time.Time
	Finished time.Time
	Cancel   context.CancelFunc
	FileName string
	Size     int64
	Done     int64
	Error    string
}

type HistoryEntry struct {
	GID      string    `json:"gid"`
	FileName string    `json:"file_name"`
	Size     int64     `json:"size"`
	SizeHuman string   `json:"size_human"`
	URL      string    `json:"url"`
	Status   string    `json:"status"`
	Added    time.Time `json:"added"`
	Finished time.Time `json:"finished"`
	AvgSpeed string    `json:"avg_speed"`
	Duration string    `json:"duration"`
}

type BWScheduleConfig struct {
	NightFrom   string `json:"night_from"`
	NightTo     string `json:"night_to"`
	DayLimit    int64  `json:"day_limit"`
	NightLimit  int64  `json:"night_limit"`
}

type RPCServer struct {
	global     *GlobalStatus
	server     *http.Server
	mu         sync.RWMutex
	tasks      map[string]*DownloadTask
	history    []*HistoryEntry
	historyMu  sync.RWMutex
	paused     atomic.Bool
	lastBytes  int64
	lastTime   time.Time
	curSpeed   atomic.Int64
	bwSchedule *BWScheduleConfig
}

type RPCCommand struct {
	ID     interface{}            `json:"id"`
	Method string                 `json:"method"`
	Params map[string]interface{} `json:"params"`
}

type RPCResponse struct {
	ID     interface{} `json:"id"`
	Result interface{} `json:"result,omitempty"`
	Error  *RPCError   `json:"error,omitempty"`
}

type RPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}



func NewRPCServer(global *GlobalStatus) *RPCServer {
	rpc := &RPCServer{
		global:   global,
		tasks:    make(map[string]*DownloadTask),
		history:  make([]*HistoryEntry, 0),
		lastTime: time.Now(),
	}
	go rpc.speedTracker()
	go rpc.bandwidthScheduler()
	return rpc
}

func (rpc *RPCServer) speedTracker() {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for range ticker.C {
		now := time.Now()
		cur := atomic.LoadInt64(&rpc.global.totalDone)
		elapsed := now.Sub(rpc.lastTime).Seconds()
		if elapsed > 0 {
			speed := int64(float64(cur-rpc.lastBytes) / elapsed)
			if speed < 0 {
				speed = 0
			}
			rpc.curSpeed.Store(speed)
		}
		rpc.lastBytes = cur
		rpc.lastTime = now
	}
}

func (rpc *RPCServer) bandwidthScheduler() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	for range ticker.C {
		rpc.mu.RLock()
		cfg := rpc.bwSchedule
		rpc.mu.RUnlock()
		if cfg == nil {
			continue
		}
		now := time.Now()
		fromT := parseTOD(cfg.NightFrom, now)
		toT := parseTOD(cfg.NightTo, now)
		inNight := false
		if toT.Before(fromT) {
			inNight = now.After(fromT) || now.Before(toT)
		} else {
			inNight = now.After(fromT) && now.Before(toT)
		}
		if inNight && cfg.NightLimit >= 0 {
			maxSpeed = cfg.NightLimit
		} else if !inNight && cfg.DayLimit >= 0 {
			maxSpeed = cfg.DayLimit
		}
	}
}

func parseTOD(s string, ref time.Time) time.Time {
	if len(s) < 3 {
		return ref
	}
	var h, m int
	fmt.Sscanf(s, "%d:%d", &h, &m)
	return time.Date(ref.Year(), ref.Month(), ref.Day(), h, m, 0, 0, ref.Location())
}

func (gs *GlobalStatus) totalSize() int64 {
	gs.mu.RLock()
	defer gs.mu.RUnlock()
	var total int64
	for _, f := range gs.files {
		if f != nil && f.Size > 0 {
			total += f.Size
		}
	}
	return total
}

func (gs *GlobalStatus) totalDownloaded() int64 {
	return atomic.LoadInt64(&gs.totalDone)
}

func (gs *GlobalStatus) activeCount() int {
	gs.mu.RLock()
	defer gs.mu.RUnlock()
	count := 0
	for _, f := range gs.files {
		if f != nil && (f.Status == "downloading" || f.Status == "hls") {
			count++
		}
	}
	return count
}

func InitDefaults() {
	if retries == 0 {
		retries = 5
	}
	if numThreads == 0 {
		numThreads = runtime.NumCPU()
	}
	if timeoutSec == 0 {
		timeoutSec = 30
	}
	if maxParallel == 0 {
		maxParallel = 2
	}
	if outDir == "" {
		outDir = "."
	}
	if protocol == "" {
		protocol = "auto"
	}
	if ftpUser == "" {
		ftpUser = "anonymous"
	}
	if ftpPass == "" {
		ftpPass = "anonymous@example.com"
	}
	logger.SetVerbose(false)
}

func (rpc *RPCServer) Start(addr string) error {
	mux := http.NewServeMux()
	mux.HandleFunc("/jsonrpc", rpc.withCORS(rpc.handleJSONRPC))
	mux.HandleFunc("/api/status", rpc.withCORS(rpc.handleStatus))
	mux.HandleFunc("/api/files", rpc.withCORS(rpc.handleFiles))
	mux.HandleFunc("/api/tasks", rpc.withCORS(rpc.handleTasks))
	mux.HandleFunc("/api/pause", rpc.withCORS(rpc.handlePause))
	mux.HandleFunc("/api/resume", rpc.withCORS(rpc.handleResume))
	mux.HandleFunc("/api/version", rpc.withCORS(rpc.handleVersion))
	mux.HandleFunc("/api/history", rpc.withCORS(rpc.handleHistory))
	mux.HandleFunc("/api/history/clear", rpc.withCORS(rpc.handleHistoryClear))
	mux.HandleFunc("/api/meta", rpc.withCORS(rpc.handleMetadata))
	mux.HandleFunc("/api/mirror-test", rpc.withCORS(rpc.handleMirrorTest))
	mux.HandleFunc("/api/bw-schedule", rpc.withCORS(rpc.handleBWSchedule))
	mux.HandleFunc("/api/checksum", rpc.withCORS(rpc.handleChecksum))

	rpc.server = &http.Server{
		Addr:         addr,
		Handler:      mux,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 60 * time.Second,
	}

	go func() {
		if err := rpc.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logError("RPC server error: %v", err)
		}
	}()

	logInfo("HAD RPC server started on %s", addr)
	return nil
}

func (rpc *RPCServer) Stop() error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return rpc.server.Shutdown(ctx)
}

func (rpc *RPCServer) withCORS(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		h(w, r)
	}
}

func (rpc *RPCServer) handleJSONRPC(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var cmd RPCCommand
	if err := json.NewDecoder(req.Body).Decode(&cmd); err != nil {
		rpc.sendError(w, nil, -32700, "Parse error: "+err.Error())
		return
	}
	result, rpcErr := rpc.dispatch(cmd.Method, cmd.Params)
	rpc.sendResponse(w, cmd.ID, result, rpcErr)
}

func (rpc *RPCServer) dispatch(method string, params map[string]interface{}) (interface{}, *RPCError) {
	switch method {
	case "had.addUri", "had.addUrls":
		return rpc.addURI(params)
	case "had.remove":
		return rpc.removeDownload(params)
	case "had.removeAll":
		return rpc.removeAll()
	case "had.tellStatus":
		return rpc.tellStatus(params)
	case "had.tellAllStatus":
		return rpc.tellAllStatus()
	case "had.getGlobalStat":
		return rpc.getGlobalStat()
	case "had.getFiles":
		return rpc.getFiles()
	case "had.pause":
		return rpc.pauseTask(params)
	case "had.pauseAll":
		return rpc.pauseAll()
	case "had.resume":
		return rpc.resumeTask(params)
	case "had.resumeAll":
		return rpc.resumeAll()
	case "had.setSpeedLimit":
		return rpc.setSpeedLimit(params)
	case "had.getSpeedLimit":
		return rpc.getSpeedLimit()
	case "had.setMaxParallel":
		return rpc.setMaxParallel(params)
	case "had.setThreads":
		return rpc.setThreads(params)
	case "had.setOutDir":
		return rpc.setOutDir(params)
	case "had.scrape":
		return rpc.scrape(params)
	case "had.shutdown":
		return rpc.shutdown()
	case "had.version":
		return rpc.versionInfo(), nil
	case "had.getHistory":
		return rpc.getHistory()
	case "had.clearHistory":
		return rpc.clearHistory()
	case "had.setBWSchedule":
		return rpc.setBWSchedule(params)
	case "had.getBWSchedule":
		return rpc.getBWSchedule()
	case "had.testMirrors":
		return rpc.testMirrorsRPC(params)
	case "had.fetchMeta":
		return rpc.fetchMetaRPC(params)
	case "had.verifyChecksum":
		return rpc.verifyChecksumRPC(params)
	case "had.listSessions":
		return rpc.listSessions()
	case "had.resumeSession":
		return rpc.resumeSession(params)
	case "had.deleteSession":
		return rpc.deleteSession(params)
	case "had.setMirrors":
		return rpc.setMirrorsRPC(params)
	case "had.getMirrors":
		return rpc.getMirrorsRPC()
	case "had.addWebDAV":
		return rpc.addWebDAV(params)
	case "had.listWebDAV":
		return rpc.listWebDAV(params)
	case "had.downloadWebDAVFile":
		return rpc.downloadWebDAVFile(params)
	case "had.pauseFile":
		return rpc.pauseFileByName(params)
	case "had.resumeFile":
		return rpc.resumeFileByName(params)
	case "had.removeFile":
		return rpc.removeFileByName(params)
	case "had.pauseAllFiles":
		return rpc.pauseAllFilesRPC()
	case "had.getPausedFiles":
		return rpc.getPausedFilesRPC()
	case "system.listMethods":
		return rpc.listMethods(), nil
	default:
		return nil, &RPCError{Code: -32601, Message: "Method not found: " + method}
	}
}

func (rpc *RPCServer) handleStatus(w http.ResponseWriter, req *http.Request) {
	active := rpc.global.activeCount()
	totalSize := rpc.global.totalSize()
	totalDL := rpc.global.totalDownloaded()
	speed := rpc.curSpeed.Load()

	eta := ""
	if speed > 0 && totalSize > totalDL {
		secs := int((totalSize - totalDL) / speed)
		eta = formatDuration(float64(secs))
	}

	rpc.mu.RLock()
	bwSched := rpc.bwSchedule
	rpc.mu.RUnlock()

	status := map[string]interface{}{
		"status":           "running",
		"version":          "2.0.0",
		"paused":           rpc.paused.Load(),
		"active_downloads": active,
		"completed_files":  atomic.LoadInt64(&rpc.global.downloadedCount),
		"total_files":      atomic.LoadInt64(&rpc.global.totalCount),
		"total_size":       totalSize,
		"total_size_human": Size4Human(totalSize),
		"downloaded_size":  Size4Human(totalDL),
		"downloaded_bytes": totalDL,
		"speed":            speed,
		"speed_human":      Size4Human(speed) + "/s",
		"eta":              eta,
		"start_time":       rpc.global.startTime.Format(time.RFC3339),
		"uptime":           time.Since(rpc.global.startTime).Round(time.Second).String(),
		"speed_limit":      maxSpeed,
		"max_parallel":     maxParallel,
		"threads":          numThreads,
		"out_dir":          outDir,
		"bw_schedule":      bwSched,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(status)
}

func (rpc *RPCServer) handleFiles(w http.ResponseWriter, req *http.Request) {
	rpc.global.mu.RLock()
	defer rpc.global.mu.RUnlock()

	type FileInfo struct {
		Name       string  `json:"name"`
		Size       int64   `json:"size"`
		SizeHuman  string  `json:"size_human"`
		Done       int64   `json:"done"`
		DoneHuman  string  `json:"done_human"`
		Progress   float64 `json:"progress"`
		Status     string  `json:"status"`
		Threads    int     `json:"threads"`
		ActiveThr  int     `json:"active_threads"`
		DoneThr    int     `json:"done_threads"`
		ElapsedSec float64 `json:"elapsed_sec"`
		Speed      string  `json:"speed"`
		URL        string  `json:"url"`
	}

	files := make([]FileInfo, 0, len(rpc.global.files))
	for _, f := range rpc.global.files {
		if f == nil {
			continue
		}
		elapsed := time.Since(f.StartTime).Seconds()
		spd := ""
		if elapsed > 0 && f.Done > 0 {
			spd = Size4Human(int64(float64(f.Done)/elapsed)) + "/s"
		}
		files = append(files, FileInfo{
			Name:       f.Name,
			Size:       f.Size,
			SizeHuman:  Size4Human(f.Size),
			Done:       f.Done,
			DoneHuman:  Size4Human(f.Done),
			Progress:   pctOf(f.Done, f.Total),
			Status:     f.Status,
			Threads:    f.TotalThreads,
			ActiveThr:  f.ActiveThreads,
			DoneThr:    f.DoneThreads,
			ElapsedSec: elapsed,
			Speed:      spd,
			URL:        f.URL,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(files)
}

func (rpc *RPCServer) handleTasks(w http.ResponseWriter, _ *http.Request) {
	rpc.mu.RLock()
	tasks := make([]map[string]interface{}, 0, len(rpc.tasks))
	for _, t := range rpc.tasks {
		tasks = append(tasks, map[string]interface{}{
			"gid":    t.GID,
			"status": t.Status,
			"urls":   t.URLs,
		})
	}
	rpc.mu.RUnlock()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(tasks)
}

func (rpc *RPCServer) handlePause(w http.ResponseWriter, _ *http.Request) {
	rpc.paused.Store(true)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "paused"})
}

func (rpc *RPCServer) handleResume(w http.ResponseWriter, _ *http.Request) {
	rpc.paused.Store(false)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "resumed"})
}

func (rpc *RPCServer) handleVersion(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(rpc.versionInfo())
}

func (rpc *RPCServer) handleHistory(w http.ResponseWriter, req *http.Request) {
	if req.Method == http.MethodDelete {
		rpc.historyMu.Lock()
		rpc.history = make([]*HistoryEntry, 0)
		rpc.historyMu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "cleared"})
		return
	}
	rpc.historyMu.RLock()
	h := make([]*HistoryEntry, len(rpc.history))
	copy(h, rpc.history)
	rpc.historyMu.RUnlock()
	sort.Slice(h, func(i, j int) bool { return h[i].Finished.After(h[j].Finished) })
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(h)
}

func (rpc *RPCServer) handleHistoryClear(w http.ResponseWriter, req *http.Request) {
	rpc.historyMu.Lock()
	rpc.history = make([]*HistoryEntry, 0)
	rpc.historyMu.Unlock()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "cleared"})
}

func (rpc *RPCServer) handleMetadata(w http.ResponseWriter, req *http.Request) {
	rawURL := req.URL.Query().Get("url")
	if rawURL == "" {
		http.Error(w, `{"error":"missing url"}`, http.StatusBadRequest)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(buildMetaResponse(rawURL))
}

var mediaExtensions = map[string]bool{
	"mp4": true, "mkv": true, "avi": true, "mov": true, "webm": true, "ts": true, "flv": true,
	"wmv": true, "m4v": true, "mpg": true, "mpeg": true, "3gp": true,
	"mp3": true, "flac": true, "wav": true, "aac": true, "ogg": true, "opus": true, "m4a": true, "wma": true,
}

func probeMediaDuration(rawURL string) (string, float64) {
	if _, err := exec.LookPath("ffprobe"); err != nil {
		return "", 0
	}
	ctx, cancel := context.WithTimeout(context.Background(), 12*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "ffprobe", "-v", "error", "-show_entries", "format=duration",
		"-of", "default=noprint_wrappers=1:nokey=1", rawURL)
	out, err := cmd.Output()
	if err != nil {
		return "", 0
	}
	secs, err := strconv.ParseFloat(strings.TrimSpace(string(out)), 64)
	if err != nil || secs <= 0 {
		return "", 0
	}
	return formatDuration(secs), secs
}

func buildMetaResponse(rawURL string) map[string]interface{} {
	client := createHTTPClient()
	meta, err := FetchFileMetadata(rawURL, client)
	if err != nil {
		return map[string]interface{}{"error": err.Error()}
	}
	out := map[string]interface{}{
		"file_name":     meta.FileName,
		"size":          meta.Size,
		"size_human":    meta.SizeHuman,
		"extension":     meta.Extension,
		"content_type":  meta.ContentType,
		"resumable":     meta.Resumable,
		"last_modified": meta.LastMod,
	}
	if meta.Checksum != "" {
		out["checksum_hint"] = meta.Checksum
		if hash, algo := fetchChecksumFromURL(meta.Checksum, client); hash != "" {
			out["checksum_value"] = hash
			out["checksum_algo"] = algo
		}
	}
	if mediaExtensions[strings.ToLower(meta.Extension)] {
		if dur, secs := probeMediaDuration(rawURL); dur != "" {
			out["duration"] = dur
			out["duration_seconds"] = secs
		}
	}
	return out
}

func (rpc *RPCServer) handleMirrorTest(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodPost {
		http.Error(w, "POST required", http.StatusMethodNotAllowed)
		return
	}
	var body struct {
		URLs []string `json:"urls"`
	}
	if err := json.NewDecoder(req.Body).Decode(&body); err != nil || len(body.URLs) == 0 {
		http.Error(w, `{"error":"invalid body"}`, http.StatusBadRequest)
		return
	}
	client := createHTTPClient()
	results := smartMirrorTest(body.URLs, client)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(results)
}

type MirrorTestResult struct {
	URL       string        `json:"url"`
	Latency   int64         `json:"latency_ms"`
	SpeedBps  int64         `json:"speed_bps"`
	SpeedHuman string       `json:"speed_human"`
	Reachable bool          `json:"reachable"`
	Rank      int           `json:"rank"`
}

func smartMirrorTest(urls []string, client *http.Client) []MirrorTestResult {
	results := make([]MirrorTestResult, 0, len(urls))
	var mu sync.Mutex
	var wg sync.WaitGroup

	for _, u := range urls {
		wg.Add(1)
		go func(target string) {
			defer wg.Done()
			r := MirrorTestResult{URL: target}

			start := time.Now()
			req, err := http.NewRequest("GET", target, nil)
			if err != nil {
				mu.Lock()
				results = append(results, r)
				mu.Unlock()
				return
			}
			req.Header.Set("Range", "bytes=0-65535")
			ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
			req = req.WithContext(ctx)
			resp, err := client.Do(req)
			cancel()
			if err != nil || (resp.StatusCode != 200 && resp.StatusCode != 206) {
				if resp != nil {
					resp.Body.Close()
				}
				mu.Lock()
				results = append(results, r)
				mu.Unlock()
				return
			}
			buf := make([]byte, 65536)
			n, _ := resp.Body.Read(buf)
			resp.Body.Close()
			elapsed := time.Since(start)
			r.Latency = elapsed.Milliseconds()
			r.Reachable = true
			if elapsed.Seconds() > 0 && n > 0 {
				r.SpeedBps = int64(float64(n) / elapsed.Seconds())
				r.SpeedHuman = Size4Human(r.SpeedBps) + "/s"
			}
			mu.Lock()
			results = append(results, r)
			mu.Unlock()
		}(u)
	}
	wg.Wait()

	sort.Slice(results, func(i, j int) bool {
		if results[i].Reachable != results[j].Reachable {
			return results[i].Reachable
		}
		return results[i].SpeedBps > results[j].SpeedBps
	})
	for i := range results {
		results[i].Rank = i + 1
	}
	return results
}

func (rpc *RPCServer) handleBWSchedule(w http.ResponseWriter, req *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if req.Method == http.MethodGet {
		rpc.mu.RLock()
		cfg := rpc.bwSchedule
		rpc.mu.RUnlock()
		json.NewEncoder(w).Encode(cfg)
		return
	}
	if req.Method == http.MethodPost {
		var cfg BWScheduleConfig
		if err := json.NewDecoder(req.Body).Decode(&cfg); err != nil {
			http.Error(w, `{"error":"invalid body"}`, http.StatusBadRequest)
			return
		}
		rpc.mu.Lock()
		rpc.bwSchedule = &cfg
		rpc.mu.Unlock()
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
		return
	}
	if req.Method == http.MethodDelete {
		rpc.mu.Lock()
		rpc.bwSchedule = nil
		rpc.mu.Unlock()
		json.NewEncoder(w).Encode(map[string]string{"status": "cleared"})
		return
	}
	http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
}

func (rpc *RPCServer) handleChecksum(w http.ResponseWriter, req *http.Request) {
	fileName := req.URL.Query().Get("file")
	algo := req.URL.Query().Get("algo")
	if fileName == "" {
		http.Error(w, `{"error":"missing file"}`, http.StatusBadRequest)
		return
	}
	if algo == "" {
		algo = "sha256"
	}
	path := filepath.Join(outDir, fileName)
	if _, err := os.Stat(path); os.IsNotExist(err) {
		http.Error(w, `{"error":"file not found"}`, http.StatusNotFound)
		return
	}
	hash, err := computeFileHash(path, algo)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"file":      fileName,
		"algorithm": algo,
		"hash":      hash,
	})
}

func (rpc *RPCServer) addURI(params map[string]interface{}) (interface{}, *RPCError) {
	var urls []string
	
	// First check "urls" (standard key)
	switch v := params["urls"].(type) {
	case []interface{}:
		for _, u := range v {
			if s, ok := u.(string); ok && s != "" {
				urls = append(urls, s)
			}
		}
	case string:
		if v != "" {
			urls = append(urls, v)
		}
	}
	
	// Also check "uris" (used by app.js)
	if len(urls) == 0 {
		switch v := params["uris"].(type) {
		case []interface{}:
			for _, u := range v {
				if s, ok := u.(string); ok && s != "" {
					urls = append(urls, s)
				}
			}
		case string:
			if v != "" {
				urls = append(urls, v)
			}
		}
	}
	
	// Also check "uri" (singular)
	if len(urls) == 0 {
		if u, ok := params["uri"].(string); ok && u != "" {
			urls = append(urls, u)
		}
	}
	
	if len(urls) == 0 {
		return nil, &RPCError{Code: -32602, Message: "Invalid params: no urls provided"}
	}

	gid := generateGID()
	ctx, cancel := context.WithCancel(context.Background())
	task := &DownloadTask{
		GID:    gid,
		URLs:   urls,
		Status: "active",
		Added:  time.Now(),
		Cancel: cancel,
	}

	rpc.mu.Lock()
	rpc.tasks[gid] = task
	rpc.mu.Unlock()

	go func() {
		task.Started = time.Now()
		client := createHTTPClient()
		var wg sync.WaitGroup
		sem := make(chan struct{}, maxParallel)

		for _, u := range urls {
			select {
			case <-ctx.Done():
				return
			default:
			}
			wg.Add(1)
			sem <- struct{}{}
			go func(rawURL string) {
				defer wg.Done()
				defer func() { <-sem }()
				if isMagnetLink(rawURL) {
					info, _ := parseMagnetLink(rawURL)
					name := "magnet"
					if info != nil && info.DisplayName != "" {
						name = info.DisplayName
					}
					task.FileName = name
					downloadMagnet(rawURL, rpc.global)
					return
				}
				if isFTPURL(rawURL) {
					name := filepath.Base(rawURL)
					task.FileName = name
					downloadFTP(rawURL, rpc.global)
					return
				}
				if strings.HasPrefix(rawURL, "sftp://") {
					name := filepath.Base(rawURL)
					task.FileName = name
					downloadSFTP(rawURL, rpc.global)
					return
				}
				if isHLSURL(rawURL) {
					os.MkdirAll(outDir, 0755)
					name := getFileName(rawURL, nil)
					outPath := filepath.Join(outDir, name)
					notifier := NewNotifier()
					rpc.global.addFile(name, -1)
					task.FileName = name
					hls := NewHLSDownloader(rawURL, outPath, name, client, rpc.global, notifier)
					if err := hls.Download(); err != nil {
						logError("HLS: %v", err)
						rpc.global.markError(name)
					}
					return
				}
				name, size, err := fetchFileInfo(rawURL, client)
				if err != nil || size <= 0 {
					name = filepath.Base(rawURL)
					size = 0
				}
				rpc.global.addFile(name, size)
				task.FileName = name
				task.Size = size
				downloadSingleFromURL(rawURL, client, rpc.global, size, name)
			}(u)
		}
		wg.Wait()

		rpc.mu.Lock()
		task.Status = "complete"
		task.Finished = time.Now()
		rpc.mu.Unlock()

		rpc.global.mu.RLock()
		for _, f := range rpc.global.files {
			if f != nil && f.Name == task.FileName {
				elapsed := f.EndTime.Sub(f.StartTime)
				avgSpd := ""
				if elapsed.Seconds() > 0 && f.Size > 0 {
					avgSpd = Size4Human(int64(float64(f.Size)/elapsed.Seconds())) + "/s"
				}
				entry := &HistoryEntry{
					GID:       gid,
					FileName:  f.Name,
					Size:      f.Size,
					SizeHuman: Size4Human(f.Size),
					URL:       f.URL,
					Status:    f.Status,
					Added:     task.Added,
					Finished:  task.Finished,
					AvgSpeed:  avgSpd,
					Duration:  elapsed.Round(time.Second).String(),
				}
				rpc.historyMu.Lock()
				rpc.history = append(rpc.history, entry)
				if len(rpc.history) > 500 {
					rpc.history = rpc.history[len(rpc.history)-500:]
				}
				rpc.historyMu.Unlock()
				break
			}
		}
		rpc.global.mu.RUnlock()
		logInfo("download complete: %s", task.FileName)
	}()

	return map[string]string{"gid": gid}, nil
}

func (rpc *RPCServer) removeDownload(params map[string]interface{}) (interface{}, *RPCError) {
	gid, ok := params["gid"].(string)
	if !ok || gid == "" {
		return nil, &RPCError{Code: -32602, Message: "Invalid params: missing gid"}
	}

	rpc.mu.Lock()
	task, exists := rpc.tasks[gid]
	if exists {
		if task.Cancel != nil {
			task.Cancel()
		}
		delete(rpc.tasks, gid)
	}
	rpc.mu.Unlock()

	if !exists {
		return nil, &RPCError{Code: 1, Message: "GID not found: " + gid}
	}

	if task.FileName != "" {
		rpc.global.RemoveFile(task.FileName)
	}
	return map[string]string{"gid": gid}, nil
}

func (rpc *RPCServer) removeAll() (interface{}, *RPCError) {
	rpc.mu.Lock()
	count := len(rpc.tasks)
	for _, t := range rpc.tasks {
		if t.Cancel != nil {
			t.Cancel()
		}
	}
	rpc.tasks = make(map[string]*DownloadTask)
	rpc.mu.Unlock()

	rpc.global.mu.Lock()
	for _, f := range rpc.global.files {
		if f != nil && f.ctrl != nil && f.ctrl.pause != nil {
			f.ctrl.pause()
		}
	}
	rpc.global.files = make([]*FileStatus, 0)
	atomic.StoreInt64(&rpc.global.downloadedCount, 0)
	atomic.StoreInt64(&rpc.global.totalCount, 0)
	atomic.StoreInt64(&rpc.global.totalDone, 0)
	rpc.global.mu.Unlock()

	return map[string]interface{}{"removed": count}, nil
}

func (rpc *RPCServer) tellStatus(params map[string]interface{}) (interface{}, *RPCError) {
	gid, ok := params["gid"].(string)
	if !ok || gid == "" {
		return nil, &RPCError{Code: -32602, Message: "Invalid params: missing gid"}
	}
	rpc.mu.RLock()
	task, exists := rpc.tasks[gid]
	rpc.mu.RUnlock()
	if !exists {
		return nil, &RPCError{Code: 1, Message: "GID not found: " + gid}
	}
	return map[string]interface{}{
		"gid":       task.GID,
		"status":    task.Status,
		"urls":      task.URLs,
		"file_name": task.FileName,
		"size":      task.Size,
		"done":      task.Done,
	}, nil
}

func (rpc *RPCServer) tellAllStatus() (interface{}, *RPCError) {
	rpc.global.mu.RLock()
	files := make([]map[string]interface{}, 0, len(rpc.global.files))
	for _, f := range rpc.global.files {
		if f == nil {
			continue
		}
		files = append(files, map[string]interface{}{
			"name":       f.Name,
			"size":       f.Size,
			"downloaded": f.Done,
			"progress":   pctOf(f.Done, f.Total),
			"status":     f.Status,
			"url":        f.URL,
		})
	}
	rpc.global.mu.RUnlock()

	rpc.mu.RLock()
	tasks := make([]map[string]interface{}, 0, len(rpc.tasks))
	for _, t := range rpc.tasks {
		tasks = append(tasks, map[string]interface{}{
			"gid":    t.GID,
			"status": t.Status,
			"urls":   t.URLs,
		})
	}
	rpc.mu.RUnlock()

	return map[string]interface{}{
		"files": files,
		"tasks": tasks,
		"count": len(files),
	}, nil
}

func (rpc *RPCServer) getGlobalStat() (interface{}, *RPCError) {
	totalSize := rpc.global.totalSize()
	totalDL := rpc.global.totalDownloaded()
	speed := rpc.curSpeed.Load()

	eta := ""
	if speed > 0 && totalSize > totalDL {
		secs := int((totalSize - totalDL) / speed)
		eta = formatDuration(float64(secs))
	}

	return map[string]interface{}{
		"num_active":             rpc.global.activeCount(),
		"num_stopped":            atomic.LoadInt64(&rpc.global.downloadedCount),
		"total_files":            atomic.LoadInt64(&rpc.global.totalCount),
		"completed_files":        atomic.LoadInt64(&rpc.global.downloadedCount),
		"total_size":             totalSize,
		"total_size_human":       Size4Human(totalSize),
		"total_downloaded":       totalDL,
		"total_downloaded_human": Size4Human(totalDL),
		"total_progress":         pctOf(totalDL, totalSize),
		"speed":                  speed,
		"speed_human":            Size4Human(speed) + "/s",
		"eta":                    eta,
		"paused":                 rpc.paused.Load(),
		"speed_limit":            maxSpeed,
		"uptime":                 time.Since(rpc.global.startTime).Round(time.Second).String(),
	}, nil
}

func (rpc *RPCServer) getFiles() (interface{}, *RPCError) {
	rpc.global.mu.RLock()
	defer rpc.global.mu.RUnlock()

	files := make([]map[string]interface{}, 0, len(rpc.global.files))
	for _, f := range rpc.global.files {
		if f == nil {
			continue
		}
		files = append(files, map[string]interface{}{
			"name":       f.Name,
			"size":       f.Size,
			"downloaded": f.Done,
			"progress":   pctOf(f.Done, f.Total),
			"status":     f.Status,
			"url":        f.URL,
		})
	}
	return files, nil
}

func (rpc *RPCServer) getHistory() (interface{}, *RPCError) {
	rpc.historyMu.RLock()
	h := make([]*HistoryEntry, len(rpc.history))
	copy(h, rpc.history)
	rpc.historyMu.RUnlock()
	sort.Slice(h, func(i, j int) bool { return h[i].Finished.After(h[j].Finished) })
	return h, nil
}

func (rpc *RPCServer) clearHistory() (interface{}, *RPCError) {
	rpc.historyMu.Lock()
	rpc.history = make([]*HistoryEntry, 0)
	rpc.historyMu.Unlock()
	return map[string]string{"status": "cleared"}, nil
}

func (rpc *RPCServer) setBWSchedule(params map[string]interface{}) (interface{}, *RPCError) {
	cfg := &BWScheduleConfig{}
	if v, ok := params["night_from"].(string); ok {
		cfg.NightFrom = v
	}
	if v, ok := params["night_to"].(string); ok {
		cfg.NightTo = v
	}
	if v, ok := toInt64(params["day_limit"]); ok {
		cfg.DayLimit = v
	} else {
		cfg.DayLimit = -1
	}
	if v, ok := toInt64(params["night_limit"]); ok {
		cfg.NightLimit = v
	} else {
		cfg.NightLimit = -1
	}
	rpc.mu.Lock()
	rpc.bwSchedule = cfg
	rpc.mu.Unlock()
	return map[string]string{"status": "ok"}, nil
}

func (rpc *RPCServer) getBWSchedule() (interface{}, *RPCError) {
	rpc.mu.RLock()
	cfg := rpc.bwSchedule
	rpc.mu.RUnlock()
	return cfg, nil
}

func (rpc *RPCServer) testMirrorsRPC(params map[string]interface{}) (interface{}, *RPCError) {
	var urls []string
	if v, ok := params["urls"].([]interface{}); ok {
		for _, u := range v {
			if s, ok := u.(string); ok {
				urls = append(urls, s)
			}
		}
	}
	if len(urls) == 0 {
		return nil, &RPCError{Code: -32602, Message: "urls required"}
	}
	client := createHTTPClient()
	return smartMirrorTest(urls, client), nil
}

func (rpc *RPCServer) fetchMetaRPC(params map[string]interface{}) (interface{}, *RPCError) {
	rawURL, ok := params["url"].(string)
	if !ok || rawURL == "" {
		return nil, &RPCError{Code: -32602, Message: "url required"}
	}
	resp := buildMetaResponse(rawURL)
	if errMsg, ok := resp["error"].(string); ok {
		return nil, &RPCError{Code: 1, Message: errMsg}
	}
	return resp, nil
}

func (rpc *RPCServer) verifyChecksumRPC(params map[string]interface{}) (interface{}, *RPCError) {
	fileName, ok := params["file"].(string)
	if !ok || fileName == "" {
		return nil, &RPCError{Code: -32602, Message: "file required"}
	}
	algo, _ := params["algo"].(string)
	if algo == "" {
		algo = "sha256"
	}
	path := filepath.Join(outDir, fileName)
	hash, err := computeFileHash(path, algo)
	if err != nil {
		return nil, &RPCError{Code: 1, Message: err.Error()}
	}
	result := map[string]string{
		"file":      fileName,
		"algorithm": algo,
		"hash":      hash,
	}
	if expected, ok := params["expected"].(string); ok && expected != "" {
		result["match"] = fmt.Sprintf("%v", strings.EqualFold(hash, expected))
	}
	return result, nil
}

type SessionInfo struct {
	File            string    `json:"file"`
	FileName        string    `json:"file_name"`
	URL             string    `json:"url"`
	Size            int64     `json:"size"`
	SizeHuman       string    `json:"size_human"`
	Downloaded      int64     `json:"downloaded"`
	DownloadedHuman string    `json:"downloaded_human"`
	Progress        float64   `json:"progress"`
	Mirrors         int       `json:"mirrors"`
	Checksum        string    `json:"checksum,omitempty"`
	Algorithm       string    `json:"algorithm,omitempty"`
	UpdatedAt       time.Time `json:"updated_at"`
}

func validateSessionPath(file string) error {
	if !strings.HasSuffix(file, ".had") {
		return fmt.Errorf("not a session file")
	}
	root := outDir
	if root == "" {
		root = "."
	}
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return err
	}
	absFile, err := filepath.Abs(file)
	if err != nil {
		return err
	}
	if !strings.HasPrefix(absFile, absRoot) {
		return fmt.Errorf("path outside output directory")
	}
	return nil
}

func (rpc *RPCServer) listSessions() (interface{}, *RPCError) {
	sessions := make([]SessionInfo, 0)
	root := outDir
	if root == "" {
		root = "."
	}
	filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil || info == nil || info.IsDir() {
			return nil
		}
		if !strings.HasSuffix(path, ".had") {
			return nil
		}
		data, rerr := os.ReadFile(path)
		if rerr != nil {
			return nil
		}
		var s Session
		if json.Unmarshal(data, &s) != nil {
			return nil
		}
		var done int64
		for _, p := range s.Progress {
			done += p
		}
		fileName := s.FileName
		if fileName == "" {
			fileName = filepath.Base(s.Path)
		}
		sessions = append(sessions, SessionInfo{
			File:            path,
			FileName:        fileName,
			URL:             s.URL,
			Size:            s.Size,
			SizeHuman:       Size4Human(s.Size),
			Downloaded:      done,
			DownloadedHuman: Size4Human(done),
			Progress:        pctOf(done, s.Size),
			Mirrors:         len(s.Mirrors),
			Checksum:        s.Checksum,
			Algorithm:       s.Algorithm,
			UpdatedAt:       s.UpdatedAt,
		})
		return nil
	})
	sort.Slice(sessions, func(i, j int) bool { return sessions[i].UpdatedAt.After(sessions[j].UpdatedAt) })
	return sessions, nil
}

func (rpc *RPCServer) resumeSession(params map[string]interface{}) (interface{}, *RPCError) {
	file, ok := params["file"].(string)
	if !ok || file == "" {
		return nil, &RPCError{Code: -32602, Message: "file required"}
	}
	if err := validateSessionPath(file); err != nil {
		return nil, &RPCError{Code: 1, Message: err.Error()}
	}
	data, err := os.ReadFile(file)
	if err != nil {
		return nil, &RPCError{Code: 1, Message: "cannot read session: " + err.Error()}
	}
	var s Session
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, &RPCError{Code: 1, Message: "cannot parse session: " + err.Error()}
	}
	if s.URL == "" || s.Path == "" {
		return nil, &RPCError{Code: 1, Message: "invalid session file"}
	}
	fileName := s.FileName
	if fileName == "" {
		fileName = filepath.Base(s.Path)
	}
	if len(s.Ranges) == 0 {
		s.Ranges = [][2]int64{{0, s.Size - 1}}
	}
	if len(s.Progress) != len(s.Ranges) {
		s.Progress = make([]int64, len(s.Ranges))
	}
	fout, err := os.OpenFile(s.Path, os.O_WRONLY|os.O_CREATE, 0644)
	if err != nil {
		return nil, &RPCError{Code: 1, Message: "cannot open output: " + err.Error()}
	}

	gid := generateGID()
	_, cancel := context.WithCancel(context.Background())
	task := &DownloadTask{
		GID: gid, URLs: []string{s.URL}, Status: "active",
		Added: time.Now(), Started: time.Now(), Cancel: cancel,
		FileName: fileName, Size: s.Size,
	}
	rpc.mu.Lock()
	rpc.tasks[gid] = task
	rpc.mu.Unlock()

	rpc.global.addFile(fileName, s.Size)

	go func() {
		defer fout.Close()
		client := createHTTPClient()
		segs := resumeSegments(s.Ranges, s.Progress)
		dl := newDownloader(s.URL, s.Path, fileName, s.Size, segs, client, fout, rpc.global)
		applyCommonHeaders(dl, s.URL)
		if len(s.Mirrors) > 1 {
			dl.setMirrors(s.Mirrors)
		}
		if s.Checksum != "" {
			dl.autoChecksum = s.Checksum
			dl.checksumAlgo = s.Algorithm
		}
		dl.Run()
		os.Remove(file)

		rpc.mu.Lock()
		task.Status = "complete"
		task.Finished = time.Now()
		rpc.mu.Unlock()

		rpc.global.mu.RLock()
		for _, f := range rpc.global.files {
			if f != nil && f.Name == fileName {
				elapsed := f.EndTime.Sub(f.StartTime)
				avgSpd := ""
				if elapsed.Seconds() > 0 && f.Size > 0 {
					avgSpd = Size4Human(int64(float64(f.Size)/elapsed.Seconds())) + "/s"
				}
				entry := &HistoryEntry{
					GID: gid, FileName: f.Name, Size: f.Size, SizeHuman: Size4Human(f.Size),
					URL: f.URL, Status: f.Status, Added: task.Added, Finished: task.Finished,
					AvgSpeed: avgSpd, Duration: elapsed.Round(time.Second).String(),
				}
				rpc.historyMu.Lock()
				rpc.history = append(rpc.history, entry)
				if len(rpc.history) > 500 {
					rpc.history = rpc.history[len(rpc.history)-500:]
				}
				rpc.historyMu.Unlock()
				break
			}
		}
		rpc.global.mu.RUnlock()
		logInfo("session resumed and complete: %s", fileName)
	}()

	return map[string]string{"gid": gid, "file_name": fileName}, nil
}

func (rpc *RPCServer) deleteSession(params map[string]interface{}) (interface{}, *RPCError) {
	file, ok := params["file"].(string)
	if !ok || file == "" {
		return nil, &RPCError{Code: -32602, Message: "file required"}
	}
	if err := validateSessionPath(file); err != nil {
		return nil, &RPCError{Code: 1, Message: err.Error()}
	}
	if err := os.Remove(file); err != nil {
		return nil, &RPCError{Code: 1, Message: err.Error()}
	}
	return map[string]string{"status": "deleted"}, nil
}

func (rpc *RPCServer) setMirrorsRPC(params map[string]interface{}) (interface{}, *RPCError) {
	var urls []string
	if v, ok := params["urls"].([]interface{}); ok {
		for _, u := range v {
			if s, ok := u.(string); ok && s != "" {
				urls = append(urls, s)
			}
		}
	}
	rpc.mu.Lock()
	mirrorURLs = strings.Join(urls, ",")
	autoMirror = len(urls) > 0
	rpc.mu.Unlock()
	return map[string]interface{}{"mirrors": urls, "auto_mirror": autoMirror}, nil
}

func (rpc *RPCServer) getMirrorsRPC() (interface{}, *RPCError) {
	rpc.mu.RLock()
	defer rpc.mu.RUnlock()
	var urls []string
	if mirrorURLs != "" {
		urls = strings.Split(mirrorURLs, ",")
	}
	return map[string]interface{}{"mirrors": urls, "auto_mirror": autoMirror}, nil
}

func (rpc *RPCServer) addWebDAV(params map[string]interface{}) (interface{}, *RPCError) {
	base, ok := params["url"].(string)
	if !ok || base == "" {
		return nil, &RPCError{Code: -32602, Message: "url required"}
	}
	user, _ := params["user"].(string)
	pass, _ := params["pass"].(string)
	path, _ := params["path"].(string)
	if path == "" {
		path = "/"
	}
	client := createHTTPClient()
	wd := NewWebDAVDownloader(base, user, pass, client, rpc.global)
	gid := generateGID()
	go wd.DownloadAll(path)
	return map[string]string{"gid": gid, "status": "downloading"}, nil
}

func (rpc *RPCServer) listWebDAV(params map[string]interface{}) (interface{}, *RPCError) {
	base, ok := params["url"].(string)
	if !ok || base == "" {
		return nil, &RPCError{Code: -32602, Message: "url required"}
	}
	user, _ := params["user"].(string)
	pass, _ := params["pass"].(string)
	path, _ := params["path"].(string)
	if path == "" {
		path = "/"
	}
	client := createHTTPClient()
	wd := NewWebDAVDownloader(base, user, pass, client, rpc.global)
	items, err := wd.List(path)
	if err != nil {
		return nil, &RPCError{Code: 1, Message: err.Error()}
	}
	out := make([]map[string]interface{}, 0, len(items))
	for _, it := range items {
		trimmed := strings.TrimSuffix(it.Href, "/")
		name := filepath.Base(trimmed)
		isDir := strings.HasSuffix(it.Href, "/")
		out = append(out, map[string]interface{}{
			"name":          name,
			"href":          it.Href,
			"size":          it.Propstat.Prop.ContentLength,
			"size_human":    Size4Human(it.Propstat.Prop.ContentLength),
			"is_dir":        isDir,
			"content_type":  it.Propstat.Prop.ContentType,
			"last_modified": it.Propstat.Prop.LastModified,
		})
	}
	return out, nil
}

func (rpc *RPCServer) downloadWebDAVFile(params map[string]interface{}) (interface{}, *RPCError) {
	base, ok := params["url"].(string)
	if !ok || base == "" {
		return nil, &RPCError{Code: -32602, Message: "url required"}
	}
	href, ok := params["href"].(string)
	if !ok || href == "" {
		return nil, &RPCError{Code: -32602, Message: "href required"}
	}
	name, _ := params["name"].(string)
	user, _ := params["user"].(string)
	pass, _ := params["pass"].(string)
	size, _ := toInt64(params["size"])
	if name == "" {
		name = filepath.Base(strings.TrimSuffix(href, "/"))
	}
	client := createHTTPClient()
	wd := NewWebDAVDownloader(base, user, pass, client, rpc.global)
	go wd.DownloadFile(href, name, size)
	return map[string]string{"status": "downloading", "file_name": name}, nil
}

func (rpc *RPCServer) pauseTask(params map[string]interface{}) (interface{}, *RPCError) {
	gid, ok := params["gid"].(string)
	if !ok || gid == "" {
		return nil, &RPCError{Code: -32602, Message: "Invalid params: missing gid"}
	}

	rpc.mu.Lock()
	task, exists := rpc.tasks[gid]
	if exists && task.Status == "active" && task.Cancel != nil {
		task.Cancel()
		task.Status = "paused"
	}
	rpc.mu.Unlock()

	if !exists {
		return nil, &RPCError{Code: 1, Message: "GID not found: " + gid}
	}
	if task.FileName != "" {
		rpc.global.PauseFile(task.FileName)
	}
	return map[string]string{"gid": gid}, nil
}

func (rpc *RPCServer) pauseAll() (interface{}, *RPCError) {
	rpc.paused.Store(true)
	rpc.mu.Lock()
	count := 0
	for _, t := range rpc.tasks {
		if t.Status == "active" && t.Cancel != nil {
			t.Cancel()
			t.Status = "paused"
			count++
		}
	}
	rpc.mu.Unlock()
	rpc.global.PauseAllFiles()
	return map[string]interface{}{"paused": count}, nil
}

func (rpc *RPCServer) resumeTask(params map[string]interface{}) (interface{}, *RPCError) {
	gid, ok := params["gid"].(string)
	if !ok || gid == "" {
		return nil, &RPCError{Code: -32602, Message: "Invalid params: missing gid"}
	}

	rpc.mu.RLock()
	task, exists := rpc.tasks[gid]
	rpc.mu.RUnlock()

	if !exists {
		return nil, &RPCError{Code: 1, Message: "GID not found: " + gid}
	}
	if task.Status != "paused" {
		return map[string]string{"gid": gid, "note": "not paused"}, nil
	}

	ctx, cancel := context.WithCancel(context.Background())
	rpc.mu.Lock()
	task.Cancel = cancel
	task.Status = "active"
	rpc.mu.Unlock()

	go func() {
		client := createHTTPClient()
		var wg sync.WaitGroup
		sem := make(chan struct{}, maxParallel)
		for _, u := range task.URLs {
			select {
			case <-ctx.Done():
				return
			default:
			}
			wg.Add(1)
			sem <- struct{}{}
			go func(rawURL string) {
				defer wg.Done()
				defer func() { <-sem }()
				if isMagnetLink(rawURL) {
					downloadMagnet(rawURL, rpc.global)
					return
				}
				if isFTPURL(rawURL) {
					downloadFTP(rawURL, rpc.global)
					return
				}
				if strings.HasPrefix(rawURL, "sftp://") {
					downloadSFTP(rawURL, rpc.global)
					return
				}
				if isHLSURL(rawURL) {
					os.MkdirAll(outDir, 0755)
					name := getFileName(rawURL, nil)
					outPath := filepath.Join(outDir, name)
					notifier := NewNotifier()
					hls := NewHLSDownloader(rawURL, outPath, name, client, rpc.global, notifier)
					if err := hls.Download(); err != nil {
						logError("HLS: %v", err)
						rpc.global.markError(name)
					}
					return
				}
				name, size, err := fetchFileInfo(rawURL, client)
				if err != nil || size <= 0 {
					name = filepath.Base(rawURL)
					size = 0
				}
				downloadSingleFromURL(rawURL, client, rpc.global, size, name)
			}(u)
		}
		wg.Wait()
		rpc.mu.Lock()
		task.Status = "complete"
		task.Finished = time.Now()
		rpc.mu.Unlock()
	}()

	return map[string]string{"gid": gid}, nil
}

func (rpc *RPCServer) resumeAll() (interface{}, *RPCError) {
	rpc.paused.Store(false)
	return map[string]interface{}{"status": "global pause lifted"}, nil
}

func (rpc *RPCServer) pauseFileByName(params map[string]interface{}) (interface{}, *RPCError) {
	name, ok := params["name"].(string)
	if !ok || name == "" {
		return nil, &RPCError{Code: -32602, Message: "Invalid params: missing name"}
	}
	if err := rpc.global.PauseFile(name); err != nil {
		return nil, &RPCError{Code: 1, Message: err.Error()}
	}
	return map[string]string{"name": name, "status": "paused"}, nil
}

func (rpc *RPCServer) resumeFileByName(params map[string]interface{}) (interface{}, *RPCError) {
	name, ok := params["name"].(string)
	if !ok || name == "" {
		return nil, &RPCError{Code: -32602, Message: "Invalid params: missing name"}
	}
	rawURL, size, err := rpc.global.ResumeFile(name)
	if err != nil {
		return nil, &RPCError{Code: 1, Message: err.Error()}
	}
	client := createHTTPClient()
	go downloadSingleFromURL(rawURL, client, rpc.global, size, name)
	return map[string]string{"name": name, "status": "resuming"}, nil
}

func (rpc *RPCServer) removeFileByName(params map[string]interface{}) (interface{}, *RPCError) {
	name, ok := params["name"].(string)
	if !ok || name == "" {
		return nil, &RPCError{Code: -32602, Message: "Invalid params: missing name"}
	}
	if err := rpc.global.RemoveFile(name); err != nil {
		return nil, &RPCError{Code: 1, Message: err.Error()}
	}
	return map[string]string{"name": name, "status": "removed"}, nil
}

func (rpc *RPCServer) pauseAllFilesRPC() (interface{}, *RPCError) {
	n := rpc.global.PauseAllFiles()
	return map[string]interface{}{"paused": n}, nil
}

func (rpc *RPCServer) getPausedFilesRPC() (interface{}, *RPCError) {
	files := rpc.global.PausedFiles()
	out := make([]map[string]interface{}, 0, len(files))
	for _, f := range files {
		out = append(out, map[string]interface{}{
			"name":       f.Name,
			"url":        f.URL,
			"size":       f.Size,
			"size_human": Size4Human(f.Size),
			"done":       f.Done,
			"done_human": Size4Human(f.Done),
			"progress":   pctOf(f.Done, f.Total),
		})
	}
	return out, nil
}

func (rpc *RPCServer) setSpeedLimit(params map[string]interface{}) (interface{}, *RPCError) {
	val, ok := params["speed"]
	if !ok {
		return nil, &RPCError{Code: -32602, Message: "Invalid params: missing speed (bytes/sec)"}
	}
	speed, ok := toInt64(val)
	if !ok || speed < 0 {
		return nil, &RPCError{Code: -32602, Message: "speed must be a non-negative integer"}
	}
	maxSpeed = speed
	return map[string]interface{}{"speed_limit": speed, "speed_human": Size4Human(speed) + "/s"}, nil
}

func (rpc *RPCServer) getSpeedLimit() (interface{}, *RPCError) {
	return map[string]interface{}{
		"speed_limit":       maxSpeed,
		"speed_limit_human": Size4Human(maxSpeed) + "/s",
	}, nil
}

func (rpc *RPCServer) setMaxParallel(params map[string]interface{}) (interface{}, *RPCError) {
	val, ok := params["max"]
	if !ok {
		return nil, &RPCError{Code: -32602, Message: "Invalid params: missing max"}
	}
	n, ok := toInt64(val)
	if !ok || n < 1 {
		return nil, &RPCError{Code: -32602, Message: "max must be >= 1"}
	}
	maxParallel = int(n)
	return map[string]interface{}{"max_parallel": maxParallel}, nil
}

func (rpc *RPCServer) setThreads(params map[string]interface{}) (interface{}, *RPCError) {
	val, ok := params["threads"]
	if !ok {
		return nil, &RPCError{Code: -32602, Message: "Invalid params: missing threads"}
	}
	n, ok := toInt64(val)
	if !ok || n < 1 {
		return nil, &RPCError{Code: -32602, Message: "threads must be >= 1"}
	}
	numThreads = int(n)
	return map[string]interface{}{"threads": numThreads}, nil
}

func (rpc *RPCServer) setOutDir(params map[string]interface{}) (interface{}, *RPCError) {
	dir, ok := params["dir"].(string)
	if !ok || dir == "" {
		return nil, &RPCError{Code: -32602, Message: "Invalid params: missing dir"}
	}
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, &RPCError{Code: 1, Message: "cannot create dir: " + err.Error()}
	}
	outDir = dir
	return map[string]interface{}{"out_dir": outDir}, nil
}

func (rpc *RPCServer) scrape(params map[string]interface{}) (interface{}, *RPCError) {
	u, ok := params["url"].(string)
	if !ok || u == "" {
		return nil, &RPCError{Code: -32602, Message: "Invalid params: missing url"}
	}
	gid := generateGID()
	go scrapeAndDownload(u, rpc.global)
	return map[string]string{"gid": gid, "url": u, "status": "scraping"}, nil
}

func (rpc *RPCServer) shutdown() (interface{}, *RPCError) {
	go func() {
		time.Sleep(300 * time.Millisecond)
		os.Exit(0)
	}()
	return map[string]string{"status": "shutting down"}, nil
}

func (rpc *RPCServer) versionInfo() interface{} {
	return map[string]string{
		"name":     "HAD (Hyper Advanced Downloader)",
		"version":  "3.6.0",
		"protocol": "had-rpc/2.0",
		"features": "http,https,ftp,ftps,sftp,hls,metalink,magnet,webdav,scrape,capture-proxy,smart-mirror,bw-schedule,checksum,media-meta,sessions,history",
	}
}

func (rpc *RPCServer) listMethods() interface{} {
	return []string{
		"had.addUri",
		"had.addUrls",
		"had.remove",
		"had.removeAll",
		"had.tellStatus",
		"had.tellAllStatus",
		"had.getGlobalStat",
		"had.getFiles",
		"had.pause",
		"had.pauseAll",
		"had.resume",
		"had.resumeAll",
		"had.setSpeedLimit",
		"had.getSpeedLimit",
		"had.setMaxParallel",
		"had.setThreads",
		"had.setOutDir",
		"had.scrape",
		"had.shutdown",
		"had.version",
		"had.getHistory",
		"had.clearHistory",
		"had.setBWSchedule",
		"had.getBWSchedule",
		"had.testMirrors",
		"had.fetchMeta",
		"had.verifyChecksum",
		"had.listSessions",
		"had.resumeSession",
		"had.deleteSession",
		"had.setMirrors",
		"had.getMirrors",
		"had.addWebDAV",
		"had.listWebDAV",
		"had.downloadWebDAVFile",
		"had.pauseFile",
		"had.resumeFile",
		"had.removeFile",
		"had.pauseAllFiles",
		"had.getPausedFiles",
		"system.listMethods",
	}
}

func (rpc *RPCServer) sendResponse(w http.ResponseWriter, id interface{}, result interface{}, rpcErr *RPCError) {
	resp := RPCResponse{ID: id, Result: result, Error: rpcErr}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func (rpc *RPCServer) sendError(w http.ResponseWriter, id interface{}, code int, message string) {
	rpc.sendResponse(w, id, nil, &RPCError{Code: code, Message: message})
}

func generateGID() string {
	return fmt.Sprintf("%016x", time.Now().UnixNano())
}

func toInt64(v interface{}) (int64, bool) {
	switch n := v.(type) {
	case float64:
		return int64(n), true
	case int:
		return int64(n), true
	case int64:
		return n, true
	case json.Number:
		i, err := n.Int64()
		return i, err == nil
	}
	return 0, false
}