package core

import (
	"bufio"
	"compress/gzip"
	"context"
	"crypto/md5"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/tls"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"encoding/xml"
	"flag"
	"fmt"
	"hash"
	"io"
	"log"
	"math"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/jlaffaye/ftp"
	"github.com/mattn/go-colorable"
	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
	"golang.org/x/net/proxy"
	"golang.org/x/net/webdav"
)

func init() {
	_ = colorable.NewColorable(os.Stdout)
	_ = webdav.Dir(".")
}

var (
	numThreads         int
	headers            headerSlice
	cookie             string
	outDir             string
	retries            int
	timeoutSec         int
	maxParallel        int
	saveSession        bool
	fileList           string
	verbose            bool
	proxyAddr          string
	protocol           string
	ftpUser            string
	ftpPass            string
	ftpMultiPart       bool
	ftpParts           int
	scrapeURL          string
	extensionsFilter   string
	maxSpeed           int64
	diskCacheSize      int64
	enableGzip         bool
	cookieFile         string
	saveCookieFile     string
	netrcFile          string
	checkIntegrity     bool
	checkSha256        string
	checkMd5           string
	checkSha1          string
	parameterizedURL   string
	parameterizedStart int
	parameterizedEnd   int
	parameterizedStep  int
	daemonMode         bool
	pidFile            string
	sshUser            string
	sshPass            string
	sshKeyFile         string
	sfftpKeyPass       string
	metalinkFile       string
	rpcEnabled         bool
	rpcAddr            string
	webSocketRPC       bool
	captureProxy       string
	captureTypes       string
	captureExts        string
	captureAuto        bool
	captureOutput      string
	captureConfidence  int
	captureMinSize     int64
	captureMaxSize     int64
	captureSaveFile    string
	captureHeaders     headerSlice
	captureCookie      string
	downloadFromJson   string
	installCert        bool

	notifyTelegram    string
	notifyTelegramBot string
	notifyDiscord     string
	notifyDesktop     bool

	scheduleFrom string
	scheduleTo   string

	postExtract bool
	postMove    string
	postRename  string

	mirrorURLs string
	autoMirror bool

	queueFile     string
	queuePriority string

	hlsURL string

	webdavURL  string
	webdavUser string
	webdavPass string

	magnetLink string
)

var colors = map[string]string{
	"reset":  "\033[0m",
	"red":    "\033[31m",
	"green":  "\033[32m",
	"yellow": "\033[33m",
	"blue":   "\033[34m",
	"cyan":   "\033[36m",
	"bold":   "\033[1m",
	"gray":   "\033[90m",
}

type Logger struct {
	verbose bool
	mu      sync.Mutex
}

var logger = &Logger{}
var LogHook func(level, msg string)

func (l *Logger) SetVerbose(v bool) { l.verbose = v }

func (l *Logger) emit(level, msg string) {
	if LogHook != nil {
		LogHook(level, msg)
	}
}

func (l *Logger) Info(format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	l.emit("info", msg)
	if !l.verbose {
		return
	}
	l.mu.Lock()
	fmt.Printf(colors["cyan"]+"[INFO] "+colors["reset"]+"%s\n", msg)
	l.mu.Unlock()
}

func (l *Logger) Error(format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	l.emit("error", msg)
	l.mu.Lock()
	fmt.Printf(colors["red"]+"[ERROR] "+colors["reset"]+"%s\n", msg)
	l.mu.Unlock()
}

func (l *Logger) Warning(format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	l.emit("warning", msg)
	l.mu.Lock()
	fmt.Printf(colors["yellow"]+"[WARN] "+colors["reset"]+"%s\n", msg)
	l.mu.Unlock()
}

func (l *Logger) Debug(format string, args ...interface{}) {
	if !l.verbose {
		return
	}
	msg := fmt.Sprintf(format, args...)
	l.emit("debug", msg)
	l.mu.Lock()
	fmt.Printf(colors["gray"]+"[DEBUG] "+colors["reset"]+"%s\n", msg)
	l.mu.Unlock()
}

func (l *Logger) Success(format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	l.emit("success", msg)
	l.mu.Lock()
	fmt.Printf(colors["green"]+"[✓] "+colors["reset"]+"%s\n", msg)
	l.mu.Unlock()
}

func logInfo(format string, args ...interface{})    { logger.Info(format, args...) }
func logError(format string, args ...interface{})   { logger.Error(format, args...) }
func logWarning(format string, args ...interface{}) { logger.Warning(format, args...) }
func logDebug(format string, args ...interface{})   { logger.Debug(format, args...) }
func logSuccess(format string, args ...interface{}) { logger.Success(format, args...) }

func termWidth() int {
	if w, err := getTermWidth(); err == nil && w > 20 {
		return w
	}
	return 80
}

type segment struct {
	start   int64
	end     int64
	written int64
}

func (s *segment) size() int64 {
	if s.end < 0 {
		return -1
	}
	return s.end - s.start + 1
}

func (s *segment) done() bool {
	sz := s.size()
	if sz <= 0 {
		return false
	}
	return atomic.LoadInt64(&s.written) >= sz
}

type writeOp struct {
	data   []byte
	offset int64
	result chan error
}

type writeBuffer struct {
	f    *os.File
	ch   chan writeOp
	done chan struct{}
	err  atomic.Value
}

func newWriteBuffer(f *os.File) *writeBuffer {
	wb := &writeBuffer{
		f:    f,
		ch:   make(chan writeOp, 4096),
		done: make(chan struct{}),
	}
	go wb.loop()
	return wb
}

func (wb *writeBuffer) loop() {
	defer close(wb.done)
	for op := range wb.ch {
		_, err := wb.f.WriteAt(op.data, op.offset)
		if err != nil {
			wb.err.Store(&err)
		}
		if op.result != nil {
			op.result <- err
		}
	}
}

func (wb *writeBuffer) WriteAsync(offset int64, data []byte) {
	cp := make([]byte, len(data))
	copy(cp, data)
	wb.ch <- writeOp{data: cp, offset: offset}
}

func (wb *writeBuffer) WriteSync(offset int64, data []byte) error {
	cp := make([]byte, len(data))
	copy(cp, data)
	res := make(chan error, 1)
	wb.ch <- writeOp{data: cp, offset: offset, result: res}
	return <-res
}

func (wb *writeBuffer) Close() error {
	close(wb.ch)
	<-wb.done
	if v := wb.err.Load(); v != nil {
		return *v.(*error)
	}
	return nil
}

type AdaptiveBuffer struct {
	cur        int
	min        int
	max        int
	history    [10]float64
	histLen    int
	lastAdjust time.Time
	mu         sync.Mutex
}

func newAdaptiveBuffer() *AdaptiveBuffer {
	return &AdaptiveBuffer{cur: 128 * 1024, min: 32 * 1024, max: 4 * 1024 * 1024}
}

func (ab *AdaptiveBuffer) Update(speedMBps float64) {
	ab.mu.Lock()
	defer ab.mu.Unlock()
	if time.Since(ab.lastAdjust) < 2*time.Second {
		return
	}
	if ab.histLen < 10 {
		ab.history[ab.histLen] = speedMBps
		ab.histLen++
	} else {
		copy(ab.history[:], ab.history[1:])
		ab.history[9] = speedMBps
	}
	var avg float64
	for i := 0; i < ab.histLen; i++ {
		avg += ab.history[i]
	}
	avg /= float64(ab.histLen)
	switch {
	case avg > 200:
		ab.cur = ab.max
	case avg > 100:
		ab.cur = clampInt(ab.cur*2, ab.min, ab.max)
	case avg > 50:
		ab.cur = clampInt(ab.cur+512*1024, ab.min, ab.max)
	case avg > 10:
		ab.cur = clampInt(ab.cur+128*1024, ab.min, ab.max)
	case avg > 2:
		ab.cur = clampInt(ab.cur-64*1024, ab.min, ab.max)
	default:
		ab.cur = clampInt(ab.cur-256*1024, ab.min, ab.max)
	}
	ab.lastAdjust = time.Now()
}

func (ab *AdaptiveBuffer) Size() int {
	ab.mu.Lock()
	defer ab.mu.Unlock()
	return ab.cur
}

type speedLimiter struct {
	limit  int64
	bucket int64
}

func newSpeedLimiter(bytesPerSec int64) *speedLimiter {
	sl := &speedLimiter{limit: bytesPerSec}
	if bytesPerSec > 0 {
		atomic.StoreInt64(&sl.bucket, bytesPerSec)
		go sl.refill()
	}
	return sl
}

func (sl *speedLimiter) refill() {
	ticker := time.NewTicker(50 * time.Millisecond)
	defer ticker.Stop()
	for range ticker.C {
		if sl.limit == 0 {
			return
		}
		add := sl.limit / 20
		for {
			cur := atomic.LoadInt64(&sl.bucket)
			if cur >= sl.limit {
				break
			}
			want := cur + add
			if want > sl.limit {
				want = sl.limit
			}
			if atomic.CompareAndSwapInt64(&sl.bucket, cur, want) {
				break
			}
		}
	}
}

func (sl *speedLimiter) Consume(n int64) {
	if sl.limit == 0 {
		return
	}
	for {
		cur := atomic.LoadInt64(&sl.bucket)
		if cur >= n {
			if atomic.CompareAndSwapInt64(&sl.bucket, cur, cur-n) {
				return
			}
		} else {
			time.Sleep(5 * time.Millisecond)
		}
	}
}

type BandwidthSchedule struct {
	From time.Time
	To   time.Time
}

func parseBandwidthSchedule(from, to string) *BandwidthSchedule {
	if from == "" || to == "" {
		return nil
	}
	now := time.Now()
	parseT := func(s string) time.Time {
		parts := strings.Split(s, ":")
		if len(parts) != 2 {
			return time.Time{}
		}
		h, _ := strconv.Atoi(parts[0])
		m, _ := strconv.Atoi(parts[1])
		return time.Date(now.Year(), now.Month(), now.Day(), h, m, 0, 0, now.Location())
	}
	return &BandwidthSchedule{From: parseT(from), To: parseT(to)}
}

func (bs *BandwidthSchedule) IsActive() bool {
	if bs == nil {
		return true
	}
	now := time.Now()
	from := bs.From
	to := bs.To
	if to.Before(from) {
		return now.After(from) || now.Before(to)
	}
	return now.After(from) && now.Before(to)
}

func waitForSchedule(bs *BandwidthSchedule) {
	if bs == nil {
		return
	}
	for !bs.IsActive() {
		logInfo("outside schedule window, waiting...")
		time.Sleep(30 * time.Second)
	}
}

type FileStatus struct {
	Name          string
	URL           string
	Size          int64
	Done          int64
	Total         int64
	Status        string
	StartTime     time.Time
	EndTime       time.Time
	TotalThreads  int
	DoneThreads   int
	ActiveThreads int
	ThreadProg    []int64
	ctrl          *fileControl
	completedOnce sync.Once
}

type fileControl struct {
	pause   func()
	done    <-chan struct{}
	url     string
	outPath string
}

type GlobalStatus struct {
	mu              sync.RWMutex
	files           []*FileStatus
	downloadedCount int64
	totalCount      int64
	startTime       time.Time
	doneCh          chan struct{}
	closeDoneOnce   sync.Once
	totalDone       int64
}

type Session struct {
	URL        string
	Path       string
	Size       int64
	Ranges     [][2]int64
	FileName   string
	Progress   []int64
	Checksum   string
	Algorithm  string
	CreatedAt  time.Time
	UpdatedAt  time.Time
	Mirrors    []string
}

func NewGlobalStatus() *GlobalStatus {
	return &GlobalStatus{
		files:     make([]*FileStatus, 0),
		doneCh:    make(chan struct{}),
		startTime: time.Now(),
	}
}

func (gs *GlobalStatus) addFile(name string, size int64) {
	if size < 0 {
		size = 0
	}
	gs.mu.Lock()
	gs.files = append(gs.files, &FileStatus{
		Name:      name,
		Size:      size,
		Total:     size,
		Status:    "pending",
		StartTime: time.Now(),
	})
	gs.mu.Unlock()
	atomic.AddInt64(&gs.totalCount, 1)
}

func (gs *GlobalStatus) updateProgress(name string, done int64) {
	gs.mu.Lock()
	defer gs.mu.Unlock()
	for _, f := range gs.files {
		if f.Name != name {
			continue
		}
		if f.Total > 0 && done > f.Total {
			done = f.Total
		}
		f.Done = done
		if f.Status == "pending" {
			f.Status = "downloading"
		}
		if f.Total > 0 && done >= f.Total {
			f.completedOnce.Do(func() {
				f.Status = "downloaded"
				f.EndTime = time.Now()
				atomic.AddInt64(&gs.downloadedCount, 1)
			})
		}
		return
	}
}

func (gs *GlobalStatus) setThreadCount(name string, n int) {
	gs.mu.Lock()
	defer gs.mu.Unlock()
	for _, f := range gs.files {
		if f.Name == name {
			f.TotalThreads = n
			f.ThreadProg = make([]int64, n)
			f.ActiveThreads = n
			break
		}
	}
}

func (gs *GlobalStatus) updateThreadProgress(name string, idx int, written, segSize int64) {
	gs.mu.Lock()
	defer gs.mu.Unlock()
	for _, f := range gs.files {
		if f.Name != name {
			continue
		}
		if idx >= 0 && idx < len(f.ThreadProg) {
			f.ThreadProg[idx] = written
		}
		done := 0
		active := 0
		var total int64
		for i, p := range f.ThreadProg {
			total += p
			var seg int64
			if f.TotalThreads > 0 && f.Total > 0 {
				base := f.Total / int64(f.TotalThreads)
				if i == f.TotalThreads-1 {
					seg = f.Total - base*int64(i)
				} else {
					seg = base
				}
			} else {
				seg = segSize
			}
			if seg > 0 && p >= seg {
				done++
			} else if p > 0 {
				active++
			}
		}
		f.DoneThreads = done
		f.ActiveThreads = active
		f.Done = total
		if f.Total > 0 && total > f.Total {
			f.Done = f.Total
		}
		if f.TotalThreads > 0 && done == f.TotalThreads {
			f.completedOnce.Do(func() {
				f.Status = "downloaded"
				f.EndTime = time.Now()
				f.Done = f.Total
				atomic.AddInt64(&gs.downloadedCount, 1)
			})
		}
		return
	}
}

func (gs *GlobalStatus) markError(name string) {
	gs.mu.Lock()
	defer gs.mu.Unlock()
	for _, f := range gs.files {
		if f.Name == name {
			f.Status = "error"
			break
		}
	}
}

