package network

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/AYColumbia/viberowser/dom"
)

func TestDocumentLoaderStylesheets(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/style.css":
			w.Header().Set("Content-Type", "text/css")
			w.Write([]byte("body { color: red; }"))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	html := `<!DOCTYPE html>
<html>
<head>
	<link rel="stylesheet" href="/style.css">
</head>
<body></body>
</html>`

	doc, err := dom.ParseHTML(html)
	if err != nil {
		t.Fatalf("ParseHTML error: %v", err)
	}

	client, err := NewClient()
	if err != nil {
		t.Fatalf("NewClient error: %v", err)
	}

	loader := NewLoader(client)
	docLoader := NewDocumentLoader(loader)

	result := docLoader.LoadDocumentWithResources(context.Background(), doc, server.URL)

	if len(result.Stylesheets) != 1 {
		t.Fatalf("expected 1 stylesheet, got %d", len(result.Stylesheets))
	}

	ss := result.Stylesheets[0]
	if ss.Error != nil {
		t.Errorf("stylesheet error: %v", ss.Error)
	}

	if ss.Content != "body { color: red; }" {
		t.Errorf("stylesheet content = %q", ss.Content)
	}

	if ss.Stylesheet == nil {
		t.Error("stylesheet was not parsed")
	}
}

func TestDocumentLoaderScripts(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/app.js":
			w.Header().Set("Content-Type", "application/javascript")
			w.Write([]byte("console.log('hello');"))
		case "/async.js":
			w.Header().Set("Content-Type", "application/javascript")
			w.Write([]byte("// async script"))
		case "/defer.js":
			w.Header().Set("Content-Type", "application/javascript")
			w.Write([]byte("// deferred script"))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	html := `<!DOCTYPE html>
<html>
<head>
	<script src="/app.js"></script>
	<script src="/async.js" async></script>
	<script src="/defer.js" defer></script>
</head>
<body></body>
</html>`

	doc, err := dom.ParseHTML(html)
	if err != nil {
		t.Fatalf("ParseHTML error: %v", err)
	}

	client, err := NewClient()
	if err != nil {
		t.Fatalf("NewClient error: %v", err)
	}

	loader := NewLoader(client)
	docLoader := NewDocumentLoader(loader)

	result := docLoader.LoadDocumentWithResources(context.Background(), doc, server.URL)

	if len(result.Scripts) != 3 {
		t.Fatalf("expected 3 scripts, got %d", len(result.Scripts))
	}

	// Check sync script
	syncScripts := result.GetSyncScripts()
	if len(syncScripts) != 1 {
		t.Errorf("expected 1 sync script, got %d", len(syncScripts))
	}
	if syncScripts[0].Content != "console.log('hello');" {
		t.Errorf("sync script content = %q", syncScripts[0].Content)
	}

	// Check async script
	asyncScripts := result.GetAsyncScripts()
	if len(asyncScripts) != 1 {
		t.Errorf("expected 1 async script, got %d", len(asyncScripts))
	}
	if !asyncScripts[0].Async {
		t.Error("expected async script to have Async=true")
	}

	// Check deferred script
	deferredScripts := result.GetDeferredScripts()
	if len(deferredScripts) != 1 {
		t.Errorf("expected 1 deferred script, got %d", len(deferredScripts))
	}
	if !deferredScripts[0].Defer {
		t.Error("expected deferred script to have Defer=true")
	}
}

func TestDocumentLoaderInlineScriptsIncluded(t *testing.T) {
	html := `<!DOCTYPE html>
<html>
<head>
	<script>console.log('inline');</script>
</head>
<body></body>
</html>`

	doc, err := dom.ParseHTML(html)
	if err != nil {
		t.Fatalf("ParseHTML error: %v", err)
	}

	client, err := NewClient()
	if err != nil {
		t.Fatalf("NewClient error: %v", err)
	}

	loader := NewLoader(client)
	docLoader := NewDocumentLoader(loader)

	result := docLoader.LoadDocumentWithResources(context.Background(), doc, "http://example.com")

	// Inline scripts are now included in the Scripts list for ordered execution
	if len(result.Scripts) != 1 {
		t.Errorf("expected 1 script (inline should be included), got %d", len(result.Scripts))
	}
	if len(result.Scripts) > 0 && !result.Scripts[0].Inline {
		t.Errorf("expected script to be marked as inline")
	}
	if len(result.Scripts) > 0 && result.Scripts[0].Content != "console.log('inline');" {
		t.Errorf("expected script content to match, got: %s", result.Scripts[0].Content)
	}
}

