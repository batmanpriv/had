package lib

import (
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"mime"
	"net/http"
	neturl "net/url"
	"os"
	"os/signal"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
	"golang.org/x/sync/errgroup"
	"golang.org/x/sync/semaphore"
	"golang.org/x/time/rate"
)

type CrawlMode int

const (
	ModeFullSite CrawlMode = iota
	ModeSinglePage
)

const (
	maxPathLen     = 240
	maxSegLen      = 120
	defaultTimeout = 30 * time.Second
	stateVersion   = 2
)

var (
	assetExtensions = map[string]bool{
		".css": true, ".js": true, ".mjs": true, ".map": true,
		".json": true, ".wasm": true, ".webmanifest": true,
		".png": true, ".jpg": true, ".jpeg": true, ".gif": true,
		".webp": true, ".avif": true, ".svg": true, ".ico": true,
		".woff": true, ".woff2": true, ".ttf": true, ".eot": true,
		".mp4": true, ".webm": true, ".mp3": true, ".pdf": true,
		".ts": true, ".tsx": true, ".jsx": true, ".scss": true, ".less": true,
		".xml": true, ".txt": true, ".csv": true,
	}

	skipSchemes = map[string]bool{
		"mailto": true, "tel": true, "sms": true, "javascript": true,
		"data": true, "geo": true, "blob": true, "about": true,
	}

	cssURLRegex    = regexp.MustCompile(`url\(\s*(?:['"]?)([^'"()\s]+)(?:['"]?)\s*\)`)
	cssImportRegex = regexp.MustCompile(`@import\s+(?:url\(\s*['"]?|['"])([^'"\);]+)`)
	jsAssetRegex   = regexp.MustCompile(`['"` + "`" + `](/[^'"` + "`" + `?#\s]+\.(?:png|jpg|jpeg|gif|svg|webp|avif|ico|css|js|mjs|map|woff2?|ttf|eot|json|wasm|webmanifest))(?:\?[^'"` + "`" + `\s]*)?['"` + "`" + `]`)
	hashRouteRegex = regexp.MustCompile(`^(https?://[^#]+)#[!/]?/?`)
	metaRefreshRx  = regexp.MustCompile(`(?i)content=["']?\d+;\s*url=([^"'\s>]+)`)
	srcsetRx       = regexp.MustCompile(`([^\s,]+)(\s+[\d.]+[wx])?`)
)

type Config struct {
	TargetURL        string
	OutputDir        string
	Mode             CrawlMode
	MaxPages         int
	Concurrency      int
	DownloadExternal bool
	ExternalDomains  []string
	Cookies          map[string]string
	Headers          map[string]string
	UserAgent        string
	Timeout          time.Duration
	Retries          int
	MinifyOutput     bool
	Resume           bool
	RateLimit        float64
	MaxAssetSize     int64
	CrawlIframes     bool
	CrawlHashRoutes  bool
	StrictMIME       bool
	FollowMetaRefresh bool
}

type Dependency struct {
	URL       string
	LocalPath string
	IsPage    bool
}

type CrawlState struct {
	Version          int       `json:"version"`
	StartURL         string    `json:"start_url"`
	OutputDir        string    `json:"output_dir"`
	VisitedPages     []string  `json:"visited_pages"`
	DownloadedAssets []string  `json:"downloaded_assets"`
	LastUpdate       time.Time `json:"last_update"`
	PagesCount       int       `json:"pages_count"`
	AssetsCount      int       `json:"assets_count"`
	TotalBytes       int64     `json:"total_bytes"`
}

type domainLimiter struct {
	mu       sync.Mutex
	limiters map[string]*rate.Limiter
	defRate  rate.Limit
	burst    int
}

func newDomainLimiter(defaultRate float64, burst int) *domainLimiter {
	return &domainLimiter{
		limiters: make(map[string]*rate.Limiter),
		defRate:  rate.Limit(defaultRate),
		burst:    burst,
	}
}

func (dl *domainLimiter) Wait(ctx context.Context, domain string) error {
	dl.mu.Lock()
	l, ok := dl.limiters[domain]
	if !ok {
		l = rate.NewLimiter(dl.defRate, dl.burst)
		dl.limiters[domain] = l
	}
	dl.mu.Unlock()
	return l.Wait(ctx)
}

type stats struct {
	pages  atomic.Int64
	assets atomic.Int64
	bytes  atomic.Int64
	errors atomic.Int64
}

func (s *stats) print() {
	fmt.Printf("\r  pages=%-6d assets=%-6d size=%-8s errors=%d",
		s.pages.Load(),
		s.assets.Load(),
		humanBytes(s.bytes.Load()),
		s.errors.Load(),
	)
}

func humanBytes(b int64) string {
	switch {
	case b >= 1<<30:
		return fmt.Sprintf("%.2fGB", float64(b)/(1<<30))
	case b >= 1<<20:
		return fmt.Sprintf("%.2fMB", float64(b)/(1<<20))
	case b >= 1<<10:
		return fmt.Sprintf("%.1fKB", float64(b)/(1<<10))
	default:
		return fmt.Sprintf("%dB", b)
	}
}

type fetchResult struct {
	body        []byte
	contentType string
	finalURL    string
}

type Crawler struct {
	cfg          *Config
	client       *http.Client
	baseURL      *neturl.URL
	outputRoot   string
	visitedPages sync.Map
	queuedAssets sync.Map
	pageSem      *semaphore.Weighted
	assetSem     *semaphore.Weighted
	st           stats
	eg           *errgroup.Group
	ctx          context.Context
	cancel       context.CancelFunc
	depsCh       chan *Dependency
	pageQueue    chan string
	limiter      *domainLimiter
	stateFile    string
	wg           sync.WaitGroup
	closeOnce    sync.Once
}