func (gs *GlobalStatus) findFile(name string) *FileStatus {
	gs.mu.RLock()
	defer gs.mu.RUnlock()
	for _, f := range gs.files {
		if f.Name == name {
			return f
		}
	}
	return nil
}

func (gs *GlobalStatus) attachControl(name string, ctrl *fileControl) {
	gs.mu.Lock()
	defer gs.mu.Unlock()
	for _, f := range gs.files {
		if f.Name == name {
			f.ctrl = ctrl
			f.URL = ctrl.url
			return
		}
	}
}

func (gs *GlobalStatus) markPaused(name string) {
	gs.mu.Lock()
	defer gs.mu.Unlock()
	for _, f := range gs.files {
		if f.Name == name && f.Status != "downloaded" {
			f.Status = "paused"
			return
		}
	}
}

func (gs *GlobalStatus) markStatus(name, status string) {
	gs.mu.Lock()
	defer gs.mu.Unlock()
	for _, f := range gs.files {
		if f.Name == name {
			f.Status = status
			return
		}
	}
}

func (gs *GlobalStatus) PauseFile(name string) error {
	f := gs.findFile(name)
	if f == nil {
		return fmt.Errorf("file not found: %s", name)
	}
	if f.Status != "downloading" && f.Status != "pending" && f.Status != "hls" {
		return fmt.Errorf("file is not active (status: %s)", f.Status)
	}
	if f.ctrl == nil || f.ctrl.pause == nil {
		return fmt.Errorf("this download type cannot be paused")
	}
	f.ctrl.pause()
	gs.markPaused(name)
	return nil
}

func (gs *GlobalStatus) ResumeFile(name string) (rawURL string, size int64, err error) {
	f := gs.findFile(name)
	if f == nil {
		return "", 0, fmt.Errorf("file not found: %s", name)
	}
	if f.Status != "paused" && f.Status != "error" {
		return "", 0, fmt.Errorf("file is not paused (status: %s)", f.Status)
	}
	if f.URL == "" {
		return "", 0, fmt.Errorf("no source URL recorded for %s — cannot resume", name)
	}
	gs.markStatus(name, "pending")
	return f.URL, f.Size, nil
}

func (gs *GlobalStatus) RemoveFile(name string) error {
	gs.mu.Lock()
	idx := -1
	var f *FileStatus
	for i, fl := range gs.files {
		if fl.Name == name {
			idx, f = i, fl
			break
		}
	}
	if idx == -1 {
		gs.mu.Unlock()
		return fmt.Errorf("file not found: %s", name)
	}
	gs.files = append(gs.files[:idx], gs.files[idx+1:]...)
	gs.mu.Unlock()

	if f.ctrl != nil && f.ctrl.pause != nil {
		f.ctrl.pause()
	}
	return nil
}

func (gs *GlobalStatus) PauseAllFiles() int {
	gs.mu.RLock()
	targets := make([]*FileStatus, 0)
	for _, f := range gs.files {
		if (f.Status == "downloading" || f.Status == "pending" || f.Status == "hls") &&
			f.ctrl != nil && f.ctrl.pause != nil {
			targets = append(targets, f)
		}
	}
	gs.mu.RUnlock()

	for _, f := range targets {
		f.ctrl.pause()
		gs.markPaused(f.Name)
	}
	return len(targets)
}

func (gs *GlobalStatus) PausedFiles() []*FileStatus {
	gs.mu.RLock()
	defer gs.mu.RUnlock()
	out := make([]*FileStatus, 0)
	for _, f := range gs.files {
		if f.Status == "paused" && f.URL != "" {
			out = append(out, f)
		}
	}
	return out
}

func (gs *GlobalStatus) closeDone() {
	gs.closeDoneOnce.Do(func() { close(gs.doneCh) })
}

func (gs *GlobalStatus) reportAllFiles() {
	w := termWidth()
	barW := clampInt(w-70, 10, 30)
	nameW := clampInt(w-barW-45, 10, 36)
	sep := strings.Repeat("─", w)
	heavy := strings.Repeat("═", w)

	prevBytes := int64(0)
	lineCount := 0

	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	moveCursor := func(n int) {
		if n > 0 {
			fmt.Printf("\033[%dA", n)
		}
	}

	statusIcon := func(s string) string {
		return map[string]string{
			"pending":     "⏳",
			"downloading": "⬇",
			"downloaded":  "✅",
			"error":       "❌",
			"hls":         "📺",
			"queued":      "🔃",
		}[s]
	}
	statusColor := func(s string) string {
		return map[string]string{
			"pending":     colors["yellow"],
			"downloading": colors["cyan"],
			"downloaded":  colors["green"],
			"error":       colors["red"],
			"hls":         colors["blue"],
			"queued":      colors["gray"],
		}[s]
	}

	var buf strings.Builder

	render := func(final bool) {
		buf.Reset()
		title := "DOWNLOAD STATUS"
		if final {
			title = "FINAL STATUS"
		}
		buf.WriteString(colors["cyan"] + heavy + colors["reset"] + "\n")
		pad := (w - len(title) - 2) / 2
		buf.WriteString(strings.Repeat(" ", pad) + colors["bold"] + title + colors["reset"] + "\n")
		buf.WriteString(colors["cyan"] + heavy + colors["reset"] + "\n")

		gs.mu.RLock()
		var totalDone, totalSize, activeDownloads, completedFiles int64
		lines := 3

		for i, f := range gs.files {
			if f == nil {
				continue
			}
			pct := pctOf(f.Done, f.Total)
			filled := clampInt(int(pct/100*float64(barW)), 0, barW)
			bar := colors["green"] + strings.Repeat("█", filled) +
				colors["reset"] + strings.Repeat("░", barW-filled)

			name := truncateString(f.Name, nameW)
			icon := statusIcon(f.Status)
			sc := statusColor(f.Status)

			buf.WriteString(fmt.Sprintf("%s%2d.%s %s %s%-*s%s [%s] %5.1f%% %s/%s",
				colors["bold"], i+1, colors["reset"],
				icon, sc, nameW, name, colors["reset"],
				bar, pct,
				Size4Human(f.Done), Size4Human(f.Total)))

			switch f.Status {
			case "downloading", "hls":
				activeDownloads++
				elapsed := time.Since(f.StartTime).Seconds()
				if elapsed > 0 && f.Done > 0 {
					spd := float64(f.Done) / 1024 / 1024 / elapsed
					buf.WriteString(fmt.Sprintf(" %s%.1fMB/s%s", colors["yellow"], spd, colors["reset"]))
					if spd > 0 && f.Total > f.Done {
						rem := float64(f.Total-f.Done) / 1024 / 1024 / spd
						buf.WriteString(fmt.Sprintf(" %sETA:%s%s", colors["cyan"], formatDuration(rem), colors["reset"]))
					}
				}
			case "downloaded":
				completedFiles++
				buf.WriteString(colors["green"] + " done" + colors["reset"])
			case "error":
				buf.WriteString(colors["red"] + " fail" + colors["reset"])
			case "queued":
				buf.WriteString(colors["gray"] + " waiting" + colors["reset"])
			}
			buf.WriteByte('\n')
			lines++

			if f.Status == "downloaded" {
				totalDone += f.Size
			} else {
				totalDone += f.Done
			}
			totalSize += f.Size
		}
		gs.mu.RUnlock()

		elapsed := time.Since(gs.startTime).Seconds()
		curBytes := atomic.LoadInt64(&gs.totalDone)
		diff := curBytes - prevBytes
		prevBytes = curBytes

		avgSpd := float64(curBytes) / 1024 / 1024 / maxF64(elapsed, 0.001)
		instSpd := float64(diff) / 1024 / 1024 / 0.5
		downloaded := atomic.LoadInt64(&gs.downloadedCount)
		totalPct := pctOf(totalDone, totalSize)

		buf.WriteString(colors["cyan"] + sep + colors["reset"] + "\n")
		buf.WriteString(fmt.Sprintf(" Avg:%.1fMB/s Inst:%.1fMB/s Active:%s%d%s Files:%d/%d %.1f%% T:%s\n",
			avgSpd, instSpd,
			colors["cyan"], activeDownloads, colors["reset"],
			downloaded, len(gs.files), totalPct,
			formatDuration(elapsed)))
		lines += 2

		if totalPct > 0 && totalPct < 100 && avgSpd > 0 {
			remBytes := float64(totalSize-totalDone) / 1024 / 1024
			remSec := remBytes / avgSpd
			if remSec < 86400 {
				buf.WriteString(fmt.Sprintf(" ETA:%s  Rem:%.1fMB\n",
					formatDuration(remSec), remBytes))
				lines++
			}
		}

		if final || (completedFiles > 0 && completedFiles == int64(len(gs.files))) {
			buf.WriteString(colors["green"] + " All downloads completed!" + colors["reset"] + "\n")
			lines++
		}

		if lineCount > 0 {
			moveCursor(lineCount)
		}
		fmt.Print(buf.String())
		lineCount = lines
	}

	for {
		select {
		case <-ticker.C:
			w = termWidth()
			barW = clampInt(w-70, 10, 30)
			nameW = clampInt(w-barW-45, 10, 36)
			sep = strings.Repeat("─", w)
			heavy = strings.Repeat("═", w)
			render(false)
			gs.mu.RLock()
			nFiles := int64(len(gs.files))
			gs.mu.RUnlock()
			if atomic.LoadInt64(&gs.downloadedCount) >= nFiles && nFiles > 0 {
				time.Sleep(time.Second)
				render(true)
				gs.closeDone()
				return
			}
		case <-gs.doneCh:
			render(true)
			return
		}
	}
}

type MirrorResult struct {
	URL     string
	Latency time.Duration
}

func probeMirrors(urls []string, client *http.Client) []string {
	if len(urls) <= 1 {
		return urls
	}
	results := make([]MirrorResult, 0, len(urls))
	var mu sync.Mutex
	var wg sync.WaitGroup

	for _, u := range urls {
		wg.Add(1)
		go func(target string) {
			defer wg.Done()
			start := time.Now()
			req, err := http.NewRequest("HEAD", target, nil)
			if err != nil {
				return
			}
			req.Header.Set("User-Agent", "Mozilla/5.0")
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			req = req.WithContext(ctx)
			resp, err := client.Do(req)
			if err != nil {
				return
			}
			resp.Body.Close()
			if resp.StatusCode >= 200 && resp.StatusCode < 400 {
				mu.Lock()
				results = append(results, MirrorResult{URL: target, Latency: time.Since(start)})
				mu.Unlock()
			}
		}(u)
	}
	wg.Wait()

	sort.Slice(results, func(i, j int) bool {
		return results[i].Latency < results[j].Latency
	})

	sorted := make([]string, 0, len(results))
	for _, r := range results {
		logInfo("mirror %s latency: %v", r.URL, r.Latency)
		sorted = append(sorted, r.URL)
	}
	if len(sorted) == 0 {
		return urls
	}
	return sorted
}

type SmartMirrorSelector struct {
	mu        sync.Mutex
	mirrors   []string
	speeds    map[string]int64
	failures  map[string]int
	lastTest  map[string]time.Time
	client    *http.Client
}

func NewSmartMirrorSelector(urls []string, client *http.Client) *SmartMirrorSelector {
	s := &SmartMirrorSelector{
		mirrors:  urls,
		speeds:   make(map[string]int64),
		failures: make(map[string]int),
		lastTest: make(map[string]time.Time),
		client:   client,
	}
	for _, u := range urls {
		s.speeds[u] = 0
		s.failures[u] = 0
	}
	return s
}

func (s *SmartMirrorSelector) TestAll() {
	var wg sync.WaitGroup
	for _, u := range s.mirrors {
		wg.Add(1)
		go func(target string) {
			defer wg.Done()
			speed := s.measureSpeed(target)
			s.mu.Lock()
			s.speeds[target] = speed
			s.lastTest[target] = time.Now()
			s.mu.Unlock()
		}(u)
	}
	wg.Wait()
}

func (s *SmartMirrorSelector) measureSpeed(target string) int64 {
	req, err := http.NewRequest("GET", target, nil)
	if err != nil {
		return 0
	}
	req.Header.Set("Range", "bytes=0-65535")
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	req = req.WithContext(ctx)
	defer cancel()

	start := time.Now()
	resp, err := s.client.Do(req)
	if err != nil || (resp.StatusCode != 200 && resp.StatusCode != 206) {
		if resp != nil {
			resp.Body.Close()
		}
		return 0
	}
	n, _ := io.Copy(io.Discard, resp.Body)
	resp.Body.Close()

	elapsed := time.Since(start).Seconds()
	if elapsed <= 0 {
		return 0
	}
	return int64(float64(n) / elapsed)
}

func (s *SmartMirrorSelector) Best() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	best := ""
	var bestSpeed int64
	for _, u := range s.mirrors {
		if s.failures[u] >= 3 {
			continue
		}
		spd := s.speeds[u]
		if best == "" || spd > bestSpeed {
			best = u
			bestSpeed = spd
		}
	}
	if best == "" && len(s.mirrors) > 0 {
		return s.mirrors[0]
	}
	return best
}

func (s *SmartMirrorSelector) ReportFailure(url string) {
	s.mu.Lock()
	s.failures[url]++
	s.mu.Unlock()
}

func (s *SmartMirrorSelector) ShouldRetest(url string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	last, ok := s.lastTest[url]
	if !ok {
		return true
	}
	return time.Since(last) > 30*time.Second
}

func buildMirrorList(primary string, extra []string) []string {
	all := []string{primary}
	for _, u := range extra {
		if u != "" && u != primary {
			all = append(all, u)
		}
	}
	return all
}

type DownloadQueue struct {
	mu    sync.Mutex
	items []QueueItem
}

type QueueItem struct {
	URL      string
	Priority int
	Added    time.Time
}

func NewDownloadQueue() *DownloadQueue {
	return &DownloadQueue{}
}

func (q *DownloadQueue) Add(u string, priority int) {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.items = append(q.items, QueueItem{URL: u, Priority: priority, Added: time.Now()})
	sort.Slice(q.items, func(i, j int) bool {
		if q.items[i].Priority != q.items[j].Priority {
			return q.items[i].Priority > q.items[j].Priority
		}
		return q.items[i].Added.Before(q.items[j].Added)
	})
}

func (q *DownloadQueue) Pop() (QueueItem, bool) {
	q.mu.Lock()
	defer q.mu.Unlock()
	if len(q.items) == 0 {
		return QueueItem{}, false
	}
	item := q.items[0]
	q.items = q.items[1:]
	return item, true
}

func (q *DownloadQueue) Len() int {
	q.mu.Lock()
	defer q.mu.Unlock()
	return len(q.items)
}

