package webui

import (
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/batmanpriv/had/core"
)

//go:embed static
var staticFiles embed.FS

type WebConfig struct {
	Addr    string
	RPCAddr string
	Token   string
}

type LogBroadcaster struct {
	mu      sync.RWMutex
	clients map[chan string]struct{}
}

var logBroadcast = &LogBroadcaster{
	clients: make(map[chan string]struct{}),
}

func (lb *LogBroadcaster) Subscribe() chan string {
	ch := make(chan string, 256)
	lb.mu.Lock()
	lb.clients[ch] = struct{}{}
	lb.mu.Unlock()
	return ch
}

func (lb *LogBroadcaster) Unsubscribe(ch chan string) {
	lb.mu.Lock()
	delete(lb.clients, ch)
	lb.mu.Unlock()
	close(ch)
}

func (lb *LogBroadcaster) Broadcast(msg string) {
	lb.mu.RLock()
	defer lb.mu.RUnlock()
	for ch := range lb.clients {
		select {
		case ch <- msg:
		default:
		}
	}
}

type WebDownloader struct {
	cfg    WebConfig
	proxy  *httputil.ReverseProxy
	mux    *http.ServeMux
	server *http.Server
}

func DefaultConfig() WebConfig {
	return WebConfig{
		Addr:    ":8090",
		RPCAddr: "http://localhost:6800",
	}
}

func NewWebDownloader(cfg WebConfig) *WebDownloader {
	if cfg.Addr == "" {
		cfg.Addr = ":8090"
	}
	if cfg.RPCAddr == "" {
		cfg.RPCAddr = "http://localhost:6800"
	}

	rpcURL, err := url.Parse(cfg.RPCAddr)
	if err != nil {
		log.Fatalf("invalid rpc addr: %v", err)
	}

	proxy := httputil.NewSingleHostReverseProxy(rpcURL)
	origDirector := proxy.Director
	proxy.Director = func(req *http.Request) {
		origDirector(req)
		req.Host = rpcURL.Host
		req.Header.Set("X-Forwarded-By", "had-webui")
	}
	proxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadGateway)
		json.NewEncoder(w).Encode(map[string]string{
			"error": "HAD RPC unreachable: " + err.Error(),
		})
	}

	wd := &WebDownloader{cfg: cfg, proxy: proxy, mux: http.NewServeMux()}
	wd.routes()
	return wd
}

func (wd *WebDownloader) routes() {
	staticFS, err := fs.Sub(staticFiles, "static")
	if err != nil {
		log.Fatalf("static fs: %v", err)
	}
	fileServer := http.FileServer(http.FS(staticFS))

	wd.mux.HandleFunc("/api/", wd.corsMiddleware(wd.rpcProxy))
	wd.mux.HandleFunc("/jsonrpc", wd.corsMiddleware(wd.rpcProxy))
	wd.mux.HandleFunc("/health", wd.handleHealth)
	wd.mux.HandleFunc("/ws/log", wd.handleLogStream)
	wd.mux.Handle("/", fileServer)
}

func (wd *WebDownloader) corsMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next(w, r)
	}
}

func (wd *WebDownloader) authorized(r *http.Request) bool {
	if wd.cfg.Token == "" {
		return true
	}
	auth := r.Header.Get("Authorization")
	token := strings.TrimPrefix(auth, "Bearer ")
	if token == wd.cfg.Token {
		return true
	}
	return r.URL.Query().Get("token") == wd.cfg.Token
}

func (wd *WebDownloader) rpcProxy(w http.ResponseWriter, r *http.Request) {
	if !wd.authorized(r) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]string{"error": "unauthorized"})
		return
	}
	wd.proxy.ServeHTTP(w, r)
}

func (wd *WebDownloader) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status": "ok",
		"time":   time.Now().Unix(),
		"rpc":    wd.cfg.RPCAddr,
	})
}

func (wd *WebDownloader) handleLogStream(w http.ResponseWriter, r *http.Request) {
	if !wd.authorized(r) {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming not supported", http.StatusInternalServerError)
		return
	}

	ch := logBroadcast.Subscribe()
	defer logBroadcast.Unsubscribe(ch)

	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()
	clientGone := r.Context().Done()

	fmt.Fprintf(w, "data: {\"ts\":%d,\"msg\":\"connected\",\"level\":\"info\"}\n\n", time.Now().Unix())
	flusher.Flush()

	for {
		select {
		case <-clientGone:
			return
		case msg, ok := <-ch:
			if !ok {
				return
			}
			fmt.Fprintf(w, "data: %s\n\n", msg)
			flusher.Flush()
		case t := <-ticker.C:
			fmt.Fprintf(w, "data: {\"ts\":%d,\"msg\":\"ping\",\"level\":\"ping\"}\n\n", t.Unix())
			flusher.Flush()
		}
	}
}

func (wd *WebDownloader) Start() error {
	wd.server = &http.Server{
		Addr:         wd.cfg.Addr,
		Handler:      wd.mux,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 60 * time.Second,
		IdleTimeout:  120 * time.Second,
	}
	log.Printf("HAD Web UI → http://localhost%s\n", wd.cfg.Addr)
	return wd.server.ListenAndServe()
}

func BroadcastLog(level, msg string) {
	data, _ := json.Marshal(map[string]interface{}{
		"ts":    time.Now().Unix(),
		"msg":   msg,
		"level": level,
	})
	logBroadcast.Broadcast(string(data))
}

func RunWebDownloader() {
	cfg := DefaultConfig()

	if addr := os.Getenv("HAD_WEB_ADDR"); addr != "" {
		cfg.Addr = addr
	}
	if rpc := os.Getenv("HAD_RPC_ADDR"); rpc != "" {
		cfg.RPCAddr = rpc
	}
	if token := os.Getenv("HAD_TOKEN"); token != "" {
		cfg.Token = token
	}

	rpcHost := strings.TrimPrefix(cfg.RPCAddr, "http://")
	rpcHost = strings.TrimPrefix(rpcHost, "https://")

	core.InitDefaults()
	core.LogHook = BroadcastLog

	gs := core.NewGlobalStatus()
	srv := core.NewRPCServer(gs)
	if err := srv.Start(rpcHost); err != nil {
		log.Fatalf("cannot start RPC: %v", err)
	}

	log.Printf("RPC server started on %s", rpcHost)
	time.Sleep(200 * time.Millisecond)

	wd := NewWebDownloader(cfg)
	if err := wd.Start(); err != nil && err != http.ErrServerClosed {
		log.Fatal(err)
	}
}