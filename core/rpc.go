package core

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"sync"
	"sync/atomic"
	"time"
)

type DownloadTask struct {
	GID       string
	URLs      []string
	Status    string
	Added     time.Time
	Started   time.Time
	Finished  time.Time
	Cancel    context.CancelFunc
	FileName  string
	Size      int64
	Done      int64
	Error     string
}

type RPCServer struct {
	global   *GlobalStatus
	server   *http.Server
	mu       sync.RWMutex
	tasks    map[string]*DownloadTask
	paused   atomic.Bool
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
	return &RPCServer{
		global: global,
		tasks:  make(map[string]*DownloadTask),
	}
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
	gs.mu.RLock()
	defer gs.mu.RUnlock()
	var total int64
	for _, f := range gs.files {
		if f != nil {
			total += f.Done
		}
	}
	return total
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

	rpc.server = &http.Server{
		Addr:         addr,
		Handler:      mux,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
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
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
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
	case "had.addUri", "fad.addUri":
		return rpc.addURI(params)
	case "had.addUrls", "fad.addUrls":
		return rpc.addURLs(params)
	case "had.remove", "fad.remove":
		return rpc.removeDownload(params)
	case "had.removeAll", "fad.removeAll":
		return rpc.removeAll()
	case "had.tellStatus", "fad.tellStatus":
		return rpc.tellStatus(params)
	case "had.tellAllStatus", "fad.tellAllStatus":
		return rpc.tellAllStatus()
	case "had.getGlobalStat", "fad.getGlobalStat":
		return rpc.getGlobalStat()
	case "had.getFiles", "fad.getFiles":
		return rpc.getFiles()
	case "had.pause", "fad.pause":
		return rpc.pauseTask(params)
	case "had.pauseAll", "fad.pauseAll":
		return rpc.pauseAll()
	case "had.resume", "fad.resume":
		return rpc.resumeTask(params)
	case "had.resumeAll", "fad.resumeAll":
		return rpc.resumeAll()
	case "had.setSpeedLimit", "fad.setSpeedLimit":
		return rpc.setSpeedLimit(params)
	case "had.getSpeedLimit", "fad.getSpeedLimit":
		return rpc.getSpeedLimit()
	case "had.setMaxParallel", "fad.setMaxParallel":
		return rpc.setMaxParallel(params)
	case "had.setThreads", "fad.setThreads":
		return rpc.setThreads(params)
	case "had.scrape", "fad.scrape":
		return rpc.scrape(params)
	case "had.shutdown", "fad.shutdown":
		return rpc.shutdown()
	case "had.version", "fad.version":
		return rpc.versionInfo(), nil
	case "system.listMethods":
		return rpc.listMethods(), nil
	default:
		return nil, &RPCError{Code: -32601, Message: "Method not found: " + method}
	}
}

