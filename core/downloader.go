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
)

func init() {
	_ = colorable.NewColorable(os.Stdout)
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

	scheduleFrom    string
	scheduleTo      string

	postExtract     bool
	postMove        string
	postRename      string

	mirrorURLs      string
	autoMirror      bool

	queueFile       string
	queuePriority   string

	hlsURL          string
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

func (l *Logger) SetVerbose(v bool) { l.verbose = v }

func (l *Logger) Info(format string, args ...interface{}) {
	if !l.verbose {
		return
	}
	l.mu.Lock()
	fmt.Printf(colors["cyan"]+"[INFO] "+colors["reset"]+format+"\n", args...)
	l.mu.Unlock()
}

func (l *Logger) Error(format string, args ...interface{}) {
	l.mu.Lock()
	fmt.Printf(colors["red"]+"[ERROR] "+colors["reset"]+format+"\n", args...)
	l.mu.Unlock()
}

func (l *Logger) Warning(format string, args ...interface{}) {
	l.mu.Lock()
	fmt.Printf(colors["yellow"]+"[WARN] "+colors["reset"]+format+"\n", args...)
	l.mu.Unlock()
}

func (l *Logger) Debug(format string, args ...interface{}) {
	if !l.verbose {
		return
	}
	l.mu.Lock()
	fmt.Printf(colors["gray"]+"[DEBUG] "+colors["reset"]+format+"\n", args...)
	l.mu.Unlock()
}

func (l *Logger) Success(format string, args ...interface{}) {
	l.mu.Lock()
	fmt.Printf(colors["green"]+"[✓] "+colors["reset"]+format+"\n", args...)
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
	completedOnce sync.Once
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
	URL      string
	Path     string
	Size     int64
	Ranges   [][2]int64
	FileName string
	Progress []int64
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

			if verbose && f.TotalThreads > 0 && len(f.ThreadProg) > 0 {
				segBase := int64(0)
				if f.Total > 0 && f.TotalThreads > 0 {
					segBase = f.Total / int64(f.TotalThreads)
				}
				buf.WriteString(fmt.Sprintf("  %s└ threads [%d/%d]:%s\n",
					colors["gray"], f.DoneThreads, f.TotalThreads, colors["reset"]))
				lines++
				for ti, tp := range f.ThreadProg {
					seg := segBase
					if ti == f.TotalThreads-1 && f.Total > 0 {
						seg = f.Total - segBase*int64(ti)
					}
					tpct := pctOf(tp, seg)
					tf := clampInt(int(tpct/100*10), 0, 10)
					tbar := colors["green"] + strings.Repeat("█", tf) +
						colors["reset"] + strings.Repeat("░", 10-tf)
					icon2 := "⬇"
					if tpct >= 100 {
						icon2 = "✅"
					} else if tp == 0 {
						icon2 = "⏳"
					}
					buf.WriteString(fmt.Sprintf("  %s  T%d:%s [%s] %.0f%%%s\n",
						colors["gray"], ti+1, icon2, tbar, tpct, colors["reset"]))
					lines++
				}
			}

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

type PostProcessor struct {
	extract bool
	moveDir string
	rename  string
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
	url       string
	file      *os.File
	wb        *writeBuffer
	headers   http.Header
	client    *http.Client
	size      int64
	segments  []*segment
	path      string
	fileName  string
	global    *GlobalStatus
	totalDone *int64
	retries   int
	ctx       context.Context
	cancel    context.CancelFunc
	ab        *AdaptiveBuffer
	limiter   *speedLimiter
	doneCh    chan struct{}
	mirrors   []string
	mirrorIdx int64
	schedule  *BandwidthSchedule
	notifier  *Notifier
	postProc  *PostProcessor
}

func newDownloader(rawURL, outPath, fileName string, size int64, segs []*segment,
	client *http.Client, f *os.File, gs *GlobalStatus) *Downloader {
	ctx, cancel := context.WithCancel(context.Background())
	notifier := NewNotifier()
	return &Downloader{
		url:       rawURL,
		file:      f,
		wb:        newWriteBuffer(f),
		headers:   defaultHeaders(),
		client:    client,
		size:      size,
		segments:  segs,
		path:      outPath,
		fileName:  fileName,
		global:    gs,
		totalDone: &gs.totalDone,
		retries:   retries,
		ctx:       ctx,
		cancel:    cancel,
		ab:        newAdaptiveBuffer(),
		limiter:   newSpeedLimiter(maxSpeed),
		doneCh:    make(chan struct{}),
		mirrors:   []string{rawURL},
		schedule:  parseBandwidthSchedule(scheduleFrom, scheduleTo),
		notifier:  notifier,
		postProc:  NewPostProcessor(notifier),
	}
}

func (dl *Downloader) setMirrors(mirrors []string) {
	dl.mirrors = mirrors
}

func (dl *Downloader) nextMirror() string {
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

		err := dl.doRequest(idx, currentStart, seg.end, segSize, targetURL)
		if err == nil {
			return nil
		}
		if err == context.Canceled {
			return nil
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
	dl.cancel()

	if err := dl.wb.Close(); err != nil {
		logError("write-buffer flush: %v", err)
	}

	close(errCh)
	var hadErr bool
	for range errCh {
		hadErr = true
	}

	os.Remove(dl.path + ".progress")

	if hadErr && dl.global != nil {
		dl.global.markError(dl.fileName)
		if dl.notifier.IsEnabled() {
			dl.notifier.Send("❌ Download Failed", fmt.Sprintf("File: %s", dl.fileName))
		}
	} else {
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
		URL: dl.url, Path: dl.path, Size: dl.size,
		Ranges: rngs, FileName: dl.fileName, Progress: prog,
	}
	fname := dl.path + ".json"
	f, err := os.Create(fname)
	if err != nil {
		logError("save session: %v", err)
		return
	}
	defer f.Close()
	json.NewEncoder(f).Encode(s)
	logInfo("Session saved → %s", fname)
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
	if data, err := os.ReadFile(outPath + ".progress"); err == nil {
		var pd struct {
			Progress []int64
			Ranges   [][2]int64
		}
		if json.Unmarshal(data, &pd) == nil && len(pd.Ranges) > 0 {
			existingSegs = resumeSegments(pd.Ranges, pd.Progress)
			logInfo("resuming %s (%d segments)", fileName, len(existingSegs))
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

	if autoMirror && mirrorURLs != "" {
		extras := strings.Split(mirrorURLs, ",")
		all := buildMirrorList(rawURL, extras)
		sorted := probeMirrors(all, client)
		dl.setMirrors(sorted)
		if len(sorted) > 0 && sorted[0] != rawURL {
			logInfo("fastest mirror: %s", sorted[0])
			dl.url = sorted[0]
		}
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
				dl.saveProgress()
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

func scrapeAndDownload(targetURL string, gs *GlobalStatus) {
	logInfo("scraping: %s", targetURL)
	client := createHTTPClient()
	req, _ := http.NewRequest("GET", targetURL, nil)
	req.Header.Set("User-Agent", "Mozilla/5.0")
	resp, err := client.Do(req)
	if err != nil {
		logError("fetch page: %v", err)
		return
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)

	links := filterLinksByContent(extractLinks(string(body), targetURL), client)
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
		if !isFTPURL(l) && !isHLSURL(l) {
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
		go func(u string) {
			defer wg.Done()
			defer func() { <-sem }()
			switch {
			case isFTPURL(u):
				downloadFTP(u, gs)
			case strings.HasPrefix(u, "sftp://"):
				downloadSFTP(u, gs)
			default:
				downloadSingle(u, createHTTPClient(), gs)
			}
		}(l)
	}

	wg.Wait()
	time.Sleep(2 * time.Second)
	gs.closeDone()
}

func isFTPURL(u string) bool {
	return strings.HasPrefix(u, "ftp://") || strings.HasPrefix(u, "ftps://") ||
		protocol == "ftp" || protocol == "ftps"
}

func extractLinks(html, baseURL string) []string {
	seen := map[string]bool{}
	var links []string
	patterns := []string{
		`href="([^"]+)"`, `href='([^']+)'`,
		`src="([^"]+)"`, `src='([^']+)'`,
		`data-url="([^"]+)"`, `data-url='([^']+)'`,
		`data-file="([^"]+)"`, `data-file='([^']+)'`,
	}
	for _, pat := range patterns {
		re := regexp.MustCompile(pat)
		for _, m := range re.FindAllStringSubmatch(html, -1) {
			if len(m) < 2 {
				continue
			}
			link := m[1]
			if strings.HasPrefix(link, "#") || strings.HasPrefix(link, "javascript:") ||
				strings.HasPrefix(link, "mailto:") || link == "" {
				continue
			}
			abs := toAbsoluteURL(link, baseURL)
			if isDownloadableFile(abs) && !seen[abs] {
				seen[abs] = true
				links = append(links, abs)
			}
		}
	}
	return links
}

func filterLinksByContent(links []string, _ *http.Client) []string {
	if extensionsFilter == "" {
		return links
	}
	exts := strings.Split(extensionsFilter, ",")
	for i, e := range exts {
		e = strings.TrimSpace(e)
		if !strings.HasPrefix(e, ".") {
			e = "." + e
		}
		exts[i] = e
	}
	var out []string
	for _, l := range links {
		ll := strings.ToLower(l)
		for _, e := range exts {
			if strings.HasSuffix(ll, e) {
				out = append(out, l)
				break
			}
		}
	}
	return out
}

func toAbsoluteURL(href, base string) string {
	if strings.HasPrefix(href, "http://") || strings.HasPrefix(href, "https://") ||
		strings.HasPrefix(href, "ftp://") || strings.HasPrefix(href, "sftp://") {
		return href
	}
	b, err := url.Parse(base)
	if err != nil {
		return href
	}
	if strings.HasPrefix(href, "//") {
		return b.Scheme + ":" + href
	}
	r, err := url.Parse(href)
	if err != nil {
		return href
	}
	return b.ResolveReference(r).String()
}

func isDownloadableFile(rawURL string) bool {
	if extensionsFilter != "" {
		return hasAllowedExtension(rawURL)
	}
	downloadableExts := []string{
		".zip", ".rar", ".7z", ".tar", ".gz", ".bz2", ".xz",
		".pdf", ".doc", ".docx", ".xls", ".xlsx", ".ppt", ".pptx",
		".jpg", ".jpeg", ".png", ".gif", ".bmp", ".svg", ".webp",
		".mp4", ".mkv", ".avi", ".mov", ".wmv", ".flv", ".webm",
		".mp3", ".wav", ".flac", ".aac", ".ogg", ".m4a",
		".exe", ".msi", ".deb", ".rpm", ".apk",
		".iso", ".img", ".bin",
		".txt", ".csv", ".json", ".xml",
		".m3u8", ".m3u", ".ts",
	}
	ll := strings.ToLower(rawURL)
	for _, ext := range downloadableExts {
		if strings.HasSuffix(ll, ext) {
			return true
		}
	}
	return strings.Contains(ll, "/download") || strings.Contains(ll, "/file") ||
		strings.Contains(ll, "/get") || strings.Contains(ll, "/hls/")
}

func hasAllowedExtension(rawURL string) bool {
	if extensionsFilter == "" {
		return true
	}
	ll := strings.ToLower(rawURL)
	for _, e := range strings.Split(extensionsFilter, ",") {
		e = strings.TrimSpace(e)
		if !strings.HasPrefix(e, ".") {
			e = "." + e
		}
		if strings.HasSuffix(ll, e) {
			return true
		}
	}
	return false
}

func getUserSelection(maxCount int) []int {
	r := bufio.NewReader(os.Stdin)
	for {
		fmt.Printf("\n%sSelect links (e.g. 1-4,7,9): %s", colors["yellow"], colors["reset"])
		input, _ := r.ReadString('\n')
		input = strings.TrimSpace(input)
		if input == "" {
			return nil
		}
		if sel := parseSelection(input, maxCount); len(sel) > 0 {
			return sel
		}
		fmt.Printf("%sInvalid format. Try: 1-4,7 or 1 2 3%s\n", colors["red"], colors["reset"])
	}
}

func parseSelection(input string, maxCount int) []int {
	selected := map[int]bool{}
	input = strings.ReplaceAll(input, " ", ",")
	for _, part := range strings.Split(input, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		if strings.Contains(part, "-") {
			pp := strings.SplitN(part, "-", 2)
			s, e1 := strconv.Atoi(strings.TrimSpace(pp[0]))
			e, e2 := strconv.Atoi(strings.TrimSpace(pp[1]))
			if e1 == nil && e2 == nil && s <= e {
				for i := s; i <= e && i <= maxCount; i++ {
					if i >= 1 {
						selected[i] = true
					}
				}
			}
		} else if n, err := strconv.Atoi(part); err == nil && n >= 1 && n <= maxCount {
			selected[n] = true
		}
	}
	result := make([]int, 0, len(selected))
	for i := 1; i <= maxCount; i++ {
		if selected[i] {
			result = append(result, i)
		}
	}
	return result
}

type NetrcEntry struct{ Machine, Login, Password string }

func loadNetrc() map[string]*NetrcEntry {
	path := netrcFile
	if path == "" {
		home, _ := os.UserHomeDir()
		path = filepath.Join(home, ".netrc")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	entries := map[string]*NetrcEntry{}
	var cur *NetrcEntry
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		fields := strings.Fields(line)
		for i := 0; i < len(fields); i++ {
			switch fields[i] {
			case "machine":
				if cur != nil && cur.Machine != "" {
					entries[cur.Machine] = cur
				}
				if i+1 < len(fields) {
					cur = &NetrcEntry{Machine: fields[i+1]}
					i++
				}
			case "login":
				if cur != nil && i+1 < len(fields) {
					cur.Login = fields[i+1]
					i++
				}
			case "password":
				if cur != nil && i+1 < len(fields) {
					cur.Password = fields[i+1]
					i++
				}
			}
		}
	}
	if cur != nil && cur.Machine != "" {
		entries[cur.Machine] = cur
	}
	return entries
}

func getAuthFromNetrc(host string) (string, string) {
	entries := loadNetrc()
	if entries == nil {
		return "", ""
	}
	h := strings.Split(host, ":")[0]
	if e, ok := entries[h]; ok {
		return e.Login, e.Password
	}
	if e, ok := entries["default"]; ok {
		return e.Login, e.Password
	}
	return "", ""
}

func loadCookiesFromFile(filePath string) (string, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return "", err
	}
	m := map[string]string{}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) >= 7 {
			m[fields[5]] = fields[6]
		}
	}
	parts := make([]string, 0, len(m))
	for k, v := range m {
		parts = append(parts, k+"="+v)
	}
	return strings.Join(parts, "; "), nil
}

func saveCookiesToFile(cookies []string, filePath string) error {
	f, err := os.Create(filePath)
	if err != nil {
		return err
	}
	defer f.Close()
	fmt.Fprintln(f, "# Netscape HTTP Cookie File")
	for _, c := range cookies {
		fmt.Fprintln(f, c)
	}
	return nil
}

func Size4Human(b int64) string {
	if b <= 0 {
		return "0B"
	}
	if b < 1024 {
		return fmt.Sprintf("%dB", b)
	}
	exp := int(math.Log(float64(b)) / math.Log(1024))
	if exp > 4 {
		exp = 4
	}
	val := float64(b) / math.Pow(1024, float64(exp))
	return fmt.Sprintf("%.2f%s", val, []string{"B", "KB", "MB", "GB", "TB"}[exp])
}

func getFileName(rawURL string, resp *http.Response) string {
	if resp != nil {
		if cd := resp.Header.Get("Content-Disposition"); strings.Contains(cd, "filename=") {
			idx := strings.Index(cd, "filename=") + 9
			name := strings.Trim(cd[idx:], "\"")
			if sc := strings.Index(name, ";"); sc >= 0 {
				name = name[:sc]
			}
			name = strings.TrimSpace(name)
			if name != "" {
				return name
			}
		}
	}
	name := filepath.Base(strings.SplitN(rawURL, "?", 2)[0])
	if name == "" || name == "/" || name == "." {
		return fmt.Sprintf("file_%d", time.Now().Unix())
	}
	return name
}

func basicAuth(u, p string) string {
	return base64.StdEncoding.EncodeToString([]byte(u + ":" + p))
}

func SetColor(c, t string) string {
	return fmt.Sprintf("%s%s%s", colors[c], t, colors["reset"])
}

func die(a ...interface{}) {
	fmt.Fprintln(os.Stderr, "FATAL:", fmt.Sprint(a...))
	os.Exit(1)
}

func pctOf(done, total int64) float64 {
	if total <= 0 {
		return 0
	}
	p := float64(done) * 100 / float64(total)
	if p > 100 {
		return 100
	}
	return p
}

func maxF64(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}

func clampInt(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

func formatDuration(seconds float64) string {
	s := int(seconds)
	switch {
	case s < 60:
		return fmt.Sprintf("%ds", s)
	case s < 3600:
		return fmt.Sprintf("%dm%ds", s/60, s%60)
	default:
		return fmt.Sprintf("%dh%dm", s/3600, s%3600/60)
	}
}

func truncateString(s string, n int) string {
	if n <= 0 {
		return ""
	}
	if len(s) <= n {
		return s
	}
	if n <= 3 {
		return s[:n]
	}
	return s[:n-3] + "..."
}

func progressBarBeautiful(pct, length int) string {
	if pct < 0 {
		pct = 0
	}
	if pct > 100 {
		pct = 100
	}
	filled := length * pct / 100
	return fmt.Sprintf("[%s%s%s%s] %6.2f%%",
		colors["green"], strings.Repeat("█", filled),
		colors["reset"], strings.Repeat("░", length-filled),
		float64(pct))
}

type headerSlice []string

func (hs *headerSlice) String() string { return strings.Join(*hs, ", ") }
func (hs *headerSlice) Set(v string) error {
	if !strings.Contains(v, ":") {
		return fmt.Errorf("invalid header (missing colon): %s", v)
	}
	*hs = append(*hs, v)
	return nil
}

func initFlags() {
	flag.IntVar(&numThreads, "t", runtime.NumCPU(), "Parallel download threads per file")
	flag.Var(&headers, "H", "Custom HTTP header (repeatable). Format: Key: Value")
	flag.StringVar(&cookie, "c", "", "Cookie header value")
	flag.StringVar(&outDir, "o", ".", "Output directory")
	flag.IntVar(&retries, "r", 5, "Retries per segment")
	flag.IntVar(&timeoutSec, "timeout", 30, "Connection timeout (seconds)")
	flag.IntVar(&maxParallel, "u", 2, "Max simultaneous file downloads")
	flag.BoolVar(&saveSession, "save-session", true, "Save session JSON on interrupt")
	flag.StringVar(&fileList, "f", "", "File containing URLs (one per line)")
	flag.BoolVar(&verbose, "v", false, "Verbose mode (per-thread progress)")
	flag.StringVar(&proxyAddr, "proxy", "", "Proxy: socks4://, socks5://, http://")
	flag.StringVar(&protocol, "protocol", "auto", "Protocol: auto|http|https|ftp|ftps|sftp")
	flag.StringVar(&ftpUser, "ftp-user", "anonymous", "FTP/SFTP username")
	flag.StringVar(&ftpPass, "ftp-pass", "anonymous@example.com", "FTP/SFTP password")
	flag.BoolVar(&ftpMultiPart, "ftp-multipart", true, "Multi-part FTP download")
	flag.IntVar(&ftpParts, "ftp-parts", 0, "FTP part count (0 = auto)")
	flag.StringVar(&scrapeURL, "scrape", "", "URL to scrape for download links")
	flag.StringVar(&extensionsFilter, "ex", "", "Extension filter (e.g. .mp4,.zip)")
	flag.Int64Var(&maxSpeed, "max-speed", 0, "Speed cap in bytes/sec (0 = unlimited)")
	flag.Int64Var(&diskCacheSize, "disk-cache", 32*1024*1024, "Write-buffer size (bytes)")
	flag.BoolVar(&enableGzip, "gzip", true, "Enable gzip/deflate")
	flag.StringVar(&cookieFile, "load-cookies", "", "Netscape cookie file to load")
	flag.StringVar(&saveCookieFile, "save-cookies", "", "Save cookies to file after download")
	flag.StringVar(&netrcFile, "netrc", "", ".netrc authentication file")
	flag.BoolVar(&checkIntegrity, "check-integrity", false, "Verify file integrity after download")
	flag.StringVar(&checkSha256, "checksum-sha256", "", "Expected SHA-256 hash")
	flag.StringVar(&checkMd5, "checksum-md5", "", "Expected MD5 hash")
	flag.StringVar(&checkSha1, "checksum-sha1", "", "Expected SHA-1 hash")
	flag.StringVar(&parameterizedURL, "parameterized-url", "", "URL pattern with {} placeholder")
	flag.IntVar(&parameterizedStart, "start", 1, "Parameterized URL start index")
	flag.IntVar(&parameterizedEnd, "end", 100, "Parameterized URL end index")
	flag.IntVar(&parameterizedStep, "step", 1, "Parameterized URL step")
	flag.BoolVar(&daemonMode, "daemon", false, "Run as background daemon")
	flag.StringVar(&pidFile, "pid-file", "/tmp/had.pid", "PID file for daemon mode")
	flag.StringVar(&sshUser, "ssh-user", "", "SSH username")
	flag.StringVar(&sshPass, "ssh-pass", "", "SSH password")
	flag.StringVar(&sshKeyFile, "ssh-key", "", "SSH private key file")
	flag.StringVar(&sfftpKeyPass, "ssh-key-pass", "", "SSH key passphrase")
	flag.StringVar(&metalinkFile, "metalink", "", "Metalink file/URL (RFC 5854)")
	flag.BoolVar(&rpcEnabled, "rpc", false, "Enable JSON-RPC server")
	flag.StringVar(&rpcAddr, "rpc-addr", "localhost:6800", "RPC listen address")
	flag.BoolVar(&webSocketRPC, "rpc-websocket", false, "WebSocket RPC (experimental)")
	flag.BoolVar(&installCert, "install-cert", false, "Install capture proxy CA certificate")
	flag.StringVar(&captureProxy, "capture-proxy", "", "MITM proxy port (e.g. :8085)")
	flag.StringVar(&captureTypes, "capture-types", "video,music", "Types to capture")
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
	flag.BoolVar(&autoMirror, "auto-mirror", false, "Auto-select fastest mirror via ping")

	flag.StringVar(&queueFile, "queue", "", "Queue file with URLs and priorities")
	flag.StringVar(&queuePriority, "priority", "0", "Download priority for this job (higher = first)")

	flag.StringVar(&hlsURL, "hls", "", "HLS/M3U8 stream URL to download")
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

	if len(args) == 1 && strings.HasSuffix(args[0], ".json") {
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
		if isFTPURL(u) {
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
			name, size, err := fetchFileInfo(u, client)
			if err != nil {
				logWarning("skip %s: %v", u, err)
				continue
			}
			gs.addFile(name, size)
			validURLs = append(validURLs, u)
			fmt.Printf("  %s•%s %s (%s)\n", colors["green"], colors["reset"],
				name, Size4Human(size))
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
			if isFTPURL(rawURL) {
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
	fmt.Println("  had -queue <file>     download from priority queue file")
	fmt.Println("  had <session.json>    resume interrupted download")
	fmt.Printf("\n%s%s%s\n", colors["cyan"], sep, colors["reset"])
	fmt.Println("\nOPTIONS:")
	flag.PrintDefaults()
}