func (q *DownloadQueue) LoadFromFile(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	for i, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, " ", 2)
		priority := i
		u := parts[0]
		if len(parts) == 2 {
			if p, err := strconv.Atoi(strings.TrimSpace(parts[1])); err == nil {
				priority = p
			}
		}
		q.Add(u, priority)
	}
	return nil
}

type Notifier struct {
	telegramToken  string
	telegramChatID string
	discordWebhook string
	desktop        bool
}

func NewNotifier() *Notifier {
	n := &Notifier{desktop: notifyDesktop}
	if notifyTelegram != "" && notifyTelegramBot != "" {
		n.telegramChatID = notifyTelegram
		n.telegramToken = notifyTelegramBot
	}
	if notifyDiscord != "" {
		n.discordWebhook = notifyDiscord
	}
	return n
}

func (n *Notifier) IsEnabled() bool {
	return n.telegramToken != "" || n.discordWebhook != "" || n.desktop
}

func (n *Notifier) Send(title, message string) {
	if !n.IsEnabled() {
		return
	}
	if n.telegramToken != "" && n.telegramChatID != "" {
		go n.sendTelegram(title, message)
	}
	if n.discordWebhook != "" {
		go n.sendDiscord(title, message)
	}
	if n.desktop {
		go n.sendDesktop(title, message)
	}
}

func (n *Notifier) sendTelegram(title, message string) {
	text := fmt.Sprintf("*%s*\n%s", title, message)
	apiURL := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", n.telegramToken)
	payload := fmt.Sprintf(`{"chat_id":"%s","text":"%s","parse_mode":"Markdown"}`,
		n.telegramChatID, strings.ReplaceAll(text, `"`, `\"`))
	req, err := http.NewRequest("POST", apiURL, strings.NewReader(payload))
	if err != nil {
		logDebug("telegram notify: %v", err)
		return
	}
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		logDebug("telegram notify: %v", err)
		return
	}
	resp.Body.Close()
}

func (n *Notifier) sendDiscord(title, message string) {
	payload := fmt.Sprintf(`{"embeds":[{"title":"%s","description":"%s","color":3066993}]}`,
		strings.ReplaceAll(title, `"`, `\"`),
		strings.ReplaceAll(message, `"`, `\"`))
	req, err := http.NewRequest("POST", n.discordWebhook, strings.NewReader(payload))
	if err != nil {
		logDebug("discord notify: %v", err)
		return
	}
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		logDebug("discord notify: %v", err)
		return
	}
	resp.Body.Close()
}

func (n *Notifier) sendDesktop(title, message string) {
	switch runtime.GOOS {
	case "linux", "android":
		exec.Command("notify-send", title, message).Run()
	case "darwin":
		script := fmt.Sprintf(`display notification "%s" with title "%s"`, message, title)
		exec.Command("osascript", "-e", script).Run()
	case "windows":
		exec.Command("powershell", "-Command",
			fmt.Sprintf(`[System.Windows.Forms.MessageBox]::Show('%s','%s')`, message, title)).Run()
	}
}

type HLSDownloader struct {
	url      string
	outPath  string
	fileName string
	client   *http.Client
	gs       *GlobalStatus
	notifier *Notifier
}

func NewHLSDownloader(rawURL, outPath, fileName string, client *http.Client, gs *GlobalStatus, notifier *Notifier) *HLSDownloader {
	return &HLSDownloader{
		url:      rawURL,
		outPath:  outPath,
		fileName: fileName,
		client:   client,
		gs:       gs,
		notifier: notifier,
	}
}

func (h *HLSDownloader) Download() error {
	if ffmpegPath, err := exec.LookPath("ffmpeg"); err == nil {
		return h.downloadWithFFmpeg(ffmpegPath)
	}
	logInfo("ffmpeg not found, using pure-Go HLS downloader")
	return h.downloadPureGo()
}

func (h *HLSDownloader) downloadWithFFmpeg(ffmpegPath string) error {
	logInfo("HLS via ffmpeg: %s", h.url)
	out := h.outPath
	if !strings.HasSuffix(out, ".mp4") && !strings.HasSuffix(out, ".mkv") && !strings.HasSuffix(out, ".ts") {
		out = out + ".mp4"
	}
	cmd := exec.Command(ffmpegPath,
		"-y",
		"-i", h.url,
		"-c", "copy",
		"-bsf:a", "aac_adtstoasc",
		out,
	)
	cmd.Stdout = nil
	cmd.Stderr = nil
	if h.gs != nil {
		h.gs.addFile(h.fileName, -1)
		h.gs.updateProgress(h.fileName, 0)
	}
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("ffmpeg: %v", err)
	}
	if h.gs != nil {
		if fi, err := os.Stat(out); err == nil {
			h.gs.updateProgress(h.fileName, fi.Size())
		}
	}
	return nil
}

func (h *HLSDownloader) downloadPureGo() error {
	playlist, err := h.fetchPlaylist(h.url)
	if err != nil {
		return fmt.Errorf("fetch playlist: %v", err)
	}

	segments := h.parseM3U8(playlist, h.url)
	if len(segments) == 0 {
		return fmt.Errorf("no segments found in playlist")
	}

	logInfo("HLS: %d segments found", len(segments))

	out := h.outPath
	if !strings.HasSuffix(out, ".ts") && !strings.HasSuffix(out, ".mp4") {
		out = out + ".ts"
	}

	f, err := os.Create(out)
	if err != nil {
		return err
	}
	defer f.Close()

	totalSize := int64(len(segments)) * 2 * 1024 * 1024
	if h.gs != nil {
		h.gs.addFile(h.fileName, totalSize)
	}

	var downloaded int64
	for i, segURL := range segments {
		data, err := h.fetchSegment(segURL)
		if err != nil {
			logWarning("HLS segment %d/%d failed: %v", i+1, len(segments), err)
			continue
		}
		f.Write(data)
		downloaded += int64(len(data))
		if h.gs != nil {
			h.gs.updateProgress(h.fileName, downloaded)
		}
	}
	return nil
}

func (h *HLSDownloader) fetchPlaylist(rawURL string) (string, error) {
	req, err := http.NewRequest("GET", rawURL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0")
	resp, err := h.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	return string(data), err
}

func (h *HLSDownloader) parseM3U8(content, baseURL string) []string {
	var segments []string
	lines := strings.Split(content, "\n")

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		abs := toAbsoluteURL(line, baseURL)
		segments = append(segments, abs)
	}

	if len(segments) == 1 && (strings.Contains(segments[0], ".m3u8") || strings.Contains(content, "#EXT-X-STREAM-INF")) {
		subPlaylist, err := h.fetchPlaylist(segments[0])
		if err == nil {
			return h.parseM3U8(subPlaylist, segments[0])
		}
	}

	return segments
}

func (h *HLSDownloader) fetchSegment(segURL string) ([]byte, error) {
	var lastErr error
	for attempt := 1; attempt <= 3; attempt++ {
		req, err := http.NewRequest("GET", segURL, nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("User-Agent", "Mozilla/5.0")
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		req = req.WithContext(ctx)
		resp, err := h.client.Do(req)
		cancel()
		if err != nil {
			lastErr = err
			time.Sleep(time.Duration(attempt) * time.Second)
			continue
		}
		data, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			lastErr = err
			time.Sleep(time.Duration(attempt) * time.Second)
			continue
		}
		return data, nil
	}
	return nil, lastErr
}

func isHLSURL(rawURL string) bool {
	lower := strings.ToLower(rawURL)
	return strings.HasSuffix(lower, ".m3u8") ||
		strings.HasSuffix(lower, ".m3u") ||
		strings.Contains(lower, "/hls/") ||
		strings.Contains(lower, "playlist.m3u8") ||
		strings.Contains(lower, "index.m3u8")
}

func isMagnetLink(s string) bool {
	return strings.HasPrefix(s, "magnet:")
}

type MagnetInfo struct {
	InfoHash    string
	DisplayName string
	Trackers    []string
	WebSeeds    []string
}

func parseMagnetLink(magnet string) (*MagnetInfo, error) {
	u, err := url.Parse(magnet)
	if err != nil {
		return nil, fmt.Errorf("invalid magnet link: %v", err)
	}
	q := u.Query()
	info := &MagnetInfo{}
	if xt := q.Get("xt"); strings.HasPrefix(xt, "urn:btih:") {
		info.InfoHash = strings.TrimPrefix(xt, "urn:btih:")
	}
	if dn := q.Get("dn"); dn != "" {
		info.DisplayName = dn
	}
	info.Trackers = q["tr"]
	info.WebSeeds = q["ws"]
	return info, nil
}

func downloadMagnet(magnet string, gs *GlobalStatus) {
	info, err := parseMagnetLink(magnet)
	if err != nil {
		logError("magnet parse: %v", err)
		return
	}
	logInfo("magnet: hash=%s name=%s trackers=%d webseeds=%d",
		info.InfoHash, info.DisplayName, len(info.Trackers), len(info.WebSeeds))

	if len(info.WebSeeds) > 0 {
		client := createHTTPClient()
		selector := NewSmartMirrorSelector(info.WebSeeds, client)
		selector.TestAll()
		best := selector.Best()
		if best != "" {
			logInfo("magnet web seed: %s", best)
			fileName := info.DisplayName
			if fileName == "" {
				fileName = info.InfoHash
			}
			if gs != nil {
				gs.addFile(fileName, 0)
			}
			downloadSingleFromURL(best, client, gs, 0, fileName)
			return
		}
	}

	logWarning("magnet: no web seeds available, hash=%s", info.InfoHash)
	logWarning("magnet: install a BitTorrent client to download torrent files")
}

type WebDAVDownloader struct {
	baseURL  string
	user     string
	pass     string
	client   *http.Client
	gs       *GlobalStatus
}

func NewWebDAVDownloader(baseURL, user, pass string, client *http.Client, gs *GlobalStatus) *WebDAVDownloader {
	return &WebDAVDownloader{
		baseURL: strings.TrimRight(baseURL, "/"),
		user:    user,
		pass:    pass,
		client:  client,
		gs:      gs,
	}
}

type webdavPropfind struct {
	XMLName   xml.Name        `xml:"multistatus"`
	Responses []webdavResponse `xml:"response"`
}

type webdavResponse struct {
	Href     string         `xml:"href"`
	Propstat webdavPropstat `xml:"propstat"`
}

type webdavPropstat struct {
	Prop   webdavProp `xml:"prop"`
	Status string     `xml:"status"`
}

type webdavProp struct {
	DisplayName     string `xml:"displayname"`
	ContentLength   int64  `xml:"getcontentlength"`
	ContentType     string `xml:"getcontenttype"`
	ResourceType    string `xml:"resourcetype"`
	LastModified    string `xml:"getlastmodified"`
}

func (w *WebDAVDownloader) List(path string) ([]webdavResponse, error) {
	targetURL := w.baseURL + "/" + strings.TrimLeft(path, "/")
	req, err := http.NewRequest("PROPFIND", targetURL, strings.NewReader(`<?xml version="1.0"?>
<d:propfind xmlns:d="DAV:">
  <d:prop>
    <d:displayname/>
    <d:getcontentlength/>
    <d:getcontenttype/>
    <d:resourcetype/>
    <d:getlastmodified/>
  </d:prop>
</d:propfind>`))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Depth", "1")
	req.Header.Set("Content-Type", "application/xml")
	if w.user != "" {
		req.SetBasicAuth(w.user, w.pass)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	req = req.WithContext(ctx)
	defer cancel()

	resp, err := w.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 207 {
		return nil, fmt.Errorf("WebDAV PROPFIND: HTTP %d", resp.StatusCode)
	}

	var result webdavPropfind
	if err := xml.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("WebDAV parse: %v", err)
	}
	return result.Responses, nil
}

func (w *WebDAVDownloader) DownloadFile(path, fileName string, size int64) {
	targetURL := w.baseURL + "/" + strings.TrimLeft(path, "/")
	if w.user != "" {
		u, _ := url.Parse(targetURL)
		u.User = url.UserPassword(w.user, w.pass)
		targetURL = u.String()
	}
	if w.gs != nil {
		w.gs.addFile(fileName, size)
	}
	downloadSingleFromURL(targetURL, w.client, w.gs, size, fileName)
}

func (w *WebDAVDownloader) DownloadAll(path string) {
	items, err := w.List(path)
	if err != nil {
		logError("WebDAV list %s: %v", path, err)
		return
	}

	var wg sync.WaitGroup
	sem := make(chan struct{}, maxParallel)

	for _, item := range items {
		href := item.Href
		if strings.HasSuffix(href, "/") {
			continue
		}
		fileName := filepath.Base(href)
		size := item.Propstat.Prop.ContentLength
		filePath := href

		wg.Add(1)
		sem <- struct{}{}
		go func(fp, fn string, sz int64) {
			defer wg.Done()
			defer func() { <-sem }()
			w.DownloadFile(fp, fn, sz)
		}(filePath, fileName, size)
	}
	wg.Wait()
}

type FileMetadata struct {
	FileName    string `json:"file_name"`
	Size        int64  `json:"size"`
	SizeHuman   string `json:"size_human"`
	Extension   string `json:"extension"`
	ContentType string `json:"content_type"`
	Resumable   bool   `json:"resumable"`
	Checksum    string `json:"checksum_hint,omitempty"`
	LastMod     string `json:"last_modified,omitempty"`
}