func NewCrawler(cfg *Config) (*Crawler, error) {
	parsedURL, err := neturl.Parse(cfg.TargetURL)
	if err != nil {
		return nil, fmt.Errorf("invalid target URL: %w", err)
	}
	if parsedURL.Scheme == "" {
		parsedURL.Scheme = "https"
	}

	transport := &http.Transport{
		MaxIdleConns:          200,
		MaxIdleConnsPerHost:   cfg.Concurrency * 2,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ResponseHeaderTimeout: cfg.Timeout,
		DisableCompression:    true,
		ForceAttemptHTTP2:     true,
	}

	client := &http.Client{
		Transport: transport,
		Timeout:   cfg.Timeout,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 15 {
				return fmt.Errorf("too many redirects")
			}
			return nil
		},
	}

	outputRoot := cfg.OutputDir
	if outputRoot == "" {
		outputRoot = sanitizeFilename(parsedURL.Hostname())
	}
	if err := os.MkdirAll(outputRoot, 0o755); err != nil {
		return nil, fmt.Errorf("failed to create output directory: %w", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	eg, ctx := errgroup.WithContext(ctx)

	pageConcurrency := cfg.Concurrency
	if cfg.Mode == ModeSinglePage {
		pageConcurrency = 1
	}

	pageQueueSize := cfg.MaxPages * 2
	if pageQueueSize < 200 {
		pageQueueSize = 200
	}

	stateFile := filepath.Join(outputRoot, ".crawl_state.json")

	c := &Crawler{
		cfg:        cfg,
		client:     client,
		baseURL:    parsedURL,
		outputRoot: outputRoot,
		pageSem:    semaphore.NewWeighted(int64(pageConcurrency)),
		assetSem:   semaphore.NewWeighted(int64(cfg.Concurrency * 4)),
		eg:         eg,
		ctx:        ctx,
		cancel:     cancel,
		depsCh:     make(chan *Dependency, 5000),
		pageQueue:  make(chan string, pageQueueSize),
		limiter:    newDomainLimiter(cfg.RateLimit, cfg.Concurrency),
		stateFile:  stateFile,
	}

	if cfg.Resume {
		c.loadState()
	}

	return c, nil
}

func (c *Crawler) loadState() {
	data, err := os.ReadFile(c.stateFile)
	if err != nil {
		return
	}
	var state CrawlState
	if err := json.Unmarshal(data, &state); err != nil || state.Version != stateVersion {
		fmt.Println("[*] State incompatible, starting fresh")
		return
	}
	for _, u := range state.VisitedPages {
		c.visitedPages.Store(u, true)
	}
	for _, u := range state.DownloadedAssets {
		c.queuedAssets.Store(u, true)
	}
	c.st.pages.Store(int64(state.PagesCount))
	c.st.assets.Store(int64(state.AssetsCount))
	c.st.bytes.Store(state.TotalBytes)
	fmt.Printf("[✓] Resumed: %d pages, %d assets\n", state.PagesCount, state.AssetsCount)
}

func (c *Crawler) saveState() {
	if !c.cfg.Resume {
		return
	}
	var vp, da []string
	c.visitedPages.Range(func(k, _ interface{}) bool { vp = append(vp, k.(string)); return true })
	c.queuedAssets.Range(func(k, _ interface{}) bool { da = append(da, k.(string)); return true })

	state := CrawlState{
		Version:          stateVersion,
		StartURL:         c.cfg.TargetURL,
		OutputDir:        c.outputRoot,
		VisitedPages:     vp,
		DownloadedAssets: da,
		LastUpdate:       time.Now(),
		PagesCount:       int(c.st.pages.Load()),
		AssetsCount:      int(c.st.assets.Load()),
		TotalBytes:       c.st.bytes.Load(),
	}
	data, _ := json.MarshalIndent(state, "", "  ")
	os.WriteFile(c.stateFile, data, 0o644)
}

func (c *Crawler) Run() error {
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		fmt.Println("\n[!] Interrupted, saving state...")
		c.cancel()
		c.saveState()
	}()

	ticker := time.NewTicker(500 * time.Millisecond)
	go func() {
		for range ticker.C {
			c.st.print()
		}
	}()

	c.wg.Add(1)
	go func() {
		defer c.wg.Done()
		c.runAssetWorkers()
	}()

	startTime := time.Now()
	var runErr error

	if c.cfg.Mode == ModeSinglePage {
		fmt.Printf("[*] Single-page: %s\n", c.cfg.TargetURL)
		runErr = c.crawlSinglePage()
	} else {
		fmt.Printf("[*] Full-site: %s (max %d pages)\n", c.baseURL.String(), c.cfg.MaxPages)
		runErr = c.crawlFullSite()
	}

	c.closeOnce.Do(func() { close(c.depsCh) })
	c.wg.Wait()
	ticker.Stop()
	c.saveState()

	fmt.Printf("\n[✓] Done in %s — pages=%d assets=%d size=%s errors=%d\n",
		time.Since(startTime).Round(time.Millisecond),
		c.st.pages.Load(), c.st.assets.Load(),
		humanBytes(c.st.bytes.Load()), c.st.errors.Load(),
	)

	return runErr
}

func (c *Crawler) crawlSinglePage() error {
	url := c.cfg.TargetURL
	if c.cfg.CrawlHashRoutes {
		url = stripHash(url)
	}
	res, err := c.fetch(url)
	if err != nil {
		return fmt.Errorf("fetch main page: %w", err)
	}
	if res.finalURL != url {
		url = res.finalURL
	}

	doc, deps, err := c.processHTMLBytes(res.body, url)
	if err != nil {
		return err
	}
	if err := c.writeHTMLDoc(doc, url, res.body); err != nil {
		return err
	}
	c.st.pages.Add(1)
	c.enqueueDeps(deps)
	return nil
}

