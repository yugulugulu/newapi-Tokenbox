package router

import (
	"embed"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/setting/system_setting"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newSEOTestEngine(t *testing.T, serverAddress string) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	original := system_setting.ServerAddress
	system_setting.ServerAddress = serverAddress
	t.Cleanup(func() { system_setting.ServerAddress = original })

	indexPage := []byte("<head>\n<!--seo-canonical-->\n" +
		`<meta property="og:image" content="__SITE_URL__/logo.png" />` + "\n</head>")
	engine := gin.New()
	SetWebRouter(engine, WebAssets{BuildFS: embed.FS{}, IndexPage: indexPage})
	return engine
}

func serveSEO(engine *gin.Engine, path string) *httptest.ResponseRecorder {
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, path, nil)
	request.Host = "fallback.example"
	engine.ServeHTTP(recorder, request)
	return recorder
}

func TestRobotsTxtPointsCrawlersAtSitemapAndHidesPrivatePaths(t *testing.T) {
	engine := newSEOTestEngine(t, "https://tokenbox.example/")

	recorder := serveSEO(engine, "/robots.txt")

	require.Equal(t, http.StatusOK, recorder.Code)
	body := recorder.Body.String()
	assert.Contains(t, body, "Sitemap: https://tokenbox.example/sitemap.xml\n")
	assert.Contains(t, body, "Disallow: /api/\n")
	assert.Contains(t, body, "Disallow: /v1/\n")
	assert.Contains(t, body, "User-agent: GPTBot\nAllow: /\n")
	assert.Contains(t, body, "User-agent: ClaudeBot\nAllow: /\n")
}

func TestSitemapListsHomepage(t *testing.T) {
	engine := newSEOTestEngine(t, "https://tokenbox.example")

	recorder := serveSEO(engine, "/sitemap.xml")

	require.Equal(t, http.StatusOK, recorder.Code)
	assert.Contains(t, recorder.Header().Get("Content-Type"), "application/xml")
	assert.Contains(t, recorder.Body.String(), "<loc>https://tokenbox.example/</loc>")
}

func TestHomepageGetsCanonicalAndSiteURL(t *testing.T) {
	engine := newSEOTestEngine(t, "https://tokenbox.example")

	home := serveSEO(engine, "/")
	signIn := serveSEO(engine, "/sign-in")

	require.Equal(t, http.StatusOK, home.Code)
	assert.Contains(t, home.Body.String(), `<link rel="canonical" href="https://tokenbox.example/" />`)
	assert.Contains(t, home.Body.String(), `content="https://tokenbox.example/logo.png"`)
	assert.NotContains(t, home.Body.String(), "__SITE_URL__")

	require.Equal(t, http.StatusOK, signIn.Code)
	assert.NotContains(t, signIn.Body.String(), `rel="canonical"`)
	assert.Contains(t, signIn.Body.String(), `content="https://tokenbox.example/logo.png"`)
}

func TestSiteURLFallsBackToRequestHost(t *testing.T) {
	engine := newSEOTestEngine(t, "")

	recorder := serveSEO(engine, "/robots.txt")

	assert.Contains(t, recorder.Body.String(), "Sitemap: http://fallback.example/sitemap.xml\n")
}