func FetchFileMetadata(rawURL string, client *http.Client) (*FileMetadata, error) {
	req, err := http.NewRequest("HEAD", rawURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0")

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	req = req.WithContext(ctx)
	defer cancel()

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 400 {
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	name := getFileName(rawURL, resp)
	ext := ""
	if i := strings.LastIndex(name, "."); i >= 0 {
		ext = name[i+1:]
	}

	meta := &FileMetadata{
		FileName:    name,
		Size:        resp.ContentLength,
		SizeHuman:   Size4Human(resp.ContentLength),
		Extension:   ext,
		ContentType: resp.Header.Get("Content-Type"),
		Resumable:   strings.Contains(resp.Header.Get("Accept-Ranges"), "bytes"),
		LastMod:     resp.Header.Get("Last-Modified"),
	}

	if resp.ContentLength <= 0 {
		req2, _ := http.NewRequest("GET", rawURL, nil)
		req2.Header.Set("User-Agent", "Mozilla/5.0")
		req2.Header.Set("Range", "bytes=0-0")
		ctx2, cancel2 := context.WithTimeout(context.Background(), 10*time.Second)
		req2 = req2.WithContext(ctx2)
		resp2, err2 := client.Do(req2)
		cancel2()
		if err2 == nil {
			io.Copy(io.Discard, resp2.Body)
			resp2.Body.Close()
			if cr := resp2.Header.Get("Content-Range"); cr != "" {
				parts := strings.Split(cr, "/")
				if len(parts) == 2 {
					if s, e := strconv.ParseInt(parts[1], 10, 64); e == nil {
						meta.Size = s
						meta.SizeHuman = Size4Human(s)
					}
				}
			}
		}
	}

	meta.Checksum = fetchExternalChecksumURL(rawURL, client)
	return meta, nil
}

func fetchExternalChecksumURL(rawURL string, client *http.Client) string {
	for _, suffix := range []string{".sha256", ".sha256sum", ".md5", ".md5sum", ".sha1"} {
		req, err := http.NewRequest("HEAD", rawURL+suffix, nil)
		if err != nil {
			continue
		}
		ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
		req = req.WithContext(ctx)
		resp, err := client.Do(req)
		cancel()
		if err == nil && resp.StatusCode == 200 {
			resp.Body.Close()
			return rawURL + suffix
		}
		if resp != nil {
			resp.Body.Close()
		}
	}
	return ""
}

func fetchChecksumFromURL(checksumURL string, client *http.Client) (string, string) {
	req, err := http.NewRequest("GET", checksumURL, nil)
	if err != nil {
		return "", ""
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	req = req.WithContext(ctx)
	resp, err := client.Do(req)
	cancel()
	if err != nil {
		return "", ""
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(resp.Body)
	content := strings.TrimSpace(string(data))

	parts := strings.Fields(content)
	if len(parts) >= 1 {
		hash := parts[0]
		algo := "sha256"
		if strings.HasSuffix(checksumURL, ".md5") || strings.HasSuffix(checksumURL, ".md5sum") {
			algo = "md5"
		} else if strings.HasSuffix(checksumURL, ".sha1") {
			algo = "sha1"
		}
		return hash, algo
	}
	return "", ""
}

type PostProcessor struct {
	extract  bool
	moveDir  string
	rename   string
	notifier *Notifier
}

func NewPostProcessor(notifier *Notifier) *PostProcessor {
	return &PostProcessor{
		extract:  postExtract,
		moveDir:  postMove,
		rename:   postRename,
		notifier: notifier,
	}
}

func (pp *PostProcessor) Run(filePath string) {
	if pp.extract {
		pp.tryExtract(filePath)
	}
	target := filePath
	if pp.rename != "" {
		newName := strings.ReplaceAll(pp.rename, "{name}", filepath.Base(filePath))
		newName = strings.ReplaceAll(newName, "{time}", strconv.FormatInt(time.Now().Unix(), 10))
		newPath := filepath.Join(filepath.Dir(filePath), newName)
		if err := os.Rename(filePath, newPath); err == nil {
			logInfo("renamed: %s → %s", filepath.Base(filePath), newName)
			target = newPath
		}
	}
	if pp.moveDir != "" {
		os.MkdirAll(pp.moveDir, 0755)
		dest := filepath.Join(pp.moveDir, filepath.Base(target))
		if err := os.Rename(target, dest); err == nil {
			logInfo("moved: %s → %s", filepath.Base(target), pp.moveDir)
			target = dest
		}
	}
	if pp.notifier.IsEnabled() {
		pp.notifier.Send("✅ Download Complete",
			fmt.Sprintf("File: %s\nSize: %s", filepath.Base(target), Size4Human(fileSize(target))))
	}
}

func (pp *PostProcessor) tryExtract(filePath string) {
	ext := strings.ToLower(filepath.Ext(filePath))
	dir := strings.TrimSuffix(filePath, filepath.Ext(filePath))
	os.MkdirAll(dir, 0755)
	var cmd *exec.Cmd
	switch ext {
	case ".zip":
		cmd = exec.Command("unzip", "-o", filePath, "-d", dir)
	case ".tar":
		cmd = exec.Command("tar", "-xf", filePath, "-C", dir)
	case ".gz", ".tgz":
		cmd = exec.Command("tar", "-xzf", filePath, "-C", dir)
	case ".bz2", ".tbz2":
		cmd = exec.Command("tar", "-xjf", filePath, "-C", dir)
	case ".xz":
		cmd = exec.Command("tar", "-xJf", filePath, "-C", dir)
	case ".rar":
		cmd = exec.Command("unrar", "x", "-o+", filePath, dir)
	case ".7z":
		cmd = exec.Command("7z", "x", "-o"+dir, "-y", filePath)
	default:
		return
	}
	if err := cmd.Run(); err != nil {
		logWarning("extract %s: %v", filepath.Base(filePath), err)
	} else {
		logSuccess("extracted: %s → %s", filepath.Base(filePath), dir)
	}
}

func fileSize(path string) int64 {
	fi, err := os.Stat(path)
	if err != nil {
		return 0
	}
	return fi.Size()
}

type Downloader struct {
	url           string
	file          *os.File
	wb            *writeBuffer
	headers       http.Header
	client        *http.Client
	size          int64
	segments      []*segment
	path          string
	fileName      string
	global        *GlobalStatus
	totalDone     *int64
	retries       int
	ctx           context.Context
	cancel        context.CancelFunc
	ab            *AdaptiveBuffer
	limiter       *speedLimiter
	doneCh        chan struct{}
	mirrors       []string
	mirrorIdx     int64
	mirrorSelect  *SmartMirrorSelector
	schedule      *BandwidthSchedule
	notifier      *Notifier
	postProc      *PostProcessor
	paused        atomic.Bool
	sessionPath   string
	autoChecksum  string
	checksumAlgo  string
}

func newDownloader(rawURL, outPath, fileName string, size int64, segs []*segment,
	client *http.Client, f *os.File, gs *GlobalStatus) *Downloader {
	ctx, cancel := context.WithCancel(context.Background())
	notifier := NewNotifier()
	return &Downloader{
		url:         rawURL,
		file:        f,
		wb:          newWriteBuffer(f),
		headers:     defaultHeaders(),
		client:      client,
		size:        size,
		segments:    segs,
		path:        outPath,
		fileName:    fileName,
		global:      gs,
		totalDone:   &gs.totalDone,
		retries:     retries,
		ctx:         ctx,
		cancel:      cancel,
		ab:          newAdaptiveBuffer(),
		limiter:     newSpeedLimiter(maxSpeed),
		doneCh:      make(chan struct{}),
		mirrors:     []string{rawURL},
		schedule:    parseBandwidthSchedule(scheduleFrom, scheduleTo),
		notifier:    notifier,
		postProc:    NewPostProcessor(notifier),
		sessionPath: outPath + ".had",
	}
}

func (dl *Downloader) setMirrors(mirrors []string) {
	dl.mirrors = mirrors
	if len(mirrors) > 1 {
		dl.mirrorSelect = NewSmartMirrorSelector(mirrors, dl.client)
		dl.mirrorSelect.TestAll()
	}
}

func (dl *Downloader) Pause() {
	dl.paused.Store(true)
	dl.cancel()
}

func (dl *Downloader) nextMirror() string {
	if dl.mirrorSelect != nil {
		return dl.mirrorSelect.Best()
	}
	idx := atomic.AddInt64(&dl.mirrorIdx, 1) % int64(len(dl.mirrors))
	return dl.mirrors[idx]
}

func defaultHeaders() http.Header {
	h := make(http.Header)
	h.Set("User-Agent",
		"Mozilla/5.0 (Windows NT 10.0; Win64; x64) "+
			"AppleWebKit/537.36 (KHTML, like Gecko) "+
			"Chrome/124.0.0.0 Safari/537.36")
	return h
}

func buildSegments(size int64, threads int) []*segment {
	if size <= 0 || threads <= 1 {
		return []*segment{{start: 0, end: -1}}
	}
	segs := make([]*segment, threads)
	base := size / int64(threads)
	for i := 0; i < threads; i++ {
		s := int64(i) * base
		e := s + base - 1
		if i == threads-1 {
			e = size - 1
		}
		segs[i] = &segment{start: s, end: e}
	}
	return segs
}

func resumeSegments(ranges [][2]int64, progress []int64) []*segment {
	segs := make([]*segment, len(ranges))
	for i, r := range ranges {
		w := int64(0)
		if i < len(progress) {
			w = progress[i]
		}
		segs[i] = &segment{start: r[0], end: r[1], written: w}
	}
	return segs
}

func (dl *Downloader) downloadPart(idx int) error {
	seg := dl.segments[idx]
	segSize := seg.size()

	defer func() {
		written := atomic.LoadInt64(&seg.written)
		if dl.global != nil {
			dl.global.updateThreadProgress(dl.fileName, idx, written, segSize)
		}
	}()

	for attempt := 1; attempt <= dl.retries; attempt++ {
		if err := dl.ctx.Err(); err != nil {
			return nil
		}

		waitForSchedule(dl.schedule)

		written := atomic.LoadInt64(&seg.written)
		currentStart := seg.start + written
		if seg.end >= 0 && currentStart > seg.end {
			return nil
		}

		targetURL := dl.url
		if len(dl.mirrors) > 1 && attempt > 1 {
			targetURL = dl.nextMirror()
			logInfo("segment %d switching to mirror: %s", idx, targetURL)
		}

		if dl.mirrorSelect != nil && dl.mirrorSelect.ShouldRetest(targetURL) {
			go func(u string) {
				dl.mirrorSelect.measureSpeed(u)
			}(targetURL)
		}

		err := dl.doRequest(idx, currentStart, seg.end, segSize, targetURL)
		if err == nil {
			return nil
		}
		if err == context.Canceled {
			return nil
		}

		if dl.mirrorSelect != nil {
			dl.mirrorSelect.ReportFailure(targetURL)
		}

		logWarning("segment %d attempt %d/%d: %v", idx, attempt, dl.retries, err)
		if attempt < dl.retries {
			backoff := time.Duration(attempt*attempt) * 500 * time.Millisecond
			if backoff > 30*time.Second {
				backoff = 30 * time.Second
			}
			select {
			case <-time.After(backoff):
			case <-dl.ctx.Done():
				return nil
			}
		}
	}
	return fmt.Errorf("segment %d failed after %d retries", idx, dl.retries)
}

func (dl *Downloader) doRequest(idx int, start, end, segSize int64, targetURL string) error {
	req, err := http.NewRequestWithContext(dl.ctx, "GET", targetURL, nil)
	if err != nil {
		return err
	}
	req.Header = dl.headers.Clone()
	if end >= 0 {
		req.Header.Set("Range", fmt.Sprintf("bytes=%d-%d", start, end))
	}
	if enableGzip {
		req.Header.Set("Accept-Encoding", "gzip, deflate, br")
	}

	resp, err := dl.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusOK, http.StatusPartialContent:
	case http.StatusRequestedRangeNotSatisfiable:
		return nil
	default:
		return fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	writeOffset := start
	if resp.StatusCode == http.StatusOK && start > 0 {
		if _, err := io.CopyN(io.Discard, resp.Body, start); err != nil {
			return fmt.Errorf("seek-skip: %v", err)
		}
	}

	var body io.Reader = resp.Body
	if enableGzip && resp.Header.Get("Content-Encoding") == "gzip" {
		gr, err := gzip.NewReader(resp.Body)
		if err != nil {
			return fmt.Errorf("gzip: %v", err)
		}
		defer gr.Close()
		body = gr
	}

	seg := dl.segments[idx]
	buf := make([]byte, dl.ab.Size())
	tStart := time.Now()
	var bytesThisReq int64

	for {
		n, readErr := body.Read(buf)
		if n > 0 {
			chunk := buf[:n]
			if end >= 0 && writeOffset+int64(n) > end+1 {
				chunk = buf[:end+1-writeOffset]
			}
			if len(chunk) == 0 {
				break
			}
			dl.limiter.Consume(int64(len(chunk)))
			dl.wb.WriteAsync(writeOffset, chunk)
			writeOffset += int64(len(chunk))
			atomic.AddInt64(&seg.written, int64(len(chunk)))
			atomic.AddInt64(dl.totalDone, int64(len(chunk)))
			bytesThisReq += int64(len(chunk))

			if dl.global != nil {
				dl.global.updateThreadProgress(dl.fileName, idx, atomic.LoadInt64(&seg.written), segSize)
			}
			if bytesThisReq%(1024*1024) == 0 {
				elapsed := time.Since(tStart).Seconds()
				if elapsed > 0 {
					dl.ab.Update(float64(bytesThisReq) / 1024 / 1024 / elapsed)
					buf = make([]byte, dl.ab.Size())
				}
			}
		}
		if readErr == io.EOF {
			return nil
		}
		if readErr != nil {
			return readErr
		}
		if end >= 0 && writeOffset > end {
			break
		}
	}
	return nil
}

func (dl *Downloader) Run() {
	nSegs := len(dl.segments)
	if dl.global != nil {
		dl.global.setThreadCount(dl.fileName, nSegs)
		for i, seg := range dl.segments {
			dl.global.updateThreadProgress(dl.fileName, i, atomic.LoadInt64(&seg.written), seg.size())
		}
	}

	var wg sync.WaitGroup
	errCh := make(chan error, nSegs)

	for i := range dl.segments {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			if dl.segments[idx].done() {
				return
			}
			if err := dl.downloadPart(idx); err != nil {
				logError("thread %d: %v", idx, err)
				errCh <- err
			}
		}(i)
	}

	wg.Wait()

	if err := dl.wb.Close(); err != nil {
		logError("write-buffer flush: %v", err)
	}
	close(errCh)
	var hadErr bool
	for range errCh {
		hadErr = true
	}

	if dl.paused.Load() {
		dl.saveSession()
		if dl.global != nil {
			dl.global.markPaused(dl.fileName)
		}
		close(dl.doneCh)
		return
	}

	dl.cancel()
	os.Remove(dl.path + ".progress")
	os.Remove(dl.sessionPath)

	if hadErr && dl.global != nil {
		dl.global.markError(dl.fileName)
		if dl.notifier.IsEnabled() {
			dl.notifier.Send("❌ Download Failed", fmt.Sprintf("File: %s", dl.fileName))
		}
	} else {
		if dl.autoChecksum != "" {
			hash, _ := computeFileHash(dl.path, dl.checksumAlgo)
			if strings.EqualFold(hash, dl.autoChecksum) {
				logSuccess("checksum OK (%s): %s", dl.checksumAlgo, hash)
			} else {
				logWarning("checksum MISMATCH: expected=%s got=%s", dl.autoChecksum, hash)
			}
		}
		dl.postProc.Run(dl.path)
	}

	close(dl.doneCh)
}

func (dl *Downloader) saveSession() {
	prog := make([]int64, len(dl.segments))
	rngs := make([][2]int64, len(dl.segments))
	for i, seg := range dl.segments {
		prog[i] = atomic.LoadInt64(&seg.written)
		rngs[i] = [2]int64{seg.start, seg.end}
	}
	s := Session{
		URL:       dl.url,
		Path:      dl.path,
		Size:      dl.size,
		Ranges:    rngs,
		FileName:  dl.fileName,
		Progress:  prog,
		Checksum:  dl.autoChecksum,
		Algorithm: dl.checksumAlgo,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		Mirrors:   dl.mirrors,
	}
	f, err := os.Create(dl.sessionPath)
	if err != nil {
		logError("save session: %v", err)
		return
	}
	defer f.Close()
	json.NewEncoder(f).Encode(s)
	logInfo("Session saved → %s", dl.sessionPath)
}

func (dl *Downloader) saveProgress() {
	type pd struct {
		Progress []int64
		Ranges   [][2]int64
	}
	p := pd{
		Progress: make([]int64, len(dl.segments)),
		Ranges:   make([][2]int64, len(dl.segments)),
	}
	for i, seg := range dl.segments {
		p.Progress[i] = atomic.LoadInt64(&seg.written)
		p.Ranges[i] = [2]int64{seg.start, seg.end}
	}
	data, _ := json.Marshal(p)
	os.WriteFile(dl.path+".progress", data, 0644)
}

func removeDuplicateURLs(urls []string) []string {
	seen := make(map[string]bool)
	result := make([]string, 0, len(urls))
	for _, u := range urls {
		baseFile := filepath.Base(strings.Split(u, "?")[0])
		nameOnly := strings.TrimSuffix(baseFile, filepath.Ext(baseFile))
		if !seen[nameOnly] {
			seen[nameOnly] = true
			result = append(result, u)
		} else {
			logDebug("Skipping duplicate: %s", u)
		}
	}
	return result
}

func createHTTPClient() *http.Client {
	dialer := &net.Dialer{
		Timeout:   time.Duration(timeoutSec) * time.Second,
		KeepAlive: 90 * time.Second,
	}
	tr := &http.Transport{
		MaxIdleConns:          2000,
		MaxIdleConnsPerHost:   numThreads * 4,
		TLSHandshakeTimeout:   20 * time.Second,
		DisableCompression:    !enableGzip,
		IdleConnTimeout:       120 * time.Second,
		ResponseHeaderTimeout: 60 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
		WriteBufferSize:       64 * 1024,
		ReadBufferSize:        64 * 1024,
		DialContext:           dialer.DialContext,
	}
	if proxyAddr != "" {
		if err := applyProxy(tr, dialer); err != nil {
			logWarning("proxy setup: %v", err)
		}
	}
	return &http.Client{Transport: tr, Timeout: 0}
}

func applyProxy(tr *http.Transport, dialer *net.Dialer) error {
	u, err := url.Parse(proxyAddr)
	if err != nil {
		return err
	}
	switch {
	case strings.HasPrefix(proxyAddr, "socks5://"):
		var auth *proxy.Auth
		if u.User != nil {
			p, _ := u.User.Password()
			auth = &proxy.Auth{User: u.User.Username(), Password: p}
		}
		d, err := proxy.SOCKS5("tcp", u.Host, auth, dialer)
		if err != nil {
			return err
		}
		tr.DialContext = func(ctx context.Context, network, addr string) (net.Conn, error) {
			return d.Dial(network, addr)
		}
	case strings.HasPrefix(proxyAddr, "socks4://"):
		tr.DialContext = func(ctx context.Context, network, addr string) (net.Conn, error) {
			return socks4Dial(ctx, dialer, u.Host, addr)
		}
	case strings.HasPrefix(proxyAddr, "http://"), strings.HasPrefix(proxyAddr, "https://"):
		tr.Proxy = http.ProxyURL(u)
	default:
		return fmt.Errorf("unsupported proxy scheme: %s", proxyAddr)
	}
	return nil
}

func socks4Dial(ctx context.Context, dialer *net.Dialer, proxyHost, targetAddr string) (net.Conn, error) {
	conn, err := dialer.DialContext(ctx, "tcp", proxyHost)
	if err != nil {
		return nil, err
	}
	host, portStr, _ := net.SplitHostPort(targetAddr)
	port, _ := strconv.Atoi(portStr)
	ip := net.ParseIP(host).To4()
	if ip == nil {
		ip = net.IPv4(0, 0, 0, 1).To4()
	}
	pkt := []byte{4, 1, byte(port >> 8), byte(port), ip[0], ip[1], ip[2], ip[3]}
	pkt = append(pkt, []byte("had\x00")...)
	if _, err := conn.Write(pkt); err != nil {
		conn.Close()
		return nil, err
	}
	resp := make([]byte, 8)
	if _, err := io.ReadFull(conn, resp); err != nil || resp[1] != 90 {
		conn.Close()
		return nil, fmt.Errorf("SOCKS4 handshake failed (code %d)", resp[1])
	}
	return conn, nil
}

func fetchFileInfo(rawURL string, client *http.Client) (name string, size int64, err error) {
	req, _ := http.NewRequest("HEAD", rawURL, nil)
	req.Header.Set("User-Agent", "Mozilla/5.0")
	resp, err := client.Do(req)
	if err == nil {
		defer resp.Body.Close()
		if resp.StatusCode == 200 || resp.StatusCode == 206 {
			name = getFileName(rawURL, resp)
			if resp.ContentLength > 0 {
				return name, resp.ContentLength, nil
			}
		}
	}

	req2, _ := http.NewRequest("GET", rawURL, nil)
	req2.Header.Set("User-Agent", "Mozilla/5.0")
	req2.Header.Set("Range", "bytes=0-0")
	resp2, err2 := client.Do(req2)
	if err2 == nil {
		defer resp2.Body.Close()
		io.Copy(io.Discard, resp2.Body)
		if cr := resp2.Header.Get("Content-Range"); cr != "" {
			parts := strings.Split(cr, "/")
			if len(parts) == 2 {
				if s, e := strconv.ParseInt(parts[1], 10, 64); e == nil && s > 0 {
					if name == "" {
						name = getFileName(rawURL, resp2)
					}
					return name, s, nil
				}
			}
		}
	}

	logWarning("cannot determine size via HEAD/Range for %s, doing full GET", rawURL)
	req3, _ := http.NewRequest("GET", rawURL, nil)
	req3.Header.Set("User-Agent", "Mozilla/5.0")
	resp3, err3 := client.Do(req3)
	if err3 != nil {
		return "", -1, err3
	}
	defer resp3.Body.Close()
	s, _ := io.Copy(io.Discard, resp3.Body)
	if name == "" {
		name = getFileName(rawURL, resp3)
	}
	return name, s, nil
}

func supportsRanges(rawURL string, client *http.Client) bool {
	req, err := http.NewRequest("HEAD", rawURL, nil)
	if err != nil {
		return false
	}
	req.Header.Set("User-Agent", "Mozilla/5.0")
	resp, err := client.Do(req)
	if err != nil {
		return false
	}
	resp.Body.Close()
	return strings.Contains(resp.Header.Get("Accept-Ranges"), "bytes")
}

func downloadSingle(rawURL string, client *http.Client, gs *GlobalStatus) {
	if isMagnetLink(rawURL) {
		downloadMagnet(rawURL, gs)
		return
	}

	if isHLSURL(rawURL) {
		fileName := getFileName(rawURL, nil)
		outPath := filepath.Join(outDir, fileName)
		notifier := NewNotifier()
		hls := NewHLSDownloader(rawURL, outPath, fileName, client, gs, notifier)
		if err := hls.Download(); err != nil {
			logError("HLS download: %v", err)
			if gs != nil {
				gs.markError(fileName)
			}
		}
		return
	}

	proto := protocol
	if proto == "auto" {
		switch {
		case strings.HasPrefix(rawURL, "ftp://"):
			proto = "ftp"
		case strings.HasPrefix(rawURL, "ftps://"):
			proto = "ftps"
		case strings.HasPrefix(rawURL, "sftp://"):
			proto = "sftp"
		case strings.HasPrefix(rawURL, "davs://") || strings.HasPrefix(rawURL, "dav://"):
			proto = "webdav"
		}
	}
	if proto == "ftp" || proto == "ftps" {
		downloadFTP(rawURL, gs)
		return
	}
	if proto == "sftp" {
		downloadSFTP(rawURL, gs)
		return
	}
	if proto == "webdav" {
		u := strings.Replace(rawURL, "dav://", "http://", 1)
		u = strings.Replace(u, "davs://", "https://", 1)
		wd := NewWebDAVDownloader(u, webdavUser, webdavPass, client, gs)
		wd.DownloadAll("/")
		return
	}

	fileName, size, err := fetchFileInfo(rawURL, client)
	if err != nil {
		logError("file info: %v", err)
		if gs != nil {
			gs.markError(rawURL)
		}
		return
	}
	if size <= 0 {
		logError("cannot determine size for %s", fileName)
		return
	}
	downloadSingleFromURL(rawURL, client, gs, size, fileName)
}

func downloadSingleFromURL(rawURL string, client *http.Client, gs *GlobalStatus, knownSize int64, fileName string) {
	size := knownSize
	if size <= 0 {
		var err error
		fileName, size, err = fetchFileInfo(rawURL, client)
		if err != nil || size <= 0 {
			logError("cannot determine file size for %s", rawURL)
			return
		}
	}

	outPath := filepath.Join(outDir, fileName)

	var existingSegs []*segment
	var existingSession *Session

	if data, err := os.ReadFile(outPath + ".had"); err == nil {
		var s Session
		if json.Unmarshal(data, &s) == nil && len(s.Ranges) > 0 {
			existingSession = &s
			existingSegs = resumeSegments(s.Ranges, s.Progress)
			logInfo("resuming from .had session: %s (%d segments, mirrors: %v)",
				fileName, len(existingSegs), s.Mirrors)
		}
	} else if data, err := os.ReadFile(outPath + ".progress"); err == nil {
		var pd struct {
			Progress []int64
			Ranges   [][2]int64
		}
		if json.Unmarshal(data, &pd) == nil && len(pd.Ranges) > 0 {
			existingSegs = resumeSegments(pd.Ranges, pd.Progress)
			logInfo("resuming from .progress: %s (%d segments)", fileName, len(existingSegs))
		}
	}

	nThreads := numThreads
	if !supportsRanges(rawURL, client) {
		nThreads = 1
		logDebug("server does not support ranges — single thread")
	}

	var segs []*segment
	if existingSegs != nil {
		segs = existingSegs
	} else {
		segs = buildSegments(size, nThreads)
	}

	f, err := os.OpenFile(outPath, os.O_CREATE|os.O_RDWR, 0644)
	if err != nil {
		logError("cannot open %s: %v", outPath, err)
		return
	}
	defer f.Close()

	if size > 0 {
		if err := f.Truncate(size); err != nil {
			logWarning("truncate: %v", err)
		}
	}

	if gs != nil {
		gs.updateProgress(fileName, 0)
	}

	dl := newDownloader(rawURL, outPath, fileName, size, segs, client, f, gs)
	applyCommonHeaders(dl, rawURL)

	if existingSession != nil && len(existingSession.Mirrors) > 1 {
		dl.setMirrors(existingSession.Mirrors)
	} else if autoMirror && mirrorURLs != "" {
		extras := strings.Split(mirrorURLs, ",")
		all := buildMirrorList(rawURL, extras)
		dl.setMirrors(all)
		best := dl.nextMirror()
		if best != rawURL {
			logInfo("fastest mirror: %s", best)
			dl.url = best
		}
	}

	if checksumURL := fetchExternalChecksumURL(rawURL, client); checksumURL != "" {
		if hash, algo := fetchChecksumFromURL(checksumURL, client); hash != "" {
			dl.autoChecksum = hash
			dl.checksumAlgo = algo
			logInfo("auto checksum detected: %s (%s)", hash, algo)
		}
	}

	if gs != nil {
		gs.attachControl(fileName, &fileControl{
			pause:   dl.Pause,
			done:    dl.doneCh,
			url:     rawURL,
			outPath: outPath,
		})
	}

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigCh
		logInfo("interrupted — saving session")
		dl.saveSession()
		os.Exit(0)
	}()

	saveTicker := time.NewTicker(10 * time.Second)
	go func() {
		for {
			select {
			case <-saveTicker.C:
				dl.saveSession()
			case <-dl.doneCh:
				return
			}
		}
	}()

	dl.Run()
	saveTicker.Stop()

	if checkIntegrity || checkSha256 != "" || checkMd5 != "" || checkSha1 != "" {
		if err := verifyChecksum(outPath); err != nil {
			logError("integrity check: %v", err)
		}
	}

	if saveCookieFile != "" {
		saveCookiesToFile([]string{}, saveCookieFile)
	}
}

func applyCommonHeaders(dl *Downloader, rawURL string) {
	finalCookie := cookie
	if cookieFile != "" {
		if c, err := loadCookiesFromFile(cookieFile); err == nil && c != "" {
			if finalCookie != "" {
				finalCookie += "; " + c
			} else {
				finalCookie = c
			}
		}
	}
	if finalCookie != "" {
		dl.headers.Set("Cookie", finalCookie)
	}
	if netrcFile != "" {
		u, _ := url.Parse(rawURL)
		if u != nil {
			if user, pass := getAuthFromNetrc(u.Host); user != "" {
				dl.headers.Set("Authorization", "Basic "+basicAuth(user, pass))
			}
		}
	}
	for _, h := range headers {
		parts := strings.SplitN(h, ":", 2)
		if len(parts) == 2 {
			dl.headers.Set(strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1]))
		}
	}
}