func (c *Crawler) crawlFullSite() error {
	startURL := c.cfg.TargetURL
	if c.cfg.CrawlHashRoutes {
		startURL = stripHash(startURL)
	}
	if _, loaded := c.visitedPages.LoadOrStore(startURL, true); !loaded {
		c.pageQueue <- startURL
	}

	var pwg sync.WaitGroup
	for i := 0; i < c.cfg.Concurrency; i++ {
		pwg.Add(1)
		go func() {
			defer pwg.Done()
			c.pageWorker()
		}()
	}
	pwg.Wait()
	return nil
}

func (c *Crawler) pageWorker() {
	for {
		select {
		case <-c.ctx.Done():
			return
		case url, ok := <-c.pageQueue:
			if !ok {
				return
			}
			if c.st.pages.Load() >= int64(c.cfg.MaxPages) {
				return
			}
			c.crawlPage(url)
		}
	}
}

func (c *Crawler) crawlPage(pageURL string) {
	if err := c.pageSem.Acquire(c.ctx, 1); err != nil {
		return
	}
	defer c.pageSem.Release(1)

	if err := c.limiter.Wait(c.ctx, c.baseURL.Host); err != nil {
		return
	}

	res, err := c.fetch(pageURL)
	if err != nil {
		c.st.errors.Add(1)
		return
	}

	finalURL := res.finalURL
	if finalURL == "" {
		finalURL = pageURL
	}

	if !isHTMLContent(res.contentType) {
		c.enqueueAsset(&Dependency{URL: pageURL, LocalPath: c.urlToLocalPath(mustParseURL(pageURL), false)})
		return
	}

	doc, deps, err := c.processHTMLBytes(res.body, finalURL)
	if err != nil {
		c.st.errors.Add(1)
		return
	}

	if err := c.writeHTMLDoc(doc, finalURL, res.body); err != nil {
		c.st.errors.Add(1)
		return
	}

	n := c.st.pages.Add(1)
	fmt.Printf("\n  [%d] %s", n, finalURL)

	c.enqueueDeps(deps)

	if c.cfg.Mode == ModeFullSite {
		c.enqueuePageLinks(doc, finalURL)
	}
}

func (c *Crawler) processHTMLBytes(body []byte, pageURL string) (*html.Node, []*Dependency, error) {
	body = stripBOM(body)

	doc, err := html.Parse(bytes.NewReader(body))
	if err != nil {
		return nil, nil, fmt.Errorf("parse HTML: %w", err)
	}

	deps := c.extractDeps(doc, pageURL)
	return doc, deps, nil
}

func (c *Crawler) writeHTMLDoc(doc *html.Node, pageURL string, rawBytes []byte) error {
	parsed, err := neturl.Parse(pageURL)
	if err != nil {
		return err
	}
	localPath := c.urlToLocalPath(parsed, true)

	if err := os.MkdirAll(filepath.Dir(localPath), 0o755); err != nil {
		return err
	}

	c.rewriteNode(doc, pageURL, localPath)

	var buf bytes.Buffer
	if err := html.Render(&buf, doc); err != nil {
		return err
	}
	out := buf.Bytes()

	if c.cfg.MinifyOutput {
		out = minifyHTML(out)
	}

	return os.WriteFile(localPath, out, 0o644)
}

func (c *Crawler) extractDeps(n *html.Node, baseURL string) []*Dependency {
	var deps []*Dependency
	var walk func(*html.Node)
	walk = func(node *html.Node) {
		if node.Type == html.ElementNode {
			deps = append(deps, c.depsFromElement(node, baseURL)...)
		}
		for ch := node.FirstChild; ch != nil; ch = ch.NextSibling {
			walk(ch)
		}
	}
	walk(n)
	return deps
}

func (c *Crawler) depsFromElement(node *html.Node, baseURL string) []*Dependency {
	var deps []*Dependency

	addURL := func(rawURL string, isPage bool) {
		if d := c.makeDep(rawURL, baseURL, isPage); d != nil {
			deps = append(deps, d)
		}
	}

	switch node.DataAtom {
	case atom.Script:
		src := attrVal(node, "src")
		if src != "" {
			addURL(src, false)
		} else if node.FirstChild != nil && node.FirstChild.Type == html.TextNode {
			for _, u := range extractURLsFromJS(node.FirstChild.Data, baseURL) {
				addURL(u, false)
			}
		}

	case atom.Link:
		href := attrVal(node, "href")
		rel := strings.ToLower(attrVal(node, "rel"))
		if href != "" && isResourceRel(rel) {
			addURL(href, false)
		}
		if rel == "prerender" || rel == "prefetch" {
			addURL(href, true)
		}

	case atom.Img:
		if src := attrVal(node, "src"); src != "" {
			addURL(src, false)
		}
		for _, u := range parseSrcset(attrVal(node, "srcset")) {
			addURL(u, false)
		}
		if di := attrVal(node, "data-src"); di != "" {
			addURL(di, false)
		}

	case atom.Source:
		if src := attrVal(node, "src"); src != "" {
			addURL(src, false)
		}
		for _, u := range parseSrcset(attrVal(node, "srcset")) {
			addURL(u, false)
		}

	case atom.Video, atom.Audio:
		if src := attrVal(node, "src"); src != "" {
			addURL(src, false)
		}
		if poster := attrVal(node, "poster"); poster != "" {
			addURL(poster, false)
		}

	case atom.Iframe:
		if src := attrVal(node, "src"); src != "" && c.cfg.CrawlIframes {
			addURL(src, true)
		}

	case atom.A:
		if href := attrVal(node, "href"); href != "" {
			if c.cfg.Mode == ModeFullSite {
				addURL(href, true)
			}
		}

	case atom.Meta:
		if c.cfg.FollowMetaRefresh {
			if strings.EqualFold(attrVal(node, "http-equiv"), "refresh") {
				if m := metaRefreshRx.FindStringSubmatch(attrVal(node, "content")); len(m) > 1 {
					addURL(m[1], true)
				}
			}
		}
		if prop := attrVal(node, "property"); strings.HasPrefix(prop, "og:image") {
			if content := attrVal(node, "content"); content != "" {
				addURL(content, false)
			}
		}

	case atom.Style:
		if node.FirstChild != nil && node.FirstChild.Type == html.TextNode {
			for _, u := range extractURLsFromCSS(node.FirstChild.Data, baseURL) {
				addURL(u, false)
			}
		}
	}

	for _, attr := range node.Attr {
		if attr.Key == "style" {
			for _, u := range extractURLsFromCSS(attr.Val, baseURL) {
				deps = append(deps, c.makeDep(u, baseURL, false))
			}
		}
		if strings.HasPrefix(attr.Key, "data-") && !strings.Contains(attr.Val, " ") {
			if looksLikeURL(attr.Val) {
				if d := c.makeDep(attr.Val, baseURL, false); d != nil {
					deps = append(deps, d)
				}
			}
		}
	}

	return deps
}