func TestDocumentLoaderFailedResources(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	}))
	defer server.Close()

	html := `<!DOCTYPE html>
<html>
<head>
	<link rel="stylesheet" href="/missing.css">
	<script src="/missing.js"></script>
</head>
<body></body>
</html>`

	doc, err := dom.ParseHTML(html)
	if err != nil {
		t.Fatalf("ParseHTML error: %v", err)
	}

	client, err := NewClient()
	if err != nil {
		t.Fatalf("NewClient error: %v", err)
	}

	loader := NewLoader(client)
	docLoader := NewDocumentLoader(loader)

	result := docLoader.LoadDocumentWithResources(context.Background(), doc, server.URL)

	if len(result.Errors) != 2 {
		t.Errorf("expected 2 errors, got %d", len(result.Errors))
	}

	if len(result.GetSuccessfulStylesheets()) != 0 {
		t.Error("expected 0 successful stylesheets")
	}

	if len(result.GetSuccessfulScripts()) != 0 {
		t.Error("expected 0 successful scripts")
	}
}

func TestDocumentLoaderDataURLStylesheet(t *testing.T) {
	html := `<!DOCTYPE html>
<html>
<head>
	<link rel="stylesheet" href="data:text/css,body{margin:0}">
</head>
<body></body>
</html>`

	doc, err := dom.ParseHTML(html)
	if err != nil {
		t.Fatalf("ParseHTML error: %v", err)
	}

	client, err := NewClient()
	if err != nil {
		t.Fatalf("NewClient error: %v", err)
	}

	loader := NewLoader(client)
	docLoader := NewDocumentLoader(loader)

	result := docLoader.LoadDocumentWithResources(context.Background(), doc, "http://example.com")

	if len(result.Stylesheets) != 1 {
		t.Fatalf("expected 1 stylesheet, got %d", len(result.Stylesheets))
	}

	ss := result.Stylesheets[0]
	if ss.Error != nil {
		t.Errorf("stylesheet error: %v", ss.Error)
	}

	if ss.Content != "body{margin:0}" {
		t.Errorf("stylesheet content = %q", ss.Content)
	}
}

func TestDocumentLoaderRelativeURLs(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/css/style.css":
			w.Header().Set("Content-Type", "text/css")
			w.Write([]byte("body {}"))
		case "/js/app.js":
			w.Header().Set("Content-Type", "application/javascript")
			w.Write([]byte("// app"))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	html := `<!DOCTYPE html>
<html>
<head>
	<link rel="stylesheet" href="css/style.css">
	<script src="js/app.js"></script>
</head>
<body></body>
</html>`

	doc, err := dom.ParseHTML(html)
	if err != nil {
		t.Fatalf("ParseHTML error: %v", err)
	}

	client, err := NewClient()
	if err != nil {
		t.Fatalf("NewClient error: %v", err)
	}

	loader := NewLoader(client)
	docLoader := NewDocumentLoader(loader)

	result := docLoader.LoadDocumentWithResources(context.Background(), doc, server.URL+"/page.html")

	successfulCSS := result.GetSuccessfulStylesheets()
	if len(successfulCSS) != 1 {
		t.Errorf("expected 1 successful stylesheet, got %d", len(successfulCSS))
	}

	successfulJS := result.GetSuccessfulScripts()
	if len(successfulJS) != 1 {
		t.Errorf("expected 1 successful script, got %d", len(successfulJS))
	}
}

func TestDocumentLoaderNonStylesheetLinks(t *testing.T) {
	html := `<!DOCTYPE html>
<html>
<head>
	<link rel="icon" href="/favicon.ico">
	<link rel="canonical" href="http://example.com/">
</head>
<body></body>
</html>`

	doc, err := dom.ParseHTML(html)
	if err != nil {
		t.Fatalf("ParseHTML error: %v", err)
	}

	client, err := NewClient()
	if err != nil {
		t.Fatalf("NewClient error: %v", err)
	}

	loader := NewLoader(client)
	docLoader := NewDocumentLoader(loader)

	result := docLoader.LoadDocumentWithResources(context.Background(), doc, "http://example.com")

	if len(result.Stylesheets) != 0 {
		t.Errorf("expected 0 stylesheets (non-stylesheet links ignored), got %d", len(result.Stylesheets))
	}
}

func TestDocumentLoaderNonJavaScriptScripts(t *testing.T) {
	html := `<!DOCTYPE html>
<html>
<head>
	<script type="text/template" src="/template.tpl"></script>
	<script type="application/json" src="/data.json"></script>
</head>
<body></body>
</html>`

	doc, err := dom.ParseHTML(html)
	if err != nil {
		t.Fatalf("ParseHTML error: %v", err)
	}

	client, err := NewClient()
	if err != nil {
		t.Fatalf("NewClient error: %v", err)
	}

	loader := NewLoader(client)
	docLoader := NewDocumentLoader(loader)

	result := docLoader.LoadDocumentWithResources(context.Background(), doc, "http://example.com")

	if len(result.Scripts) != 0 {
		t.Errorf("expected 0 scripts (non-JS scripts ignored), got %d", len(result.Scripts))
	}
}