func connectFTP(host string, useTLS bool) (*ftp.ServerConn, error) {
	if useTLS {
		return ftp.Dial(host+":21", ftp.DialWithTLS(createTLSConfig(host, false)))
	}
	return ftp.Dial(host + ":21")
}

func createTLSConfig(host string, insecure bool) *tls.Config {
	cfg := &tls.Config{
		MinVersion:               tls.VersionTLS12,
		PreferServerCipherSuites: true,
	}
	if insecure {
		cfg.InsecureSkipVerify = true
	} else {
		cfg.ServerName = strings.Split(host, ":")[0]
	}
	return cfg
}

func downloadFTP(fileURL string, gs *GlobalStatus) {
	if !ftpMultiPart {
		downloadFTPSingle(fileURL, gs)
		return
	}
	parts := ftpParts
	if parts <= 0 {
		parts = clampInt(numThreads, 2, 16)
	}
	downloadFTPMultiPart(fileURL, gs, parts)
}

func downloadFTPSingle(fileURL string, gs *GlobalStatus) {
	pu, err := url.Parse(fileURL)
	if err != nil {
		logError("FTP URL: %v", err)
		return
	}
	host, path := pu.Host, pu.Path
	if path == "" {
		path = "/"
	}
	fileName := filepath.Base(path)
	if fileName == "" || fileName == "." || fileName == "/" {
		fileName = fmt.Sprintf("ftp_dl_%d", time.Now().Unix())
	}
	outPath := filepath.Join(outDir, fileName)

	c, err := connectFTP(host, protocol == "ftps")
	if err != nil {
		logError("FTP connect: %v", err)
		return
	}
	defer c.Quit()
	if err := c.Login(ftpUser, ftpPass); err != nil {
		logError("FTP login: %v", err)
		return
	}

	size, _ := c.FileSize(path)
	if gs != nil {
		gs.addFile(fileName, size)
	}

	existing := int64(0)
	if fi, err := os.Stat(outPath); err == nil {
		existing = fi.Size()
		if existing >= size && size > 0 {
			logSuccess("already complete: %s", fileName)
			if gs != nil {
				gs.updateProgress(fileName, size)
			}
			return
		}
	}

	f, err := os.OpenFile(outPath, os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		logError("create file: %v", err)
		return
	}
	defer f.Close()
	if existing > 0 {
		f.Seek(existing, io.SeekStart)
	}

	var reader io.ReadCloser
	if existing > 0 {
		reader, err = c.RetrFrom(path, uint64(existing))
	} else {
		reader, err = c.Retr(path)
	}
	if err != nil {
		logError("FTP RETR: %v", err)
		return
	}
	defer reader.Close()

	buf := make([]byte, 256*1024)
	downloaded := existing
	t0 := time.Now()
	lastPrint := time.Now()
	for {
		n, err := reader.Read(buf)
		if n > 0 {
			f.Write(buf[:n])
			downloaded += int64(n)
			atomic.AddInt64(&gs.totalDone, int64(n))
			if gs != nil {
				gs.updateProgress(fileName, downloaded)
			}
			if time.Since(lastPrint) >= time.Second {
				elapsed := time.Since(t0).Seconds()
				spd := float64(downloaded-existing) / 1024 / 1024 / maxF64(elapsed, 0.001)
				fmt.Printf("\r%sFTP %s %.1f%% %.2fMB/s%s",
					colors["cyan"], fileName, pctOf(downloaded, size), spd, colors["reset"])
				lastPrint = time.Now()
			}
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			fmt.Println()
			logError("FTP read: %v", err)
			return
		}
	}
	fmt.Println()
	logSuccess("FTP complete: %s", fileName)
}