func (c *Crawler) makeDep(rawURL, baseURL string, isPage bool) *Dependency {
	if rawURL == "" {
		return nil
	}
	rawURL = strings.TrimSpace(rawURL)
	if strings.HasPrefix(rawURL, "#") || strings.HasPrefix(rawURL, "data:") {
		return nil
	}

	parsed, err := neturl.Parse(rawURL)
	if err != nil {
		return nil
	}
	if parsed.Scheme != "" && skipSchemes[parsed.Scheme] {
		return nil
	}

	abs := c.resolveURL(rawURL, baseURL)
	if abs == "" {
		return nil
	}

	absP, err := neturl.Parse(abs)
	if err != nil {
		return nil
	}

	isExternal := !c.isSameDomain(absP)
	if isExternal {
		if !c.cfg.DownloadExternal {
			return nil
		}
		if len(c.cfg.ExternalDomains) > 0 {
			allowed := false
			for _, d := range c.cfg.ExternalDomains {
				if strings.HasSuffix(absP.Hostname(), d) {
					allowed = true
					break
				}
			}
			if !allowed {
				return nil
			}
		}
	}

	if !isPage {
		ext := strings.ToLower(filepath.Ext(absP.Path))
		if ext != "" && !assetExtensions[ext] {
			return nil
		}
	}

	localPath := c.urlToLocalPath(absP, isPage)
	return &Dependency{URL: abs, LocalPath: localPath, IsPage: isPage}
}

func (c *Crawler) enqueueDeps(deps []*Dependency) {
	for _, d := range deps {
		if d == nil {
			continue
		}
		if d.IsPage {
			if c.cfg.CrawlHashRoutes {
				d.URL = stripHash(d.URL)
			}
			if _, loaded := c.visitedPages.LoadOrStore(d.URL, true); !loaded {
				if c.st.pages.Load() < int64(c.cfg.MaxPages) {
					select {
					case c.pageQueue <- d.URL:
					case <-c.ctx.Done():
						return
					default:
					}
				}
			}
		} else {
			c.enqueueAsset(d)
		}
	}
}

func (c *Crawler) enqueueAsset(d *Dependency) {
	if _, loaded := c.queuedAssets.LoadOrStore(d.URL, true); loaded {
		return
	}
	select {
	case c.depsCh <- d:
	case <-c.ctx.Done():
	}
}

func (c *Crawler) enqueuePageLinks(doc *html.Node, baseURL string) {
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode && n.DataAtom == atom.A {
			if href := attrVal(n, "href"); href != "" {
				abs := c.resolveURL(href, baseURL)
				if abs != "" && c.isSameDomainOrSub(abs) {
					if c.cfg.CrawlHashRoutes {
						abs = stripHash(abs)
					}
					if _, loaded := c.visitedPages.LoadOrStore(abs, true); !loaded {
						if c.st.pages.Load() < int64(c.cfg.MaxPages) {
							select {
							case c.pageQueue <- abs:
							case <-c.ctx.Done():
								return
							default:
							}
						}
					}
				}
			}
		}
		for ch := n.FirstChild; ch != nil; ch = ch.NextSibling {
			walk(ch)
		}
	}
	walk(doc)
}

func (c *Crawler) runAssetWorkers() {
	var inner sync.WaitGroup
	for dep := range c.depsCh {
		d := dep
		if err := c.assetSem.Acquire(c.ctx, 1); err != nil {
			return
		}
		inner.Add(1)
		go func() {
			defer c.assetSem.Release(1)
			defer inner.Done()
			c.downloadAsset(d)
		}()
	}
	inner.Wait()
}

func (c *Crawler) downloadAsset(d *Dependency) {
	if _, err := os.Stat(d.LocalPath); err == nil {
		return
	}

	if err := c.limiter.Wait(c.ctx, mustParseURL(d.URL).Host); err != nil {
		return
	}

	res, err := c.fetch(d.URL)
	if err != nil {
		c.st.errors.Add(1)
		return
	}

	if err := os.MkdirAll(filepath.Dir(d.LocalPath), 0o755); err != nil {
		c.st.errors.Add(1)
		return
	}

	body := res.body
	ext := strings.ToLower(filepath.Ext(d.LocalPath))
	switch ext {
	case ".css":
		body = c.rewriteCSS(body, d.URL, filepath.Dir(d.LocalPath))
		extraDeps := extractURLsFromCSS(string(res.body), d.URL)
		for _, u := range extraDeps {
			if dep := c.makeDep(u, d.URL, false); dep != nil {
				c.enqueueAsset(dep)
			}
		}
	case ".js", ".mjs":
		body = c.rewriteJS(body, d.URL, filepath.Dir(d.LocalPath))
	}

	if err := atomicWrite(d.LocalPath, body); err != nil {
		c.st.errors.Add(1)
		return
	}

	c.st.assets.Add(1)
	c.st.bytes.Add(int64(len(body)))
}