func (rpc *RPCServer) handleStatus(w http.ResponseWriter, req *http.Request) {
	rpc.mu.RLock()
	active := rpc.getActiveCount()
	rpc.mu.RUnlock()

	status := map[string]interface{}{
		"status":           "running",
		"version":          "2.0.0",
		"paused":           rpc.paused.Load(),
		"active_downloads": active,
		"completed_files":  atomic.LoadInt64(&rpc.global.downloadedCount),
		"total_files":      atomic.LoadInt64(&rpc.global.totalCount),
		"total_size":       Size4Human(rpc.global.totalSize()),
		"downloaded_size":  Size4Human(rpc.global.totalDownloaded()),
		"start_time":       rpc.global.startTime.Format(time.RFC3339),
		"uptime":           time.Since(rpc.global.startTime).Round(time.Second).String(),
		"speed_limit":      maxSpeed,
		"max_parallel":     maxParallel,
		"threads":          numThreads,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(status)
}

func (rpc *RPCServer) handleFiles(w http.ResponseWriter, req *http.Request) {
	rpc.global.mu.RLock()
	defer rpc.global.mu.RUnlock()

	type FileInfo struct {
		Name        string  `json:"name"`
		Size        int64   `json:"size"`
		SizeHuman   string  `json:"size_human"`
		Done        int64   `json:"done"`
		DoneHuman   string  `json:"done_human"`
		Progress    float64 `json:"progress"`
		Status      string  `json:"status"`
		Threads     int     `json:"threads"`
		ActiveThr   int     `json:"active_threads"`
		DoneThr     int     `json:"done_threads"`
		ElapsedSec  float64 `json:"elapsed_sec"`
	}

	files := make([]FileInfo, 0, len(rpc.global.files))
	for _, f := range rpc.global.files {
		if f == nil {
			continue
		}
		progress := pctOf(f.Done, f.Total)
		elapsed := 0.0
		if !f.StartTime.IsZero() {
			if f.Status == "downloaded" && !f.EndTime.IsZero() {
				elapsed = f.EndTime.Sub(f.StartTime).Seconds()
			} else {
				elapsed = time.Since(f.StartTime).Seconds()
			}
		}
		files = append(files, FileInfo{
			Name:       f.Name,
			Size:       f.Size,
			SizeHuman:  Size4Human(f.Size),
			Done:       f.Done,
			DoneHuman:  Size4Human(f.Done),
			Progress:   progress,
			Status:     f.Status,
			Threads:    f.TotalThreads,
			ActiveThr:  f.ActiveThreads,
			DoneThr:    f.DoneThreads,
			ElapsedSec: elapsed,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(files)
}

func (rpc *RPCServer) handleTasks(w http.ResponseWriter, req *http.Request) {
	rpc.mu.RLock()
	defer rpc.mu.RUnlock()

	tasks := make([]map[string]interface{}, 0, len(rpc.tasks))
	for _, t := range rpc.tasks {
		tasks = append(tasks, map[string]interface{}{
			"gid":    t.GID,
			"urls":   t.URLs,
			"status": t.Status,
			"added":  t.Added.Format(time.RFC3339),
			"error":  t.Error,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(tasks)
}

func (rpc *RPCServer) handlePause(w http.ResponseWriter, req *http.Request) {
	rpc.paused.Store(true)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "paused"})
}

func (rpc *RPCServer) handleResume(w http.ResponseWriter, req *http.Request) {
	rpc.paused.Store(false)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "resumed"})
}

func (rpc *RPCServer) handleVersion(w http.ResponseWriter, req *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(rpc.versionInfo())
}

func (rpc *RPCServer) addURI(params map[string]interface{}) (interface{}, *RPCError) {
	raw, ok := params["uris"]
	if !ok {
		raw = params["urls"]
	}
	uris, ok := raw.([]interface{})
	if !ok || len(uris) == 0 {
		return nil, &RPCError{Code: -32602, Message: "Invalid params: missing uris/urls array"}
	}

	var urlList []string
	for _, u := range uris {
		if s, ok := u.(string); ok && s != "" {
			urlList = append(urlList, s)
		}
	}
	if len(urlList) == 0 {
		return nil, &RPCError{Code: -32602, Message: "No valid URLs provided"}
	}

	gid := generateGID()
	ctx, cancel := context.WithCancel(context.Background())

	task := &DownloadTask{
		GID:    gid,
		URLs:   urlList,
		Status: "active",
		Added:  time.Now(),
		Cancel: cancel,
	}

	rpc.mu.Lock()
	rpc.tasks[gid] = task
	rpc.mu.Unlock()

	go func() {
		task.Started = time.Now()
		for _, u := range urlList {
			select {
			case <-ctx.Done():
				rpc.mu.Lock()
				task.Status = "removed"
				rpc.mu.Unlock()
				return
			default:
				downloadSingle(u, createHTTPClient(), rpc.global)
			}
		}
		rpc.mu.Lock()
		task.Status = "complete"
		task.Finished = time.Now()
		rpc.mu.Unlock()
	}()

	return map[string]string{"gid": gid}, nil
}

func (rpc *RPCServer) addURLs(params map[string]interface{}) (interface{}, *RPCError) {
	return rpc.addURI(params)
}

func (rpc *RPCServer) removeDownload(params map[string]interface{}) (interface{}, *RPCError) {
	gid, ok := params["gid"].(string)
	if !ok || gid == "" {
		return nil, &RPCError{Code: -32602, Message: "Invalid params: missing gid"}
	}

	rpc.mu.Lock()
	task, exists := rpc.tasks[gid]
	if exists && task.Cancel != nil {
		task.Cancel()
		task.Status = "removed"
	}
	rpc.mu.Unlock()

	if !exists {
		return nil, &RPCError{Code: 1, Message: "GID not found: " + gid}
	}
	return map[string]string{"gid": gid}, nil
}

func (rpc *RPCServer) removeAll() (interface{}, *RPCError) {
	rpc.mu.Lock()
	count := 0
	for _, t := range rpc.tasks {
		if t.Cancel != nil && t.Status == "active" {
			t.Cancel()
			t.Status = "removed"
			count++
		}
	}
	rpc.mu.Unlock()
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
		"gid":    task.GID,
		"status": task.Status,
		"urls":   task.URLs,
		"added":  task.Added.Format(time.RFC3339),
		"error":  task.Error,
	}, nil
}

func (rpc *RPCServer) tellAllStatus() (interface{}, *RPCError) {
	rpc.global.mu.RLock()
	files := make([]map[string]interface{}, 0, len(rpc.global.files))
	for i, f := range rpc.global.files {
		if f == nil {
			continue
		}
		files = append(files, map[string]interface{}{
			"index":             i,
			"name":              f.Name,
			"size":              f.Size,
			"size_human":        Size4Human(f.Size),
			"downloaded":        f.Done,
			"downloaded_human":  Size4Human(f.Done),
			"progress":          pctOf(f.Done, f.Total),
			"status":            f.Status,
			"threads":           f.TotalThreads,
			"active_threads":    f.ActiveThreads,
			"completed_threads": f.DoneThreads,
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

	rpc.mu.RLock()
	active := rpc.getActiveCount()
	rpc.mu.RUnlock()

	return map[string]interface{}{
		"num_active":             active,
		"num_waiting":            0,
		"num_stopped":            atomic.LoadInt64(&rpc.global.downloadedCount),
		"total_files":            atomic.LoadInt64(&rpc.global.totalCount),
		"completed_files":        atomic.LoadInt64(&rpc.global.downloadedCount),
		"total_size":             totalSize,
		"total_size_human":       Size4Human(totalSize),
		"total_downloaded":       totalDL,
		"total_downloaded_human": Size4Human(totalDL),
		"total_progress":         pctOf(totalDL, totalSize),
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
		})
	}
	return files, nil
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
		return map[string]string{"gid": gid, "note": "task not paused"}, nil
	}

	ctx, cancel := context.WithCancel(context.Background())
	rpc.mu.Lock()
	task.Cancel = cancel
	task.Status = "active"
	rpc.mu.Unlock()

	go func() {
		for _, u := range task.URLs {
			select {
			case <-ctx.Done():
				return
			default:
				downloadSingle(u, createHTTPClient(), rpc.global)
			}
		}
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

func (rpc *RPCServer) scrape(params map[string]interface{}) (interface{}, *RPCError) {
	u, ok := params["url"].(string)
	if !ok || u == "" {
		return nil, &RPCError{Code: -32602, Message: "Invalid params: missing url"}
	}

	gid := generateGID()
	gs := NewGlobalStatus()
	go scrapeAndDownload(u, gs)

	return map[string]string{"gid": gid, "url": u, "status": "scraping"}, nil
}

func (rpc *RPCServer) shutdown() (interface{}, *RPCError) {
	go func() {
		time.Sleep(500 * time.Millisecond)
		os.Exit(0)
	}()
	return map[string]string{"status": "shutting down"}, nil
}

func (rpc *RPCServer) versionInfo() interface{} {
	return map[string]string{
		"name":     "HAD (Hyper Advanced Downloader)",
		"version":  "2.0.0",
		"protocol": "had-rpc/2.0",
		"features": "http,https,ftp,ftps,sftp,hls,metalink,scrape,capture-proxy",
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
		"had.scrape",
		"had.shutdown",
		"had.version",
		"system.listMethods",
	}
}

func (rpc *RPCServer) getActiveCount() int {
	count := 0
	for _, f := range rpc.global.files {
		if f != nil && f.Status == "downloading" {
			count++
		}
	}
	return count
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