func downloadFTPMultiPart(fileURL string, gs *GlobalStatus, numParts int) {
	pu, err := url.Parse(fileURL)
	if err != nil {
		logError("FTP URL: %v", err)
		return
	}
	host, path := pu.Host, pu.Path
	if path == "" {
		path = "/"
	}
	fileName := filepath.Base(path)
	if fileName == "" || fileName == "." || fileName == "/" {
		fileName = fmt.Sprintf("ftp_dl_%d", time.Now().Unix())
	}
	outPath := filepath.Join(outDir, fileName)

	c, err := connectFTP(host, protocol == "ftps")
	if err != nil {
		logError("FTP connect: %v", err)
		return
	}
	defer c.Quit()
	if err := c.Login(ftpUser, ftpPass); err != nil {
		logError("FTP login: %v", err)
		return
	}

	size, err := c.FileSize(path)
	if err != nil || size < 10*1024*1024 {
		logInfo("falling back to single-thread FTP")
		downloadFTPSingle(fileURL, gs)
		return
	}

	if fi, err := os.Stat(outPath); err == nil && fi.Size() >= size {
		logSuccess("already complete: %s", fileName)
		if gs != nil {
			gs.addFile(fileName, size)
			gs.updateProgress(fileName, size)
		}
		return
	}

	f, err := os.OpenFile(outPath, os.O_CREATE|os.O_RDWR, 0644)
	if err != nil {
		logError("create file: %v", err)
		return
	}
	defer f.Close()
	f.Truncate(size)

	segs := buildSegments(size, numParts)
	if gs != nil {
		gs.addFile(fileName, size)
		gs.setThreadCount(fileName, numParts)
	}

	wb := newWriteBuffer(f)
	var wg sync.WaitGroup
	for i, seg := range segs {
		wg.Add(1)
		go func(idx int, sg *segment) {
			defer wg.Done()
			if err := downloadFTPSegment(host, path, wb, gs, fileName, idx, sg, size); err != nil {
				logError("FTP segment %d: %v", idx, err)
			}
		}(i, seg)
	}
	wg.Wait()
	wb.Close()
	logSuccess("FTP multi-part complete: %s", fileName)
	if gs != nil {
		gs.updateProgress(fileName, size)
	}
}

func downloadFTPSegment(host, path string, wb *writeBuffer, gs *GlobalStatus, fileName string, idx int, seg *segment, totalSize int64) error {
	c, err := connectFTP(host, protocol == "ftps")
	if err != nil {
		return err
	}
	defer c.Quit()
	if err := c.Login(ftpUser, ftpPass); err != nil {
		return err
	}

	written := atomic.LoadInt64(&seg.written)
	startPos := seg.start + written
	reader, err := c.RetrFrom(path, uint64(startPos))
	if err != nil {
		return err
	}
	defer reader.Close()

	buf := make([]byte, 256*1024)
	pos := startPos
	segSize := seg.size()

	for {
		n, err := reader.Read(buf)
		if n > 0 {
			chunk := buf[:n]
			if pos+int64(n) > seg.end+1 {
				chunk = buf[:seg.end+1-pos]
			}
			if len(chunk) == 0 {
				break
			}
			wb.WriteAsync(pos, chunk)
			pos += int64(len(chunk))
			atomic.AddInt64(&seg.written, int64(len(chunk)))
			atomic.AddInt64(&gs.totalDone, int64(len(chunk)))
			if gs != nil {
				gs.updateThreadProgress(fileName, idx, atomic.LoadInt64(&seg.written), segSize)
			}
		}
		if err == io.EOF || pos > seg.end {
			break
		}
		if err != nil {
			return err
		}
	}
	return nil
}

func connectSFTP(sftpURL string) (*sftp.Client, error) {
	pu, err := url.Parse(sftpURL)
	if err != nil {
		return nil, err
	}
	host := pu.Host
	user, pass := ftpUser, ftpPass
	if pu.User != nil {
		if u := pu.User.Username(); u != "" {
			user = u
		}
		if p, ok := pu.User.Password(); ok {
			pass = p
		}
	}
	if user == "" {
		if u, p := getAuthFromNetrc(host); u != "" {
			user, pass = u, p
		} else {
			user = "anonymous"
		}
	}

	var auths []ssh.AuthMethod
	if sshKeyFile != "" {
		keyData, err := os.ReadFile(sshKeyFile)
		if err == nil {
			var signer ssh.Signer
			if sfftpKeyPass != "" {
				signer, err = ssh.ParsePrivateKeyWithPassphrase(keyData, []byte(sfftpKeyPass))
			} else {
				signer, err = ssh.ParsePrivateKey(keyData)
			}
			if err == nil {
				auths = append(auths, ssh.PublicKeys(signer))
			}
		}
	}
	if pass != "" && pass != "anonymous@example.com" {
		auths = append(auths, ssh.Password(pass))
	}

	cfg := &ssh.ClientConfig{
		User: user,
		Auth: auths,
		HostKeyCallback: func(_ string, _ net.Addr, key ssh.PublicKey) error {
			logDebug("SSH fingerprint: %s", ssh.FingerprintSHA256(key))
			return nil
		},
		Timeout: time.Duration(timeoutSec) * time.Second,
	}
	conn, err := ssh.Dial("tcp", host+":22", cfg)
	if err != nil {
		return nil, err
	}
	return sftp.NewClient(conn)
}

func downloadSFTP(fileURL string, gs *GlobalStatus) {
	sc, err := connectSFTP(fileURL)
	if err != nil {
		logError("SFTP connect: %v", err)
		return
	}
	defer sc.Close()

	pu, _ := url.Parse(fileURL)
	path := pu.Path
	if path == "" {
		path = "/"
	}
	fileName := filepath.Base(path)
	if fileName == "" || fileName == "." || fileName == "/" {
		fileName = fmt.Sprintf("sftp_dl_%d", time.Now().Unix())
	}
	outPath := filepath.Join(outDir, fileName)

	fi, err := sc.Stat(path)
	if err != nil {
		logError("SFTP stat: %v", err)
		return
	}
	size := fi.Size()

	if gs != nil {
		gs.addFile(fileName, size)
	}

	existing := int64(0)
	if lfi, err := os.Stat(outPath); err == nil {
		existing = lfi.Size()
		if existing >= size && size > 0 {
			logSuccess("already complete: %s", fileName)
			if gs != nil {
				gs.updateProgress(fileName, size)
			}
			return
		}
	}

	f, err := os.OpenFile(outPath, os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		logError("create file: %v", err)
		return
	}
	defer f.Close()
	if existing > 0 {
		f.Seek(existing, io.SeekStart)
	}

	rf, err := sc.Open(path)
	if err != nil {
		logError("SFTP open: %v", err)
		return
	}
	defer rf.Close()
	if existing > 0 {
		rf.Seek(existing, io.SeekStart)
	}

	buf := make([]byte, 256*1024)
	downloaded := existing
	t0 := time.Now()
	lastPrint := time.Now()
	limiter := newSpeedLimiter(maxSpeed)

	for {
		n, err := rf.Read(buf)
		if n > 0 {
			limiter.Consume(int64(n))
			f.Write(buf[:n])
			downloaded += int64(n)
			atomic.AddInt64(&gs.totalDone, int64(n))
			if gs != nil {
				gs.updateProgress(fileName, downloaded)
			}
			if time.Since(lastPrint) >= time.Second {
				elapsed := time.Since(t0).Seconds()
				spd := float64(downloaded-existing) / 1024 / 1024 / maxF64(elapsed, 0.001)
				fmt.Printf("\rSFTP %s %.1f%% %.2fMB/s", fileName, pctOf(downloaded, size), spd)
				lastPrint = time.Now()
			}
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			fmt.Println()
			logError("SFTP read: %v", err)
			return
		}
	}
	fmt.Println()
	logSuccess("SFTP complete: %s", fileName)
	if gs != nil && size > 0 {
		gs.updateProgress(fileName, size)
	}
}