func atomicWrite(path string, data []byte) error {
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func (c *Crawler) fetch(rawURL string) (*fetchResult, error) {
	var lastErr error
	retries := c.cfg.Retries
	if retries <= 0 {
		retries = 3
	}

	for attempt := 0; attempt <= retries; attempt++ {
		if attempt > 0 {
			select {
			case <-c.ctx.Done():
				return nil, c.ctx.Err()
			case <-time.After(time.Duration(attempt*attempt) * time.Second):
			}
		}

		result, err := c.doFetch(rawURL)
		if err == nil {
			return result, nil
		}

		lastErr = err
		if isNonRetryable(err) {
			break
		}
	}

	return nil, fmt.Errorf("fetch %s after %d tries: %w", rawURL, retries, lastErr)
}

func (c *Crawler) doFetch(rawURL string) (*fetchResult, error) {
	req, err := http.NewRequestWithContext(c.ctx, "GET", rawURL, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("User-Agent", c.cfg.UserAgent)
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,*/*;q=0.8")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")
	req.Header.Set("Accept-Encoding", "gzip, deflate, br")
	req.Header.Set("Connection", "keep-alive")
	req.Header.Set("Cache-Control", "no-cache")

	for k, v := range c.cfg.Headers {
		req.Header.Set(k, v)
	}
	for name, value := range c.cfg.Cookies {
		req.AddCookie(&http.Cookie{Name: name, Value: value})
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 500 {
		return nil, fmt.Errorf("server error %d", resp.StatusCode)
	}
	if resp.StatusCode == 429 {
		return nil, fmt.Errorf("rate limited")
	}
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("client error %d (non-retryable)", resp.StatusCode)
	}
	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("unexpected redirect %d", resp.StatusCode)
	}

	if c.cfg.MaxAssetSize > 0 && resp.ContentLength > c.cfg.MaxAssetSize {
		return nil, fmt.Errorf("too large: %d bytes", resp.ContentLength)
	}

	var reader io.Reader = resp.Body
	switch resp.Header.Get("Content-Encoding") {
	case "gzip":
		gr, err := gzip.NewReader(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("gzip reader: %w", err)
		}
		defer gr.Close()
		reader = gr
	case "br":
		reader = resp.Body
	}

	limited := io.Reader(reader)
	if c.cfg.MaxAssetSize > 0 {
		limited = io.LimitReader(reader, c.cfg.MaxAssetSize+1)
	}

	body, err := io.ReadAll(limited)
	if err != nil {
		return nil, fmt.Errorf("read body: %w", err)
	}

	if c.cfg.MaxAssetSize > 0 && int64(len(body)) > c.cfg.MaxAssetSize {
		return nil, fmt.Errorf("body exceeded max size")
	}

	ct := resp.Header.Get("Content-Type")
	finalURL := resp.Request.URL.String()

	c.st.bytes.Add(int64(len(body)))
	return &fetchResult{body: body, contentType: ct, finalURL: finalURL}, nil
}

func (c *Crawler) rewriteNode(n *html.Node, pageURL, pagePath string) {
	if n.Type == html.ElementNode {
		switch n.DataAtom {
		case atom.Base:
			if n.Parent != nil {
				n.Parent.RemoveChild(n)
				return
			}

		case atom.A:
			setAttr(n, "href", func(v string) string {
				return c.rewriteURLForPage(v, pageURL, pagePath, true)
			})

		case atom.Img:
			setAttr(n, "src", func(v string) string {
				return c.rewriteURLForPage(v, pageURL, pagePath, false)
			})
			setAttr(n, "data-src", func(v string) string {
				return c.rewriteURLForPage(v, pageURL, pagePath, false)
			})
			transformSrcset(n, func(u string) string {
				return c.rewriteURLForPage(u, pageURL, pagePath, false)
			})

		case atom.Script:
			setAttr(n, "src", func(v string) string {
				return c.rewriteURLForPage(v, pageURL, pagePath, false)
			})
			if n.FirstChild != nil && n.FirstChild.Type == html.TextNode {
				n.FirstChild.Data = string(c.rewriteJS([]byte(n.FirstChild.Data), pageURL, filepath.Dir(pagePath)))
			}

		case atom.Link:
			setAttr(n, "href", func(v string) string {
				rel := strings.ToLower(attrVal(n, "rel"))
				if isResourceRel(rel) {
					return c.rewriteURLForPage(v, pageURL, pagePath, false)
				}
				return v
			})

		case atom.Video, atom.Audio:
			setAttr(n, "src", func(v string) string {
				return c.rewriteURLForPage(v, pageURL, pagePath, false)
			})
			setAttr(n, "poster", func(v string) string {
				return c.rewriteURLForPage(v, pageURL, pagePath, false)
			})

		case atom.Source:
			setAttr(n, "src", func(v string) string {
				return c.rewriteURLForPage(v, pageURL, pagePath, false)
			})
			transformSrcset(n, func(u string) string {
				return c.rewriteURLForPage(u, pageURL, pagePath, false)
			})

		case atom.Iframe:
			if c.cfg.CrawlIframes {
				setAttr(n, "src", func(v string) string {
					return c.rewriteURLForPage(v, pageURL, pagePath, true)
				})
			}

		case atom.Style:
			if n.FirstChild != nil && n.FirstChild.Type == html.TextNode {
				n.FirstChild.Data = string(c.rewriteCSS([]byte(n.FirstChild.Data), pageURL, filepath.Dir(pagePath)))
			}

		case atom.Meta:
			name := strings.ToLower(attrVal(n, "name"))
			prop := attrVal(n, "property")
			if strings.HasPrefix(prop, "og:image") || name == "twitter:image" {
				setAttr(n, "content", func(v string) string {
					return c.rewriteURLForPage(v, pageURL, pagePath, false)
				})
			}
		}

		for i, attr := range n.Attr {
			if attr.Key == "style" {
				n.Attr[i].Val = string(c.rewriteCSS([]byte(attr.Val), pageURL, filepath.Dir(pagePath)))
			}
		}
	}

	for ch := n.FirstChild; ch != nil; ch = ch.NextSibling {
		c.rewriteNode(ch, pageURL, pagePath)
	}
}

func (c *Crawler) rewriteURLForPage(rawURL, pageURL, pagePath string, isPage bool) string {
	if rawURL == "" || strings.HasPrefix(rawURL, "#") ||
		strings.HasPrefix(rawURL, "javascript:") || strings.HasPrefix(rawURL, "data:") {
		return rawURL
	}

	parsed, err := neturl.Parse(rawURL)
	if err != nil {
		return rawURL
	}
	if parsed.Scheme != "" && skipSchemes[parsed.Scheme] {
		return rawURL
	}

	abs := c.resolveURL(rawURL, pageURL)
	if abs == "" {
		return rawURL
	}

	absP, err := neturl.Parse(abs)
	if err != nil {
		return rawURL
	}

	isExternal := !c.isSameDomain(absP)
	if isPage && isExternal {
		return rawURL
	}
	if !isPage && isExternal && !c.cfg.DownloadExternal {
		return rawURL
	}

	localPath := c.urlToLocalPath(absP, isPage)
	rel, err := filepath.Rel(filepath.Dir(pagePath), localPath)
	if err != nil {
		return rawURL
	}
	rel = filepath.ToSlash(rel)

	if absP.Fragment != "" {
		rel += "#" + absP.Fragment
	}

	return rel
}

func (c *Crawler) rewriteCSS(data []byte, baseURL, baseDir string) []byte {
	s := string(data)

	s = cssURLRegex.ReplaceAllStringFunc(s, func(match string) string {
		parts := cssURLRegex.FindStringSubmatch(match)
		if len(parts) < 2 {
			return match
		}
		u := strings.Trim(strings.TrimSpace(parts[1]), "'\"")
		if strings.HasPrefix(u, "data:") {
			return match
		}
		abs := c.resolveURL(u, baseURL)
		if abs == "" {
			return match
		}
		p, err := neturl.Parse(abs)
		if err != nil {
			return match
		}
		if !c.isSameDomain(p) && !c.cfg.DownloadExternal {
			return match
		}
		lp := c.urlToLocalPath(p, false)
		rel, err := filepath.Rel(baseDir, lp)
		if err != nil {
			return match
		}
		c.enqueueAsset(&Dependency{URL: abs, LocalPath: lp})
		return "url('" + filepath.ToSlash(rel) + "')"
	})

	s = cssImportRegex.ReplaceAllStringFunc(s, func(match string) string {
		parts := cssImportRegex.FindStringSubmatch(match)
		if len(parts) < 2 {
			return match
		}
		u := strings.Trim(strings.TrimSpace(parts[1]), "'\"")
		abs := c.resolveURL(u, baseURL)
		if abs == "" {
			return match
		}
		p, err := neturl.Parse(abs)
		if err != nil {
			return match
		}
		lp := c.urlToLocalPath(p, false)
		rel, err := filepath.Rel(baseDir, lp)
		if err != nil {
			return match
		}
		c.enqueueAsset(&Dependency{URL: abs, LocalPath: lp})
		return "@import '" + filepath.ToSlash(rel) + "'"
	})

	return []byte(s)
}

func (c *Crawler) rewriteJS(data []byte, baseURL, baseDir string) []byte {
	s := string(data)
	s = jsAssetRegex.ReplaceAllStringFunc(s, func(match string) string {
		parts := jsAssetRegex.FindStringSubmatch(match)
		if len(parts) < 2 {
			return match
		}
		u := parts[1]
		abs := c.resolveURL(u, baseURL)
		if abs == "" {
			return match
		}
		p, err := neturl.Parse(abs)
		if err != nil {
			return match
		}
		lp := c.urlToLocalPath(p, false)
		rel, err := filepath.Rel(baseDir, lp)
		if err != nil {
			return match
		}
		c.enqueueAsset(&Dependency{URL: abs, LocalPath: lp})
		q := string(match[0])
		return q + filepath.ToSlash(rel) + q
	})
	return []byte(s)
}

func (c *Crawler) resolveURL(rawURL, baseURL string) string {
	rawURL = strings.TrimSpace(rawURL)
	if strings.HasPrefix(rawURL, "//") {
		rawURL = c.baseURL.Scheme + ":" + rawURL
	}
	base, err := neturl.Parse(baseURL)
	if err != nil {
		return ""
	}
	ref, err := neturl.Parse(rawURL)
	if err != nil {
		return ""
	}
	resolved := base.ResolveReference(ref)
	resolved.Fragment = ""
	return resolved.String()
}

func (c *Crawler) isSameDomain(u *neturl.URL) bool {
	return u.Hostname() == c.baseURL.Hostname()
}

func (c *Crawler) isSameDomainOrSub(rawURL string) bool {
	p, err := neturl.Parse(rawURL)
	if err != nil {
		return false
	}
	h := p.Hostname()
	base := c.baseURL.Hostname()
	return h == base || strings.HasSuffix(h, "."+base)
}

func (c *Crawler) urlToLocalPath(u *neturl.URL, isPage bool) string {
	rel := strings.TrimPrefix(u.Path, "/")

	if isPage {
		if rel == "" {
			rel = "index.html"
		} else if strings.HasSuffix(rel, "/") {
			rel += "index.html"
		} else if filepath.Ext(rel) == "" {
			rel += "/index.html"
		}
	} else {
		if rel == "" {
			rel = "_root"
		}
	}

	if u.RawQuery != "" {
		h := sha256.Sum256([]byte(u.RawQuery))
		hs := hex.EncodeToString(h[:])[:10]
		ext := filepath.Ext(rel)
		stem := strings.TrimSuffix(rel, ext)
		rel = stem + "-" + hs + ext
	}

	parts := strings.Split(rel, "/")
	for i, p := range parts {
		p = sanitizeSegment(p)
		if len(p) > maxSegLen {
			p = shortenSegment(p, maxSegLen)
		}
		parts[i] = p
	}

	localPath := filepath.Join(c.outputRoot, filepath.Join(parts...))
	if len(localPath) > maxPathLen {
		h := sha256.Sum256([]byte(u.String()))
		hs := hex.EncodeToString(h[:])[:16]
		ext := filepath.Ext(localPath)
		dir := filepath.Dir(localPath)
		base := filepath.Base(localPath)
		stem := strings.TrimSuffix(base, ext)
		maxStem := 50
		if len(stem) < maxStem {
			maxStem = len(stem)
		}
		localPath = filepath.Join(dir, stem[:maxStem]+"-"+hs+ext)
	}

	return localPath
}

func extractURLsFromCSS(css, baseURL string) []string {
	seen := map[string]bool{}
	var out []string
	for _, m := range cssURLRegex.FindAllStringSubmatch(css, -1) {
		u := strings.Trim(strings.TrimSpace(m[1]), "'\"")
		if !seen[u] && !strings.HasPrefix(u, "data:") {
			seen[u] = true
			out = append(out, u)
		}
	}
	for _, m := range cssImportRegex.FindAllStringSubmatch(css, -1) {
		u := strings.Trim(strings.TrimSpace(m[1]), "'\"")
		if !seen[u] {
			seen[u] = true
			out = append(out, u)
		}
	}
	return out
}

func extractURLsFromJS(js, baseURL string) []string {
	seen := map[string]bool{}
	var out []string
	for _, m := range jsAssetRegex.FindAllStringSubmatch(js, -1) {
		u := m[1]
		if !seen[u] {
			seen[u] = true
			out = append(out, u)
		}
	}
	return out
}

func parseSrcset(srcset string) []string {
	if srcset == "" {
		return nil
	}
	var urls []string
	for _, m := range srcsetRx.FindAllStringSubmatch(srcset, -1) {
		u := strings.TrimSpace(m[1])
		if u != "" && !strings.HasSuffix(u, "x") && !strings.HasSuffix(u, "w") {
			urls = append(urls, u)
		}
	}
	return urls
}

func isResourceRel(rel string) bool {
	switch rel {
	case "stylesheet", "icon", "shortcut icon", "apple-touch-icon",
		"preload", "modulepreload", "manifest", "mask-icon":
		return true
	}
	return false
}

func isHTMLContent(ct string) bool {
	mt, _, _ := mime.ParseMediaType(ct)
	return mt == "text/html" || mt == "application/xhtml+xml" || ct == ""
}

func isNonRetryable(err error) bool {
	s := err.Error()
	return strings.Contains(s, "non-retryable") || strings.Contains(s, "too many redirects")
}

func looksLikeURL(s string) bool {
	return strings.HasPrefix(s, "/") || strings.HasPrefix(s, "http://") || strings.HasPrefix(s, "https://")
}

func stripBOM(b []byte) []byte {
	if len(b) >= 3 && b[0] == 0xEF && b[1] == 0xBB && b[2] == 0xBF {
		return b[3:]
	}
	return b
}

func stripHash(rawURL string) string {
	if m := hashRouteRegex.FindStringSubmatch(rawURL); len(m) > 1 {
		return m[1]
	}
	return rawURL
}

func mustParseURL(rawURL string) *neturl.URL {
	u, err := neturl.Parse(rawURL)
	if err != nil {
		return &neturl.URL{}
	}
	return u
}

func attrVal(n *html.Node, key string) string {
	for _, a := range n.Attr {
		if a.Key == key {
			return a.Val
		}
	}
	return ""
}

func setAttr(n *html.Node, key string, fn func(string) string) {
	for i, a := range n.Attr {
		if a.Key == key {
			n.Attr[i].Val = fn(a.Val)
			return
		}
	}
}

func transformSrcset(n *html.Node, fn func(string) string) {
	for i, a := range n.Attr {
		if a.Key == "srcset" {
			parts := srcsetRx.FindAllStringSubmatch(a.Val, -1)
			var out []string
			for _, p := range parts {
				newURL := fn(p[1])
				out = append(out, newURL+p[2])
			}
			n.Attr[i].Val = strings.Join(out, ", ")
			return
		}
	}
}

func sanitizeSegment(s string) string {
	s = strings.TrimSpace(s)
	s = strings.Trim(s, ". ")
	r := strings.NewReplacer(
		"<", "_", ">", "_", ":", "_", `"`, "_", "|", "_",
		"?", "_", "*", "_", `\`, "_", "/", "_",
	)
	s = r.Replace(s)
	if s == "" || s == "." || s == ".." {
		return "_"
	}
	return s
}

func shortenSegment(s string, limit int) string {
	if len(s) <= limit {
		return s
	}
	ext := filepath.Ext(s)
	stem := strings.TrimSuffix(s, ext)
	h := sha256.Sum256([]byte(s))
	hs := hex.EncodeToString(h[:])[:8]
	keep := limit - len(ext) - 9
	if keep < 8 {
		keep = 8
	}
	if keep > len(stem) {
		keep = len(stem)
	}
	return stem[:keep] + "-" + hs + ext
}

func sanitizeFilename(name string) string {
	return strings.NewReplacer(".", "_", ":", "_", "/", "_").Replace(name)
}

func minifyHTML(b []byte) []byte {
	s := string(b)
	s = regexp.MustCompile(`>\s+<`).ReplaceAllString(s, "><")
	s = regexp.MustCompile(`[ \t]{2,}`).ReplaceAllString(s, " ")
	s = regexp.MustCompile(`\n{2,}`).ReplaceAllString(s, "\n")
	return []byte(s)
}

func RunWebDownloader() {
	var (
		targetURL       string
		outputDir       string
		mode            string
		maxPages        int
		concurrency     int
		downloadExt     bool
		externalDomStr  string
		cookieStr       string
		headerStr       string
		userAgent       string
		timeoutSec      int
		retries         int
		minify          bool
		resume          bool
		rateLimit       float64
		maxAssetSizeMB  int
		crawlIframes    bool
		crawlHashRoutes bool
		strictMIME      bool
		followMeta      bool
	)

	fs := flag.NewFlagSet("web", flag.ExitOnError)
	fs.StringVar(&targetURL, "url", "", "Target URL to backup")
	fs.StringVar(&outputDir, "output", "", "Output directory (default: domain name)")
	fs.StringVar(&mode, "mode", "single", "Crawl mode: 'single' or 'full'")
	fs.IntVar(&maxPages, "max-pages", 100, "Max pages for full-site mode")
	fs.IntVar(&concurrency, "concurrency", 5, "Concurrent workers")
	fs.BoolVar(&downloadExt, "download-external", false, "Download external assets")
	fs.StringVar(&externalDomStr, "external-domains", "", "Comma-separated allowed external domains")
	fs.StringVar(&cookieStr, "cookies", "", "Cookies: name1=value1; name2=value2")
	fs.StringVar(&headerStr, "headers", "", "Extra headers: Key1:Value1,Key2:Value2")
	fs.StringVar(&userAgent, "user-agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0 Safari/537.36", "User-Agent string")
	fs.IntVar(&timeoutSec, "timeout", 30, "Request timeout (seconds)")
	fs.IntVar(&retries, "retries", 3, "Retries on failure")
	fs.BoolVar(&minify, "minify", false, "Minify HTML output")
	fs.BoolVar(&resume, "resume", false, "Resume interrupted crawl")
	fs.Float64Var(&rateLimit, "rate-limit", 10, "Requests/second per domain")
	fs.IntVar(&maxAssetSizeMB, "max-asset-size", 100, "Max asset size in MB (0 = unlimited)")
	fs.BoolVar(&crawlIframes, "crawl-iframes", true, "Download iframe content")
	fs.BoolVar(&crawlHashRoutes, "crawl-hash-routes", true, "Handle hash-based SPA routing")
	fs.BoolVar(&strictMIME, "strict-mime", false, "Only download assets matching expected MIME type")
	fs.BoolVar(&followMeta, "follow-meta-refresh", true, "Follow meta-refresh redirects")
	fs.Parse(os.Args[1:])

	if targetURL == "" {
		fmt.Fprintln(os.Stderr, "Error: --url is required")
		fs.Usage()
		os.Exit(1)
	}

	var crawlMode CrawlMode
	switch strings.ToLower(mode) {
	case "single":
		crawlMode = ModeSinglePage
	case "full":
		crawlMode = ModeFullSite
	default:
		fmt.Fprintf(os.Stderr, "Invalid mode: %s\n", mode)
		os.Exit(1)
	}

	var externalDomains []string
	for _, d := range strings.Split(externalDomStr, ",") {
		if t := strings.TrimSpace(d); t != "" {
			externalDomains = append(externalDomains, t)
		}
	}

	cookies := make(map[string]string)
	for _, part := range strings.Split(cookieStr, ";") {
		kv := strings.SplitN(strings.TrimSpace(part), "=", 2)
		if len(kv) == 2 {
			cookies[strings.TrimSpace(kv[0])] = strings.TrimSpace(kv[1])
		}
	}

	headers := make(map[string]string)
	for _, part := range strings.Split(headerStr, ",") {
		kv := strings.SplitN(strings.TrimSpace(part), ":", 2)
		if len(kv) == 2 {
			headers[strings.TrimSpace(kv[0])] = strings.TrimSpace(kv[1])
		}
	}

	cfg := &Config{
		TargetURL:         targetURL,
		OutputDir:         outputDir,
		Mode:              crawlMode,
		MaxPages:          maxPages,
		Concurrency:       concurrency,
		DownloadExternal:  downloadExt,
		ExternalDomains:   externalDomains,
		Cookies:           cookies,
		Headers:           headers,
		UserAgent:         userAgent,
		Timeout:           time.Duration(timeoutSec) * time.Second,
		Retries:           retries,
		MinifyOutput:      minify,
		Resume:            resume,
		RateLimit:         rateLimit,
		MaxAssetSize:      int64(maxAssetSizeMB) * 1024 * 1024,
		CrawlIframes:      crawlIframes,
		CrawlHashRoutes:   crawlHashRoutes,
		StrictMIME:        strictMIME,
		FollowMetaRefresh: followMeta,
	}

	crawler, err := NewCrawler(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	if err := crawler.Run(); err != nil && err != context.Canceled {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}