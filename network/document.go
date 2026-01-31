package network

import (
	"context"
	"fmt"

	"github.com/AYColumbia/viberowser/css"
	"github.com/AYColumbia/viberowser/dom"
)

// DocumentLoader handles loading a complete document with all its resources.
type DocumentLoader struct {
	loader *Loader
}

// NewDocumentLoader creates a new document loader.
func NewDocumentLoader(loader *Loader) *DocumentLoader {
	return &DocumentLoader{
		loader: loader,
	}
}

// LoadedDocument represents a fully loaded document with its resources.
type LoadedDocument struct {
	Document    *dom.Document
	Stylesheets []*LoadedStylesheet
	Scripts     []*LoadedScript
	BaseURL     string
	Errors      []error
}

// LoadedStylesheet represents a loaded stylesheet.
type LoadedStylesheet struct {
	URL        string
	Content    string
	Stylesheet *css.Stylesheet
	Error      error
}

// LoadedScript represents a loaded script.
type LoadedScript struct {
	URL      string
	Content  string
	Async    bool
	Defer    bool
	Module   bool
	Inline   bool // true if this is an inline script (no src attribute)
	Position int  // position in document order (for ordered execution)
	Error    error
}

// LoadDocumentWithResources loads an HTML document and all its external resources.
func (dl *DocumentLoader) LoadDocumentWithResources(ctx context.Context, doc *dom.Document, baseURL string) *LoadedDocument {
	result := &LoadedDocument{
		Document:    doc,
		Stylesheets: make([]*LoadedStylesheet, 0),
		Scripts:     make([]*LoadedScript, 0),
		BaseURL:     baseURL,
		Errors:      make([]error, 0),
	}

	// Set base URL on loader
	dl.loader.SetBaseURL(baseURL)

	// Load stylesheets
	dl.loadStylesheets(ctx, doc, result)

	// Load scripts
	dl.loadScripts(ctx, doc, result)

	return result
}

// loadStylesheets finds and loads all external stylesheets.
func (dl *DocumentLoader) loadStylesheets(ctx context.Context, doc *dom.Document, result *LoadedDocument) {
	linkElements := doc.GetElementsByTagName("link")
	for i := 0; i < linkElements.Length(); i++ {
		el := linkElements.Item(i)
		if el == nil {
			continue
		}

		rel := el.GetAttribute("rel")
		if rel != "stylesheet" {
			continue
		}

		href := el.GetAttribute("href")
		if href == "" {
			continue
		}

		loaded := &LoadedStylesheet{URL: href}

		resource := dl.loader.LoadStylesheet(ctx, href)
		if !resource.IsSuccess() {
			loaded.Error = fmt.Errorf("failed to load stylesheet %s: %v", href, resource.Error)
			if resource.Error != nil {
				loaded.Error = resource.Error
			} else {
				loaded.Error = fmt.Errorf("HTTP %d", resource.StatusCode)
			}
			result.Errors = append(result.Errors, loaded.Error)
			result.Stylesheets = append(result.Stylesheets, loaded)
			continue
		}

		loaded.Content = string(resource.Content)

		// Parse the CSS
		parser := css.NewParser(loaded.Content)
		loaded.Stylesheet = parser.Parse()

		result.Stylesheets = append(result.Stylesheets, loaded)
	}
}

// loadScripts finds and loads all scripts (both external and inline) in document order.
func (dl *DocumentLoader) loadScripts(ctx context.Context, doc *dom.Document, result *LoadedDocument) {
	scriptElements := doc.GetElementsByTagName("script")
	for i := 0; i < scriptElements.Length(); i++ {
		el := scriptElements.Item(i)
		if el == nil {
			continue
		}

		// Check script type
		scriptType := el.GetAttribute("type")
		if scriptType != "" && scriptType != "text/javascript" && scriptType != "application/javascript" && scriptType != "module" {
			continue // Not JavaScript
		}

		src := el.GetAttribute("src")
		if src == "" {
			// Inline script - include in ordered list
			loaded := &LoadedScript{
				Content:  el.TextContent(),
				Inline:   true,
				Position: i,
				Async:    false,
				Defer:    false,
				Module:   scriptType == "module",
			}
			result.Scripts = append(result.Scripts, loaded)
			continue
		}

		// External script
		loaded := &LoadedScript{
			URL:      src,
			Async:    el.HasAttribute("async"),
			Defer:    el.HasAttribute("defer"),
			Module:   scriptType == "module",
			Inline:   false,
			Position: i,
		}

		resource := dl.loader.LoadScript(ctx, src)
		if !resource.IsSuccess() {
			loaded.Error = fmt.Errorf("failed to load script %s: %v", src, resource.Error)
			if resource.Error != nil {
				loaded.Error = resource.Error
			} else {
				loaded.Error = fmt.Errorf("HTTP %d", resource.StatusCode)
			}
			result.Errors = append(result.Errors, loaded.Error)
			result.Scripts = append(result.Scripts, loaded)
			continue
		}

		loaded.Content = string(resource.Content)
		result.Scripts = append(result.Scripts, loaded)
	}
}

// GetSuccessfulStylesheets returns only successfully loaded stylesheets.
func (ld *LoadedDocument) GetSuccessfulStylesheets() []*LoadedStylesheet {
	var result []*LoadedStylesheet
	for _, s := range ld.Stylesheets {
		if s.Error == nil {
			result = append(result, s)
		}
	}
	return result
}

// GetSuccessfulScripts returns only successfully loaded scripts.
func (ld *LoadedDocument) GetSuccessfulScripts() []*LoadedScript {
	var result []*LoadedScript
	for _, s := range ld.Scripts {
		if s.Error == nil {
			result = append(result, s)
		}
	}
	return result
}

// GetSyncScripts returns successfully loaded synchronous external scripts (not async/defer, not inline).
// Deprecated: Use GetOrderedSyncScripts for proper document-order execution.
func (ld *LoadedDocument) GetSyncScripts() []*LoadedScript {
	var result []*LoadedScript
	for _, s := range ld.Scripts {
		if s.Error == nil && !s.Async && !s.Defer && !s.Inline {
			result = append(result, s)
		}
	}
	return result
}

// GetOrderedSyncScripts returns all synchronous scripts (both inline and external)
// in document order for proper interleaved execution.
func (ld *LoadedDocument) GetOrderedSyncScripts() []*LoadedScript {
	var result []*LoadedScript
	for _, s := range ld.Scripts {
		if s.Error == nil && !s.Async && !s.Defer {
			result = append(result, s)
		}
	}
	return result
}

// GetDeferredScripts returns successfully loaded deferred scripts.
func (ld *LoadedDocument) GetDeferredScripts() []*LoadedScript {
	var result []*LoadedScript
	for _, s := range ld.Scripts {
		if s.Error == nil && s.Defer {
			result = append(result, s)
		}
	}
	return result
}

// GetAsyncScripts returns successfully loaded async scripts.
func (ld *LoadedDocument) GetAsyncScripts() []*LoadedScript {
	var result []*LoadedScript
	for _, s := range ld.Scripts {
		if s.Error == nil && s.Async {
			result = append(result, s)
		}
	}
	return result
}