func resumeFromSession(file string, gs *GlobalStatus) {
	f, err := os.Open(file)
	if err != nil {
		die("open session:", err)
	}
	var s Session
	if err := json.NewDecoder(f).Decode(&s); err != nil {
		f.Close()
		die("decode session:", err)
	}
	f.Close()

	if gs == nil {
		gs = NewGlobalStatus()
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

	segs := resumeSegments(s.Ranges, s.Progress)
	fout, err := os.OpenFile(s.Path, os.O_WRONLY|os.O_CREATE, 0644)
	if err != nil {
		die("open file:", err)
	}
	defer fout.Close()

	gs.addFile(fileName, s.Size)
	client := createHTTPClient()
	dl := newDownloader(s.URL, s.Path, fileName, s.Size, segs, client, fout, gs)
	applyCommonHeaders(dl, s.URL)

	if len(s.Mirrors) > 1 {
		dl.setMirrors(s.Mirrors)
	}
	if s.Checksum != "" {
		dl.autoChecksum = s.Checksum
		dl.checksumAlgo = s.Algorithm
	}

	go gs.reportAllFiles()
	dl.Run()
	os.Remove(file)
	gs.closeDone()
	time.Sleep(time.Second)
}

func verifyChecksum(filePath string) error {
	f, err := os.Open(filePath)
	if err != nil {
		return err
	}
	defer f.Close()

	var h hash.Hash
	var expected string
	switch {
	case checkSha256 != "":
		h, expected = sha256.New(), strings.ToLower(strings.TrimSpace(checkSha256))
	case checkMd5 != "":
		h, expected = md5.New(), strings.ToLower(strings.TrimSpace(checkMd5))
	case checkSha1 != "":
		h, expected = sha1.New(), strings.ToLower(strings.TrimSpace(checkSha1))
	default:
		return nil
	}

	if _, err := io.Copy(h, f); err != nil {
		return err
	}
	got := hex.EncodeToString(h.Sum(nil))
	if got != expected {
		return fmt.Errorf("checksum mismatch\n  expected: %s\n  got:      %s", expected, got)
	}
	logSuccess("checksum OK: %s", got)
	return nil
}

func computeFileHash(filePath, algo string) (string, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer f.Close()

	var h hash.Hash
	switch strings.ToLower(algo) {
	case "sha256":
		h = sha256.New()
	case "md5":
		h = md5.New()
	case "sha1":
		h = sha1.New()
	default:
		return "", fmt.Errorf("unsupported algorithm: %s", algo)
	}
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func DownloadFromCapturedJSON(jsonFile string, maxConcurrent int) error {
	data, err := os.ReadFile(jsonFile)
	if err != nil {
		return fmt.Errorf("read JSON: %v", err)
	}
	var items []CapturedItem
	if err := json.Unmarshal(data, &items); err != nil {
		return fmt.Errorf("parse JSON: %v", err)
	}
	if len(items) == 0 {
		return fmt.Errorf("no items in %s", jsonFile)
	}

	valid := items[:0]
	for _, it := range items {
		if it.URL != "" {
			valid = append(valid, it)
		}
	}
	if len(valid) == 0 {
		return fmt.Errorf("no downloadable items")
	}
	if maxConcurrent <= 0 {
		maxConcurrent = 3
	}

	gs := NewGlobalStatus()
	for _, it := range valid {
		gs.addFile(getFileNameFromItem(it), it.Size)
	}

	sem := make(chan struct{}, maxConcurrent)
	var wg sync.WaitGroup
	go gs.reportAllFiles()

	for _, it := range valid {
		wg.Add(1)
		sem <- struct{}{}
		go func(item CapturedItem) {
			defer wg.Done()
			defer func() { <-sem }()
			oldN := numThreads
			numThreads = determineThreadsBySize(item.Size)
			os.MkdirAll(outDir, 0755)
			downloadSingleFromURL(item.URL, createHTTPClient(), gs, item.Size, getFileNameFromItem(item))
			numThreads = oldN
		}(it)
	}

	wg.Wait()
	gs.closeDone()
	logSuccess("all captured downloads complete")
	return nil
}

func getFileNameFromItem(item CapturedItem) string {
	if item.Title != "" && item.Title != "unknown" {
		safe := sanitizeFileName(item.Title)
		if item.Extension != "" && !strings.HasSuffix(safe, item.Extension) {
			return safe + item.Extension
		}
		return safe
	}
	base := filepath.Base(strings.Split(item.URL, "?")[0])
	if base == "" || base == "/" || base == "." {
		return fmt.Sprintf("download_%d%s", item.Timestamp.Unix(), item.Extension)
	}
	return base
}

func sanitizeFileName(s string) string {
	repl := strings.NewReplacer("/", "_", "\\", "_", ":", "_",
		"*", "_", "?", "_", "\"", "_", "<", "_", ">", "_", "|", "_")
	return repl.Replace(s)
}

func determineThreadsBySize(size int64) int {
	switch {
	case size > 500*1024*1024:
		return 8
	case size > 200*1024*1024:
		return 6
	case size > 50*1024*1024:
		return 4
	case size > 10*1024*1024:
		return 3
	case size > 1024*1024:
		return 2
	default:
		return 1
	}
}

func generateParameterizedURLs() []string {
	var urls []string
	for i := parameterizedStart; i <= parameterizedEnd; i += parameterizedStep {
		u := strings.ReplaceAll(parameterizedURL, "{}", strconv.Itoa(i))
		u = strings.ReplaceAll(u, "{0}", fmt.Sprintf("%02d", i))
		u = strings.ReplaceAll(u, "{00}", fmt.Sprintf("%03d", i))
		urls = append(urls, u)
	}
	return urls
}

func fetchPageLinks(targetURL string) ([]string, error) {
	client := createHTTPClient()
	req, err := http.NewRequest("GET", targetURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0")
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch page: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	return filterLinksByContent(extractLinks(string(body), targetURL), client), nil
}

func scrapeAndDownload(targetURL string, gs *GlobalStatus) {
	logInfo("scraping: %s", targetURL)
	links, err := fetchPageLinks(targetURL)
	if err != nil {
		logError("%v", err)
		return
	}
	if len(links) == 0 {
		logWarning("no matching links found")
		return
	}

	w := termWidth()
	fmt.Printf("\n%s Found %d links%s\n", colors["green"], len(links), colors["reset"])
	for i, l := range links {
		ext := filepath.Ext(l)
		name := truncateString(filepath.Base(strings.SplitN(l, "?", 2)[0]), w-15)
		fmt.Printf("  %s%3d.%s [%s] %s\n", colors["bold"], i+1, colors["reset"], ext, name)
	}

	indices := getUserSelection(len(links))
	if len(indices) == 0 {
		return
	}
	selected := make([]string, 0, len(indices))
	for _, idx := range indices {
		if idx >= 1 && idx <= len(links) {
			selected = append(selected, links[idx-1])
		}
	}
	startDownloads(selected)
}

func startDownloads(links []string) {
	links = removeDuplicateURLs(links)
	if len(links) == 0 {
		return
	}
	gs := NewGlobalStatus()
	for _, l := range links {
		name := filepath.Base(strings.SplitN(l, "?", 2)[0])
		if name == "" || name == "/" || name == "." {
			name = fmt.Sprintf("file_%d", time.Now().Unix())
		}
		var size int64
		if !isFTPURL(l) && !isHLSURL(l) && !isMagnetLink(l) {
			if _, s, err := fetchFileInfo(l, createHTTPClient()); err == nil {
				size = s
			}
		}
		gs.addFile(name, size)
	}

	sem := make(chan struct{}, maxParallel)
	var wg sync.WaitGroup
	go gs.reportAllFiles()

	for _, l := range links {
		wg.Add(1)
		sem <- struct{}{}
		go func(rawURL string) {
			defer wg.Done()
			defer func() { <-sem }()
			downloadSingle(rawURL, createHTTPClient(), gs)
		}(l)
	}
	wg.Wait()
	gs.closeDone()
}

func isFTPURL(u string) bool {
	return strings.HasPrefix(u, "ftp://") || strings.HasPrefix(u, "ftps://")
}

func extractLinks(htmlContent, baseURL string) []string {
	re := regexp.MustCompile(`(?i)href=["']([^"']+)["']`)
	matches := re.FindAllStringSubmatch(htmlContent, -1)
	seen := make(map[string]bool)
	var links []string
	for _, m := range matches {
		if len(m) < 2 {
			continue
		}
		href := strings.TrimSpace(m[1])
		if href == "" || href == "#" || strings.HasPrefix(href, "javascript:") || strings.HasPrefix(href, "mailto:") {
			continue
		}
		abs := toAbsoluteURL(href, baseURL)
		if !seen[abs] {
			seen[abs] = true
			links = append(links, abs)
		}
	}
	return links
}

func filterLinksByContent(links []string, _ *http.Client) []string {
	mediaExts := map[string]bool{
		".zip": true, ".rar": true, ".7z": true, ".tar": true, ".gz": true,
		".mp4": true, ".mkv": true, ".avi": true, ".mov": true, ".webm": true,
		".mp3": true, ".flac": true, ".wav": true, ".aac": true, ".ogg": true,
		".pdf": true, ".doc": true, ".docx": true, ".xls": true, ".xlsx": true,
		".iso": true, ".img": true, ".bin": true,
		".exe": true, ".msi": true, ".deb": true, ".apk": true, ".dmg": true,
		".m3u8": true, ".m3u": true,
		".torrent": true,
	}
	filter := extensionsFilter
	var allowed map[string]bool
	if filter != "" {
		allowed = make(map[string]bool)
		for _, e := range strings.Split(filter, ",") {
			e = strings.TrimSpace(strings.ToLower(e))
			if e != "" {
				if !strings.HasPrefix(e, ".") {
					e = "." + e
				}
				allowed[e] = true
			}
		}
	}
	var result []string
	for _, l := range links {
		base := strings.ToLower(filepath.Base(strings.SplitN(l, "?", 2)[0]))
		ext := filepath.Ext(base)
		if allowed != nil {
			if allowed[ext] {
				result = append(result, l)
			}
		} else if mediaExts[ext] {
			result = append(result, l)
		}
	}
	return result
}

func toAbsoluteURL(href, base string) string {
	if strings.HasPrefix(href, "http://") || strings.HasPrefix(href, "https://") ||
		strings.HasPrefix(href, "ftp://") || strings.HasPrefix(href, "ftps://") {
		return href
	}
	bu, err := url.Parse(base)
	if err != nil {
		return href
	}
	ref, err := url.Parse(href)
	if err != nil {
		return href
	}
	return bu.ResolveReference(ref).String()
}

func getUserSelection(maxCount int) []int {
	fmt.Printf("\n%sEnter numbers to download (e.g. 1,3,5-8 or 'all'): %s",
		colors["bold"], colors["reset"])
	scanner := bufio.NewScanner(os.Stdin)
	if !scanner.Scan() {
		return nil
	}
	input := strings.TrimSpace(scanner.Text())
	if strings.ToLower(input) == "all" {
		result := make([]int, maxCount)
		for i := range result {
			result[i] = i + 1
		}
		return result
	}
	var selected []int
	seen := make(map[int]bool)
	for _, part := range strings.Split(input, ",") {
		part = strings.TrimSpace(part)
		if strings.Contains(part, "-") {
			bounds := strings.SplitN(part, "-", 2)
			if len(bounds) == 2 {
				lo, err1 := strconv.Atoi(strings.TrimSpace(bounds[0]))
				hi, err2 := strconv.Atoi(strings.TrimSpace(bounds[1]))
				if err1 == nil && err2 == nil {
					for i := lo; i <= hi; i++ {
						if !seen[i] {
							selected = append(selected, i)
							seen[i] = true
						}
					}
				}
			}
		} else if n, err := strconv.Atoi(part); err == nil && !seen[n] {
			selected = append(selected, n)
			seen[n] = true
		}
	}
	return selected
}

func getAuthFromNetrc(host string) (string, string) {
	paths := []string{netrcFile}
	if netrcFile == "" {
		home, _ := os.UserHomeDir()
		paths = []string{filepath.Join(home, ".netrc"), filepath.Join(home, "_netrc")}
	}
	for _, p := range paths {
		if p == "" {
			continue
		}
		data, err := os.ReadFile(p)
		if err != nil {
			continue
		}
		lines := strings.Fields(string(data))
		for i := 0; i+1 < len(lines); i++ {
			if lines[i] == "machine" && i+1 < len(lines) && (lines[i+1] == host || strings.HasSuffix(host, "."+lines[i+1])) {
				var user, pass string
				for j := i + 2; j+1 < len(lines); j += 2 {
					switch lines[j] {
					case "login":
						user = lines[j+1]
					case "password":
						pass = lines[j+1]
					case "machine", "default":
						goto next
					}
				}
			next:
				return user, pass
			}
		}
	}
	return "", ""
}

func loadCookiesFromFile(filePath string) (string, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return "", err
	}
	var cookies []string
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		fields := strings.Split(line, "\t")
		if len(fields) >= 7 {
			cookies = append(cookies, fields[5]+"="+fields[6])
		} else if strings.Contains(line, "=") {
			cookies = append(cookies, line)
		}
	}
	return strings.Join(cookies, "; "), nil
}

func saveCookiesToFile(cookies []string, path string) {
	f, err := os.Create(path)
	if err != nil {
		return
	}
	defer f.Close()
	for _, c := range cookies {
		f.WriteString(c + "\n")
	}
}

func basicAuth(user, pass string) string {
	return base64.StdEncoding.EncodeToString([]byte(user + ":" + pass))
}

func pctOf(done, total int64) float64 {
	if total <= 0 {
		return 0
	}
	p := float64(done) / float64(total) * 100
	if p > 100 {
		return 100
	}
	return p
}

func Size4Human(n int64) string {
	if n <= 0 {
		return "0 B"
	}
	units := []string{"B", "KB", "MB", "GB", "TB", "PB"}
	f := float64(n)
	i := 0
	for f >= 1024 && i < len(units)-1 {
		f /= 1024
		i++
	}
	if i == 0 {
		return fmt.Sprintf("%d B", n)
	}
	return fmt.Sprintf("%.1f %s", f, units[i])
}

func formatDuration(secs float64) string {
	if secs < 0 || math.IsNaN(secs) || math.IsInf(secs, 0) {
		return "—"
	}
	s := int(secs)
	h := s / 3600
	m := (s % 3600) / 60
	s = s % 60
	if h > 0 {
		return fmt.Sprintf("%dh%02dm%02ds", h, m, s)
	}
	if m > 0 {
		return fmt.Sprintf("%dm%02ds", m, s)
	}
	return fmt.Sprintf("%ds", s)
}

func clampInt(v, min, max int) int {
	if v < min {
		return min
	}
	if v > max {
		return max
	}
	return v
}

func maxF64(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}

func truncateString(s string, n int) string {
	if len(s) <= n {
		return s
	}
	if n < 3 {
		return s[:n]
	}
	return s[:n-3] + "..."
}

func getFileName(rawURL string, resp *http.Response) string {
	if resp != nil {
		cd := resp.Header.Get("Content-Disposition")
		if cd != "" {
			re := regexp.MustCompile(`filename[^;=\n]*=(?:(['"])([^'"]+)\1|([^;\n]+))`)
			if m := re.FindStringSubmatch(cd); len(m) > 0 {
				name := strings.TrimSpace(m[2])
				if name == "" {
					name = strings.TrimSpace(m[3])
				}
				if name != "" {
					return sanitizeFileName(name)
				}
			}
		}
	}
	u, err := url.Parse(rawURL)
	if err == nil {
		base := filepath.Base(u.Path)
		if base != "" && base != "." && base != "/" {
			if idx := strings.Index(base, "?"); idx > 0 {
				base = base[:idx]
			}
			return base
		}
	}
	return fmt.Sprintf("download_%d", time.Now().Unix())
}

func die(msg string, err error) {
	fmt.Fprintf(os.Stderr, "%s%s: %v%s\n", colors["red"], msg, err, colors["reset"])
	os.Exit(1)
}

type headerSlice []string

func (h *headerSlice) String() string  { return strings.Join(*h, ", ") }
func (h *headerSlice) Set(v string) error { *h = append(*h, v); return nil }

func initFlags() {
	flag.IntVar(&numThreads, "t", 0, "Number of download threads (default: num CPUs)")
	flag.Var(&headers, "H", "Custom HTTP header (repeatable, format: 'Name: value')")
	flag.StringVar(&cookie, "b", "", "Cookie string (Name=value; ...)")
	flag.StringVar(&outDir, "o", ".", "Output directory")
	flag.IntVar(&retries, "r", 5, "Max retries per segment")
	flag.IntVar(&timeoutSec, "timeout", 30, "HTTP timeout in seconds")
	flag.IntVar(&maxParallel, "p", 2, "Max parallel downloads")
	flag.BoolVar(&saveSession, "save-session", false, "Save session on exit")
	flag.StringVar(&fileList, "f", "", "File containing list of URLs")
	flag.BoolVar(&verbose, "v", false, "Verbose logging")
	flag.StringVar(&proxyAddr, "proxy", "", "Proxy URL (http://, https://, socks4://, socks5://)")
	flag.StringVar(&protocol, "protocol", "auto", "Protocol (auto, http, ftp, ftps, sftp, webdav)")
	flag.StringVar(&ftpUser, "ftp-user", "anonymous", "FTP username")
	flag.StringVar(&ftpPass, "ftp-pass", "anonymous@example.com", "FTP password")
	flag.BoolVar(&ftpMultiPart, "ftp-multipart", false, "Enable FTP multi-part download")
	flag.IntVar(&ftpParts, "ftp-parts", 0, "FTP parts count (default: num threads)")
	flag.StringVar(&scrapeURL, "scrape", "", "URL to scrape for downloadable links")
	flag.StringVar(&extensionsFilter, "ext", "", "Comma-separated extensions to filter (e.g. mp4,zip)")
	flag.Int64Var(&maxSpeed, "limit", 0, "Speed limit bytes/sec (0=unlimited)")
	flag.Int64Var(&diskCacheSize, "cache", 0, "Disk cache size in bytes")
	flag.BoolVar(&enableGzip, "gzip", false, "Enable gzip decoding")
	flag.StringVar(&cookieFile, "cookie-file", "", "Cookie file (Netscape format)")
	flag.StringVar(&saveCookieFile, "save-cookie", "", "Save cookies to file after download")
	flag.StringVar(&netrcFile, "netrc", "", "Path to .netrc file for auth")
	flag.BoolVar(&checkIntegrity, "check", false, "Verify file integrity after download")
	flag.StringVar(&checkSha256, "sha256", "", "Expected SHA-256 hash")
	flag.StringVar(&checkMd5, "md5", "", "Expected MD5 hash")
	flag.StringVar(&checkSha1, "sha1", "", "Expected SHA-1 hash")
	flag.StringVar(&parameterizedURL, "url-pattern", "", "URL pattern with {} placeholder")
	flag.IntVar(&parameterizedStart, "url-start", 1, "Pattern start value")
	flag.IntVar(&parameterizedEnd, "url-end", 10, "Pattern end value")
	flag.IntVar(&parameterizedStep, "url-step", 1, "Pattern step value")
	flag.BoolVar(&daemonMode, "daemon", false, "Run as daemon")
	flag.StringVar(&pidFile, "pid", "", "PID file path (daemon mode)")
	flag.StringVar(&sshUser, "ssh-user", "", "SSH username (SFTP)")
	flag.StringVar(&sshPass, "ssh-pass", "", "SSH password (SFTP)")
	flag.StringVar(&sshKeyFile, "ssh-key", "", "SSH private key file")
	flag.StringVar(&sfftpKeyPass, "ssh-key-pass", "", "SSH key passphrase")
	flag.StringVar(&metalinkFile, "metalink", "", "Metalink (.meta4 or .metalink) file")
	flag.BoolVar(&rpcEnabled, "rpc", false, "Enable JSON-RPC server")
	flag.StringVar(&rpcAddr, "rpc-addr", "localhost:6800", "JSON-RPC listen address")
	flag.BoolVar(&webSocketRPC, "ws-rpc", false, "Enable WebSocket RPC")
	flag.StringVar(&captureProxy, "capture-proxy", "", "Capture proxy port (e.g. :8085)")
	flag.StringVar(&captureTypes, "capture-types", "video,music,archive", "File types to capture")
	flag.StringVar(&captureExts, "capture-exts", "", "Custom extensions to capture")
	flag.BoolVar(&captureAuto, "capture-auto", false, "Auto-download captured files")
	flag.StringVar(&captureOutput, "capture-output", "captured", "Capture output directory")
	flag.IntVar(&captureConfidence, "capture-confidence", 30, "Capture confidence (0-100)")
	flag.Int64Var(&captureMinSize, "capture-min-size", 1024, "Min capture file size")
	flag.Int64Var(&captureMaxSize, "capture-max-size", 0, "Max capture file size (0=∞)")
	flag.StringVar(&captureSaveFile, "capture-save", "captured_links.txt", "Save captured links")
	flag.Var(&captureHeaders, "capture-header", "Capture proxy custom header (repeatable)")
	flag.StringVar(&captureCookie, "capture-cookie", "", "Capture proxy cookie")
	flag.StringVar(&downloadFromJson, "download-json", "", "Download from captured JSON file")

	flag.StringVar(&notifyTelegram, "notify-telegram", "", "Telegram chat ID for notifications")
	flag.StringVar(&notifyTelegramBot, "notify-telegram-bot", "", "Telegram bot token")
	flag.StringVar(&notifyDiscord, "notify-discord", "", "Discord webhook URL")
	flag.BoolVar(&notifyDesktop, "notify-desktop", false, "Enable desktop notifications")

	flag.StringVar(&scheduleFrom, "schedule-from", "", "Download window start (HH:MM)")
	flag.StringVar(&scheduleTo, "schedule-to", "", "Download window end (HH:MM)")

	flag.BoolVar(&postExtract, "post-extract", false, "Auto-extract archive after download")
	flag.StringVar(&postMove, "post-move", "", "Move file to this directory after download")
	flag.StringVar(&postRename, "post-rename", "", "Rename pattern after download ({name}, {time})")

	flag.StringVar(&mirrorURLs, "mirrors", "", "Comma-separated mirror URLs")
	flag.BoolVar(&autoMirror, "auto-mirror", false, "Auto-select fastest mirror via speed test")

	flag.StringVar(&queueFile, "queue", "", "Queue file with URLs and priorities")
	flag.StringVar(&queuePriority, "priority", "0", "Download priority for this job (higher = first)")

	flag.StringVar(&hlsURL, "hls", "", "HLS/M3U8 stream URL to download")

	flag.StringVar(&webdavURL, "webdav", "", "WebDAV server URL")
	flag.StringVar(&webdavUser, "webdav-user", "", "WebDAV username")
	flag.StringVar(&webdavPass, "webdav-pass", "", "WebDAV password")

	flag.StringVar(&magnetLink, "magnet", "", "Magnet link to download (web seeds)")
}

func RunHAD() {
	if daemonMode {
		if err := runDaemon(); err != nil {
			die("daemon:", err)
		}
	}

	flag.Usage = showUsage
	initFlags()
	flag.Parse()
	logger.SetVerbose(verbose)

	if magnetLink != "" {
		gs := NewGlobalStatus()
		downloadMagnet(magnetLink, gs)
		return
	}

	if webdavURL != "" {
		client := createHTTPClient()
		gs := NewGlobalStatus()
		wd := NewWebDAVDownloader(webdavURL, webdavUser, webdavPass, client, gs)
		go gs.reportAllFiles()
		wd.DownloadAll("/")
		gs.closeDone()
		return
	}

	if hlsURL != "" {
		os.MkdirAll(outDir, 0755)
		client := createHTTPClient()
		gs := NewGlobalStatus()
		notifier := NewNotifier()
		fileName := getFileName(hlsURL, nil)
		outPath := filepath.Join(outDir, fileName)
		hls := NewHLSDownloader(hlsURL, outPath, fileName, client, gs, notifier)
		go gs.reportAllFiles()
		if err := hls.Download(); err != nil {
			logError("HLS: %v", err)
		}
		gs.closeDone()
		return
	}

	if queueFile != "" {
		q := NewDownloadQueue()
		if err := q.LoadFromFile(queueFile); err != nil {
			die("load queue:", err)
		}
		logInfo("queue loaded: %d items", q.Len())
		var urls []string
		for {
			item, ok := q.Pop()
			if !ok {
				break
			}
			urls = append(urls, item.URL)
		}
		startDownloads(urls)
		return
	}

	if downloadFromJson != "" {
		if outDir == "" {
			outDir = "captured_downloads"
		}
		if err := DownloadFromCapturedJSON(downloadFromJson, maxParallel); err != nil {
			logError("%v", err)
			os.Exit(1)
		}
		return
	}

	if captureProxy != "" {
		port := captureProxy
		if !strings.HasPrefix(port, ":") {
			port = ":" + port
		}
		if err := InstallCertificate(); err != nil {
			fmt.Printf("%s⚠ cert install failed: %v%s\n", colors["yellow"], err, colors["reset"])
			ShowManualInstructions()
		} else {
			fmt.Printf("%s✅ CA certificate installed%s\n", colors["green"], colors["reset"])
		}

		var fileTypes []FileType
		for _, t := range strings.Split(captureTypes, ",") {
			switch strings.ToLower(strings.TrimSpace(t)) {
			case "video":
				fileTypes = append(fileTypes, TypeVideo)
			case "music":
				fileTypes = append(fileTypes, TypeMusic)
			case "image":
				fileTypes = append(fileTypes, TypeImage)
			case "document":
				fileTypes = append(fileTypes, TypeDocument)
			case "archive":
				fileTypes = append(fileTypes, TypeArchive)
			case "all":
				fileTypes = append(fileTypes, TypeAll)
			}
		}

		var customExts []string
		for _, e := range strings.Split(captureExts, ",") {
			if e = strings.TrimSpace(e); e != "" {
				customExts = append(customExts, e)
			}
		}

		hdrs := map[string]string{}
		for _, h := range captureHeaders {
			if parts := strings.SplitN(h, ":", 2); len(parts) == 2 {
				hdrs[strings.TrimSpace(parts[0])] = strings.TrimSpace(parts[1])
			}
		}

		cfg := &CaptureConfig{
			Port:             port,
			FileTypes:        fileTypes,
			CustomExtensions: customExts,
			Headers:          hdrs,
			Cookie:           captureCookie,
			AutoDownload:     captureAuto,
			OutputDir:        captureOutput,
			MinFileSize:      captureMinSize,
			MaxFileSize:      captureMaxSize,
			ConfidenceLevel:  captureConfidence,
			SaveToFile:       captureSaveFile,
			Verbose:          verbose,
			CaptureBody:      true,
		}
		p := NewCaptureProxy(cfg)
		if err := p.Start(); err != nil {
			log.Fatal(err)
		}
		return
	}

	if installCert {
		if err := InstallCertificate(); err != nil {
			fmt.Printf("%sCertificate install failed: %v%s\n", colors["red"], err, colors["reset"])
			ShowManualInstructions()
			os.Exit(1)
		}
		fmt.Println(colors["green"] + "Certificate installed. Run: had -capture-proxy :8085" + colors["reset"])
		return
	}

	if metalinkFile != "" {
		gs := NewGlobalStatus()
		downloadMetalink(metalinkFile, gs)
		go gs.reportAllFiles()
		<-gs.doneCh
		return
	}

	if rpcEnabled {
		gs := NewGlobalStatus()
		srv := NewRPCServer(gs)
		if err := srv.Start(rpcAddr); err != nil {
			logError("RPC: %v", err)
		}
		logInfo("RPC running on %s", rpcAddr)
		select {}
	}

	if parameterizedURL != "" {
		urls := generateParameterizedURLs()
		logInfo("%d parameterized URLs", len(urls))
		startDownloads(urls)
		return
	}

	if scrapeURL != "" {
		scrapeAndDownload(scrapeURL, NewGlobalStatus())
		return
	}

	var args []string
	if fileList != "" {
		data, err := os.ReadFile(fileList)
		if err != nil {
			die("read file list:", err)
		}
		for _, line := range strings.Split(string(data), "\n") {
			if line = strings.TrimSpace(line); line != "" && !strings.HasPrefix(line, "#") {
				args = append(args, line)
			}
		}
	} else {
		args = flag.Args()
	}

	args = removeDuplicateURLs(args)
	if len(args) == 0 {
		logError("no unique URLs after removing duplicates")
		return
	}

	if len(args) == 1 && (strings.HasSuffix(args[0], ".json") || strings.HasSuffix(args[0], ".had")) {
		resumeFromSession(args[0], nil)
		return
	}

	client := createHTTPClient()
	gs := NewGlobalStatus()

	w := termWidth()
	heavy := strings.Repeat("═", w)
	fmt.Printf("%s%s%s\n", colors["cyan"], heavy, colors["reset"])
	fmt.Printf("%s  FETCHING FILE METADATA%s\n", colors["bold"], colors["reset"])
	fmt.Printf("%s%s%s\n", colors["cyan"], heavy, colors["reset"])

	validURLs := make([]string, 0, len(args))
	for _, u := range args {
		if isMagnetLink(u) {
			info, _ := parseMagnetLink(u)
			name := "magnet"
			if info != nil && info.DisplayName != "" {
				name = info.DisplayName
			}
			gs.addFile(name, 0)
			validURLs = append(validURLs, u)
			fmt.Printf("  %s•%s %s %s(MAGNET)%s\n", colors["green"], colors["reset"],
				name, colors["blue"], colors["reset"])
		} else if isFTPURL(u) {
			pu, _ := url.Parse(u)
			name := filepath.Base(pu.Path)
			if name == "" || name == "/" {
				name = fmt.Sprintf("ftp_%d", time.Now().Unix())
			}
			gs.addFile(name, -1)
			validURLs = append(validURLs, u)
			fmt.Printf("  %s•%s %s %s(FTP)%s\n", colors["green"], colors["reset"],
				name, colors["yellow"], colors["reset"])
		} else if strings.HasPrefix(u, "sftp://") || protocol == "sftp" {
			pu, _ := url.Parse(u)
			name := filepath.Base(pu.Path)
			gs.addFile(name, -1)
			validURLs = append(validURLs, u)
			fmt.Printf("  %s•%s %s %s(SFTP)%s\n", colors["green"], colors["reset"],
				name, colors["yellow"], colors["reset"])
		} else if isHLSURL(u) {
			name := getFileName(u, nil)
			gs.addFile(name, -1)
			validURLs = append(validURLs, u)
			fmt.Printf("  %s•%s %s %s(HLS)%s\n", colors["green"], colors["reset"],
				name, colors["blue"], colors["reset"])
		} else {
			meta, err := FetchFileMetadata(u, client)
			if err != nil {
				logWarning("skip %s: %v", u, err)
				continue
			}
			gs.addFile(meta.FileName, meta.Size)
			validURLs = append(validURLs, u)
			resumable := ""
			if meta.Resumable {
				resumable = " ✓resume"
			}
			checksumHint := ""
			if meta.Checksum != "" {
				checksumHint = " 🔒checksum"
			}
			fmt.Printf("  %s•%s %s (%s)%s%s\n", colors["green"], colors["reset"],
				meta.FileName, meta.SizeHuman, resumable, checksumHint)
		}
	}

	if len(validURLs) == 0 {
		logError("no valid URLs")
		return
	}

	fmt.Print("\033[2J\033[H")

	sem := make(chan struct{}, maxParallel)
	var wg sync.WaitGroup
	go gs.reportAllFiles()

	for _, u := range validURLs {
		wg.Add(1)
		sem <- struct{}{}
		go func(rawURL string) {
			defer wg.Done()
			defer func() { <-sem }()
			if isMagnetLink(rawURL) {
				downloadMagnet(rawURL, gs)
			} else if isFTPURL(rawURL) {
				downloadFTP(rawURL, gs)
			} else if strings.HasPrefix(rawURL, "sftp://") || protocol == "sftp" {
				downloadSFTP(rawURL, gs)
			} else {
				downloadSingle(rawURL, createHTTPClient(), gs)
			}
		}(u)
	}

	wg.Wait()
	gs.closeDone()
	time.Sleep(time.Second)
}

func showUsage() {
	w := termWidth()
	sep := strings.Repeat("─", w)
	fmt.Printf("%s%s%s\n", colors["cyan"], sep, colors["reset"])
	fmt.Printf("%s  had — Hyper Advanced Downloader%s\n", colors["bold"], colors["reset"])
	fmt.Printf("%s%s%s\n", colors["cyan"], sep, colors["reset"])
	fmt.Println("\nUSAGE:")
	fmt.Println("  had [OPTIONS] <url> [url ...]")
	fmt.Println("  had -f <file>         read URLs from file")
	fmt.Println("  had -hls <url>        download HLS/M3U8 stream")
	fmt.Println("  had -magnet <link>    download via magnet link (web seeds)")
	fmt.Println("  had -webdav <url>     download from WebDAV server")
	fmt.Println("  had -queue <file>     download from priority queue file")
	fmt.Println("  had <session.had>     resume interrupted download")
	fmt.Printf("\n%s%s%s\n", colors["cyan"], sep, colors["reset"])
	fmt.Println("\nOPTIONS:")
	flag.PrintDefaults()
}