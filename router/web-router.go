package router

import (
	"bytes"
	"embed"
	"html"
	"net/http"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/controller"
	"github.com/QuantumNous/new-api/middleware"
	"github.com/QuantumNous/new-api/setting/system_setting"
	"github.com/gin-contrib/gzip"
	"github.com/gin-contrib/static"
	"github.com/gin-gonic/gin"
)

// WebAssets holds the embedded dashboard frontend assets.
type WebAssets struct {
	BuildFS   embed.FS
	IndexPage []byte
}

func SetWebRouter(router *gin.Engine, assets WebAssets) {
	frontendFS := common.EmbedFolder(assets.BuildFS, "web/dist")

	router.Use(gzip.Gzip(gzip.DefaultCompression))
	router.Use(middleware.GlobalWebRateLimit())
	router.Use(middleware.Cache())
	router.Use(static.Serve("/", frontendFS))
	
	// Handle /docs routes - serve docs index.html for directory access
	router.GET("/doc", func(c *gin.Context) {
		c.Redirect(http.StatusMovedPermanently, "/docs/")
	})
	router.GET("/docs", func(c *gin.Context) {
		c.Redirect(http.StatusMovedPermanently, "/docs/")
	})
	router.GET("/docs/", func(c *gin.Context) {
		// Read docs/index.html from embedded FS
		data, err := assets.BuildFS.ReadFile("web/dist/docs/index.html")
		if err != nil {
			c.Status(http.StatusNotFound)
			return
		}
		c.Header("Content-Type", "text/html; charset=utf-8")
		c.Data(http.StatusOK, "text/html; charset=utf-8", data)
	})
	
	router.GET("/robots.txt", func(c *gin.Context) {
		c.Header("Cache-Control", "max-age=3600")
		c.String(http.StatusOK, buildRobotsTxt(siteBaseURL(c)))
	})
	router.GET("/sitemap.xml", func(c *gin.Context) {
		c.Header("Cache-Control", "max-age=3600")
		c.Data(http.StatusOK, "application/xml; charset=utf-8", []byte(
			`<?xml version="1.0" encoding="UTF-8"?>`+"\n"+
				`<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">`+"\n"+
				`  <url><loc>`+html.EscapeString(siteBaseURL(c))+`/</loc></url>`+"\n"+
				`</urlset>`+"\n"))
	})

	router.NoRoute(func(c *gin.Context) {
		c.Set(middleware.RouteTagKey, "web")
		if strings.HasPrefix(c.Request.RequestURI, "/v1") || strings.HasPrefix(c.Request.RequestURI, "/api") || strings.HasPrefix(c.Request.RequestURI, "/assets") {
			controller.RelayNotFound(c)
			return
		}
		baseURL := html.EscapeString(siteBaseURL(c))
		page := bytes.ReplaceAll(assets.IndexPage, []byte("__SITE_URL__"), []byte(baseURL))
		// Only the homepage declares a canonical URL; SPA-only paths must not claim to be "/".
		if c.Request.URL.Path == "/" {
			canonical := `<link rel="canonical" href="` + baseURL + `/" />` + "\n" +
				`    <meta property="og:url" content="` + baseURL + `/" />` + "\n"
			page = bytes.Replace(page, []byte("<!--seo-canonical-->\n"), []byte(canonical), 1)
		}
		c.Header("Cache-Control", "no-cache")
		c.Data(http.StatusOK, "text/html; charset=utf-8", page)
	})
}

// siteBaseURL returns the public site origin used in SEO output (canonical, robots, sitemap),
// preferring the admin-configured ServerAddress and falling back to the request host.
func siteBaseURL(c *gin.Context) string {
	address := strings.TrimRight(strings.TrimSpace(system_setting.ServerAddress), "/")
	if address != "" {
		return address
	}
	scheme := "http"
	if c.Request.TLS != nil || c.GetHeader("X-Forwarded-Proto") == "https" {
		scheme = "https"
	}
	return scheme + "://" + c.Request.Host
}

// buildRobotsTxt lists crawler rules: public pages are open, API/relay endpoints and
// account/console pages are excluded, and AI search crawlers are explicitly welcomed.
func buildRobotsTxt(baseURL string) string {
	privatePaths := []string{
		"/api/", "/v1/", "/v1beta/", "/mj/", "/suno/", "/pg/", "/kling/",
		"/sign-in", "/sign-up", "/register", "/forgot-password", "/reset", "/otp", "/oauth", "/user/",
		"/channels", "/chat", "/commissions", "/dashboard", "/keys", "/models", "/playground",
		"/profile", "/redemption-codes", "/subscriptions", "/system-info", "/system-settings",
		"/usage-logs", "/users", "/wallet", "/withdrawal", "/setup",
	}
	aiCrawlers := []string{"GPTBot", "OAI-SearchBot", "ChatGPT-User", "ClaudeBot", "Claude-SearchBot", "PerplexityBot", "Google-Extended"}

	var b strings.Builder
	b.WriteString("User-agent: *\nAllow: /\n")
	for _, path := range privatePaths {
		b.WriteString("Disallow: " + path + "\n")
	}
	for _, bot := range aiCrawlers {
		b.WriteString("\nUser-agent: " + bot + "\nAllow: /\n")
		for _, path := range privatePaths {
			b.WriteString("Disallow: " + path + "\n")
		}
	}
	b.WriteString("\nSitemap: " + baseURL + "/sitemap.xml\n")
	return b.String()
}
