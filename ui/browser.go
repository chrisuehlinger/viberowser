// Package ui provides the browser user interface using Fyne.
package ui

import (
	"context"
	"fmt"
	"image"
	"net/url"
	"strings"
	"sync"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"github.com/chrisuehlinger/viberowser/css"
	"github.com/chrisuehlinger/viberowser/dom"
	vibelayout "github.com/chrisuehlinger/viberowser/layout"
	"github.com/chrisuehlinger/viberowser/network"
	"github.com/chrisuehlinger/viberowser/render"
)

// BrowserUI represents the main browser UI.
type BrowserUI struct {
	app    fyne.App
	window fyne.Window

	// Tab management
	tabs       []*BrowserTab
	activeTab  int
	tabBar     *container.AppTabs
	contentBox *fyne.Container

	// Navigation controls
	backBtn    *widget.Button
	forwardBtn *widget.Button
	refreshBtn *widget.Button
	stopBtn    *widget.Button
	urlEntry   *widget.Entry
	goBtn      *widget.Button

	// Network
	httpClient *network.Client
	loader     *network.Loader

	mu sync.Mutex
}

// BrowserTab represents a single browser tab.
type BrowserTab struct {
	URL     string
	Title   string
	Loading bool

	// Navigation history
	history      []string
	historyIndex int

	// Rendered content
	document    *dom.Document
	layoutRoot  *vibelayout.LayoutBox
	canvas      *render.Canvas
	canvasImage *canvas.Image

	// Content container
	content *fyne.Container
	scroll  *container.Scroll

	// Loading cancellation
	cancelFunc context.CancelFunc
}

// NewBrowserUI creates a new browser UI instance.
func NewBrowserUI() *BrowserUI {
	a := app.New()
	w := a.NewWindow("Viberowser")
	w.Resize(fyne.NewSize(1280, 800))

	client, err := network.NewClient()
	if err != nil {
		panic(fmt.Sprintf("failed to create HTTP client: %v", err))
	}

	b := &BrowserUI{
		app:        a,
		window:     w,
		tabs:       make([]*BrowserTab, 0),
		activeTab:  -1,
		httpClient: client,
	}

	b.loader = network.NewLoader(b.httpClient)

	b.setupUI()
	b.setupKeyboardShortcuts()

	return b
}

// setupUI creates the browser UI components.
func (b *BrowserUI) setupUI() {
	// Navigation buttons
	b.backBtn = widget.NewButtonWithIcon("", theme.NavigateBackIcon(), b.goBack)
	b.forwardBtn = widget.NewButtonWithIcon("", theme.NavigateNextIcon(), b.goForward)
	b.refreshBtn = widget.NewButtonWithIcon("", theme.ViewRefreshIcon(), b.refresh)
	b.stopBtn = widget.NewButtonWithIcon("", theme.CancelIcon(), b.stop)

	// URL entry
	b.urlEntry = widget.NewEntry()
	b.urlEntry.SetPlaceHolder("Enter URL...")
	b.urlEntry.OnSubmitted = func(url string) {
		b.navigate(url)
	}

	b.goBtn = widget.NewButton("Go", func() {
		b.navigate(b.urlEntry.Text)
	})

	// New tab button
	newTabBtn := widget.NewButtonWithIcon("", theme.ContentAddIcon(), b.newTab)

	// Navigation bar - URL entry fills available space
	navBar := container.NewBorder(nil, nil,
		container.NewHBox(b.backBtn, b.forwardBtn, b.refreshBtn, b.stopBtn),
		container.NewHBox(b.goBtn, newTabBtn),
		b.urlEntry,
	)

	// Tab bar
	b.tabBar = container.NewAppTabs()
	b.tabBar.SetTabLocation(container.TabLocationTop)
	b.tabBar.OnSelected = func(tab *container.TabItem) {
		b.onTabSelected(tab)
	}

	// Content area
	b.contentBox = container.NewStack()

	// Main layout
	mainContent := container.NewBorder(
		container.NewVBox(navBar, b.tabBar),
		nil, nil, nil,
		b.contentBox,
	)

	b.window.SetContent(mainContent)

	// Create initial tab
	b.newTab()
}

// setupKeyboardShortcuts sets up keyboard shortcuts.
func (b *BrowserUI) setupKeyboardShortcuts() {
	// Ctrl+T: New tab
	b.window.Canvas().AddShortcut(&desktop.CustomShortcut{
		KeyName:  fyne.KeyT,
		Modifier: fyne.KeyModifierControl,
	}, func(_ fyne.Shortcut) {
		b.newTab()
	})

	// Ctrl+W: Close tab
	b.window.Canvas().AddShortcut(&desktop.CustomShortcut{
		KeyName:  fyne.KeyW,
		Modifier: fyne.KeyModifierControl,
	}, func(_ fyne.Shortcut) {
		b.closeActiveTab()
	})

	// Ctrl+L: Focus URL bar
	b.window.Canvas().AddShortcut(&desktop.CustomShortcut{
		KeyName:  fyne.KeyL,
		Modifier: fyne.KeyModifierControl,
	}, func(_ fyne.Shortcut) {
		b.focusURLBar()
	})

	// Ctrl+R: Refresh
	b.window.Canvas().AddShortcut(&desktop.CustomShortcut{
		KeyName:  fyne.KeyR,
		Modifier: fyne.KeyModifierControl,
	}, func(_ fyne.Shortcut) {
		b.refresh()
	})

	// Alt+Left: Back
	b.window.Canvas().AddShortcut(&desktop.CustomShortcut{
		KeyName:  fyne.KeyLeft,
		Modifier: fyne.KeyModifierAlt,
	}, func(_ fyne.Shortcut) {
		b.goBack()
	})

	// Alt+Right: Forward
	b.window.Canvas().AddShortcut(&desktop.CustomShortcut{
		KeyName:  fyne.KeyRight,
		Modifier: fyne.KeyModifierAlt,
	}, func(_ fyne.Shortcut) {
		b.goForward()
	})
}

// newTab creates a new browser tab.
func (b *BrowserUI) newTab() {
	b.mu.Lock()
	defer b.mu.Unlock()

	tab := &BrowserTab{
		URL:          "",
		Title:        "New Tab",
		Loading:      false,
		history:      make([]string, 0),
		historyIndex: -1,
	}

	// Create content container with placeholder
	placeholder := widget.NewLabel("Enter a URL to browse")
	placeholder.Alignment = fyne.TextAlignCenter
	tab.content = container.NewStack(container.NewCenter(placeholder))
	tab.scroll = container.NewScroll(tab.content)

	b.tabs = append(b.tabs, tab)
	b.activeTab = len(b.tabs) - 1

	// Add tab to tab bar
	tabItem := container.NewTabItem(tab.Title, tab.scroll)
	b.tabBar.Append(tabItem)
	b.tabBar.Select(tabItem)

	// Update content
	b.updateContent()
}

// closeActiveTab closes the currently active tab.
func (b *BrowserUI) closeActiveTab() {
	b.mu.Lock()
	defer b.mu.Unlock()

	if len(b.tabs) <= 1 {
		// Don't close the last tab, just clear it
		if len(b.tabs) == 1 {
			if b.tabs[0].cancelFunc != nil {
				b.tabs[0].cancelFunc()
			}
			b.tabs[0].URL = ""
			b.tabs[0].Title = "New Tab"
			b.tabs[0].history = make([]string, 0)
			b.tabs[0].historyIndex = -1
			b.urlEntry.SetText("")
			b.updateTabTitle(0)
		}
		return
	}

	if b.activeTab < 0 || b.activeTab >= len(b.tabs) {
		return
	}

	// Cancel any ongoing loading
	if b.tabs[b.activeTab].cancelFunc != nil {
		b.tabs[b.activeTab].cancelFunc()
	}

	// Remove from tab bar
	b.tabBar.Remove(b.tabBar.Items[b.activeTab])

	// Remove from tabs slice
	b.tabs = append(b.tabs[:b.activeTab], b.tabs[b.activeTab+1:]...)

	// Adjust active tab index
	if b.activeTab >= len(b.tabs) {
		b.activeTab = len(b.tabs) - 1
	}

	if b.activeTab >= 0 {
		b.tabBar.Select(b.tabBar.Items[b.activeTab])
	}

	b.updateContent()
}

// onTabSelected handles tab selection.
func (b *BrowserUI) onTabSelected(tabItem *container.TabItem) {
	b.mu.Lock()
	defer b.mu.Unlock()

	for i, item := range b.tabBar.Items {
		if item == tabItem {
			b.activeTab = i
			break
		}
	}

	b.updateContent()
}

// updateContent updates the displayed content for the active tab.
func (b *BrowserUI) updateContent() {
	if b.activeTab < 0 || b.activeTab >= len(b.tabs) {
		return
	}

	tab := b.tabs[b.activeTab]
	b.urlEntry.SetText(tab.URL)
	b.updateNavigationButtons()
}

// updateNavigationButtons updates the enabled state of nav buttons.
func (b *BrowserUI) updateNavigationButtons() {
	if b.activeTab < 0 || b.activeTab >= len(b.tabs) {
		b.backBtn.Disable()
		b.forwardBtn.Disable()
		return
	}

	tab := b.tabs[b.activeTab]

	if tab.historyIndex > 0 {
		b.backBtn.Enable()
	} else {
		b.backBtn.Disable()
	}

	if tab.historyIndex < len(tab.history)-1 {
		b.forwardBtn.Enable()
	} else {
		b.forwardBtn.Disable()
	}

	if tab.Loading {
		b.stopBtn.Enable()
		b.refreshBtn.Disable()
	} else {
		b.stopBtn.Disable()
		b.refreshBtn.Enable()
	}
}

// updateTabTitle updates the title of a tab.
func (b *BrowserUI) updateTabTitle(index int) {
	if index < 0 || index >= len(b.tabs) || index >= len(b.tabBar.Items) {
		return
	}

	tab := b.tabs[index]
	title := tab.Title
	if len(title) > 30 {
		title = title[:27] + "..."
	}
	b.tabBar.Items[index].Text = title
	b.tabBar.Refresh()
}

// navigate navigates to the given URL.
func (b *BrowserUI) navigate(urlStr string) {
	b.mu.Lock()
	if b.activeTab < 0 || b.activeTab >= len(b.tabs) {
		b.mu.Unlock()
		return
	}
	tab := b.tabs[b.activeTab]

	// Cancel any previous loading
	if tab.cancelFunc != nil {
		tab.cancelFunc()
	}
	b.mu.Unlock()

	// Normalize URL
	urlStr = normalizeURL(urlStr)

	// Add to history
	b.mu.Lock()
	// Truncate forward history
	if tab.historyIndex < len(tab.history)-1 {
		tab.history = tab.history[:tab.historyIndex+1]
	}
	tab.history = append(tab.history, urlStr)
	tab.historyIndex = len(tab.history) - 1
	tab.URL = urlStr
	tab.Loading = true
	b.updateNavigationButtons()
	b.urlEntry.SetText(urlStr)
	b.mu.Unlock()

	// Create cancellable context
	ctx, cancel := context.WithCancel(context.Background())
	b.mu.Lock()
	tab.cancelFunc = cancel
	b.mu.Unlock()

	// Load in background
	go b.loadPage(tab, urlStr, ctx)
}

// loadPage loads and renders a page.
func (b *BrowserUI) loadPage(tab *BrowserTab, urlStr string, ctx context.Context) {
	// Show loading indicator
	b.showLoading(tab)

	// Fetch the page
	resp := b.loader.LoadDocument(ctx, urlStr)
	if !resp.IsSuccess() {
		b.showError(tab, fmt.Sprintf("Failed to load page: %v", resp.Error))
		return
	}

	// Check for cancellation
	select {
	case <-ctx.Done():
		return
	default:
	}

	// Parse HTML
	doc, err := dom.ParseHTML(string(resp.Content))
	if err != nil {
		b.showError(tab, fmt.Sprintf("Failed to parse HTML: %v", err))
		return
	}

	tab.document = doc

	// Get page title
	title := doc.Title()
	if title != "" {
		b.mu.Lock()
		tab.Title = title
		b.updateTabTitle(b.activeTab)
		b.mu.Unlock()
	}

	// Load external stylesheets
	docLoader := network.NewDocumentLoader(b.loader)
	loadedDoc := docLoader.LoadDocumentWithResources(ctx, doc, urlStr)

	// Build style resolver with loaded stylesheets
	styleResolver := css.NewStyleResolver()

	// Add user agent stylesheet
	styleResolver.SetUserAgentStylesheet(css.GetUserAgentStylesheet())

	// Add external author stylesheets
	for _, stylesheet := range loadedDoc.GetSuccessfulStylesheets() {
		if stylesheet.Stylesheet != nil {
			styleResolver.AddAuthorStylesheet(stylesheet.Stylesheet)
		}
	}

	// Also process inline styles
	styleElements := doc.GetElementsByTagName("style")
	for i := 0; i < styleElements.Length(); i++ {
		styleEl := styleElements.Item(i)
		if styleEl != nil {
			cssContent := styleEl.TextContent()
			parser := css.NewParser(cssContent)
			stylesheet := parser.Parse()
			styleResolver.AddAuthorStylesheet(stylesheet)
		}
	}

	// Get the document element (html) or body
	rootElement := doc.DocumentElement()
	if rootElement == nil {
		b.showError(tab, "No document element found")
		return
	}

	// Check for cancellation
	select {
	case <-ctx.Done():
		return
	default:
	}

	// Build layout tree
	viewportWidth := 1200.0
	viewportHeight := 2000.0 // Allow for tall pages
	layoutCtx := vibelayout.NewLayoutContext(viewportWidth, viewportHeight)
	tab.layoutRoot = vibelayout.BuildLayoutTree(rootElement, styleResolver, layoutCtx)

	if tab.layoutRoot != nil {
		// Layout the tree
		tab.layoutRoot.Layout(layoutCtx)

		// Update element geometries for getBoundingClientRect and related APIs
		vibelayout.UpdateElementGeometries(tab.layoutRoot, nil, 0, 0)

		// Calculate content height
		contentHeight := tab.layoutRoot.Dimensions.MarginBox().Height
		if contentHeight < viewportHeight {
			contentHeight = viewportHeight
		}

		// Create canvas and paint
		tab.canvas = render.NewCanvas(int(viewportWidth), int(contentHeight))
		tab.canvas.Paint(tab.layoutRoot)

		// Convert to image and display
		img := tab.canvas.ToImage()
		b.displayImage(tab, img)
	}

	b.mu.Lock()
	tab.Loading = false
	b.updateNavigationButtons()
	b.mu.Unlock()
}

// showLoading displays a loading indicator in the tab.
func (b *BrowserUI) showLoading(tab *BrowserTab) {
	b.mu.Lock()
	defer b.mu.Unlock()

	loadingLabel := widget.NewLabel("Loading...")
	loadingLabel.Alignment = fyne.TextAlignCenter

	tab.content.Objects = []fyne.CanvasObject{container.NewCenter(loadingLabel)}
	tab.content.Refresh()
}

// displayImage displays the rendered image in the tab.
func (b *BrowserUI) displayImage(tab *BrowserTab, img *image.RGBA) {
	// Create a Fyne image from the rendered content
	fyneImg := canvas.NewImageFromImage(img)
	fyneImg.FillMode = canvas.ImageFillOriginal
	fyneImg.ScaleMode = canvas.ImageScalePixels

	b.mu.Lock()
	tab.canvasImage = fyneImg

	// Update the content container
	tab.content.Objects = []fyne.CanvasObject{fyneImg}
	tab.content.Refresh()
	tab.scroll.Refresh()
	b.mu.Unlock()
}

// showError displays an error message in the tab.
func (b *BrowserUI) showError(tab *BrowserTab, message string) {
	b.mu.Lock()
	defer b.mu.Unlock()

	tab.Loading = false
	b.updateNavigationButtons()

	errorLabel := widget.NewLabel(message)
	errorLabel.Wrapping = fyne.TextWrapWord
	errorLabel.Alignment = fyne.TextAlignCenter

	errorBox := container.NewVBox(
		widget.NewLabel("Error"),
		errorLabel,
	)

	tab.content.Objects = []fyne.CanvasObject{container.NewCenter(errorBox)}
	tab.content.Refresh()
}

// goBack navigates back in history.
func (b *BrowserUI) goBack() {
	b.mu.Lock()
	if b.activeTab < 0 || b.activeTab >= len(b.tabs) {
		b.mu.Unlock()
		return
	}
	tab := b.tabs[b.activeTab]

	if tab.historyIndex <= 0 {
		b.mu.Unlock()
		return
	}

	// Cancel any previous loading
	if tab.cancelFunc != nil {
		tab.cancelFunc()
	}

	tab.historyIndex--
	urlStr := tab.history[tab.historyIndex]
	tab.URL = urlStr
	tab.Loading = true
	b.updateNavigationButtons()
	b.urlEntry.SetText(urlStr)
	b.mu.Unlock()

	ctx, cancel := context.WithCancel(context.Background())
	b.mu.Lock()
	tab.cancelFunc = cancel
	b.mu.Unlock()

	go b.loadPage(tab, urlStr, ctx)
}

// goForward navigates forward in history.
func (b *BrowserUI) goForward() {
	b.mu.Lock()
	if b.activeTab < 0 || b.activeTab >= len(b.tabs) {
		b.mu.Unlock()
		return
	}
	tab := b.tabs[b.activeTab]

	if tab.historyIndex >= len(tab.history)-1 {
		b.mu.Unlock()
		return
	}

	// Cancel any previous loading
	if tab.cancelFunc != nil {
		tab.cancelFunc()
	}

	tab.historyIndex++
	urlStr := tab.history[tab.historyIndex]
	tab.URL = urlStr
	tab.Loading = true
	b.updateNavigationButtons()
	b.urlEntry.SetText(urlStr)
	b.mu.Unlock()

	ctx, cancel := context.WithCancel(context.Background())
	b.mu.Lock()
	tab.cancelFunc = cancel
	b.mu.Unlock()

	go b.loadPage(tab, urlStr, ctx)
}

// refresh reloads the current page.
func (b *BrowserUI) refresh() {
	b.mu.Lock()
	if b.activeTab < 0 || b.activeTab >= len(b.tabs) {
		b.mu.Unlock()
		return
	}
	tab := b.tabs[b.activeTab]

	if tab.URL == "" {
		b.mu.Unlock()
		return
	}

	// Cancel any previous loading
	if tab.cancelFunc != nil {
		tab.cancelFunc()
	}

	urlStr := tab.URL
	tab.Loading = true
	b.updateNavigationButtons()
	b.mu.Unlock()

	ctx, cancel := context.WithCancel(context.Background())
	b.mu.Lock()
	tab.cancelFunc = cancel
	b.mu.Unlock()

	go b.loadPage(tab, urlStr, ctx)
}

// stop stops loading the current page.
func (b *BrowserUI) stop() {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.activeTab < 0 || b.activeTab >= len(b.tabs) {
		return
	}
	tab := b.tabs[b.activeTab]

	if tab.cancelFunc != nil {
		tab.cancelFunc()
	}

	tab.Loading = false
	b.updateNavigationButtons()
}

// focusURLBar focuses the URL entry.
func (b *BrowserUI) focusURLBar() {
	b.window.Canvas().Focus(b.urlEntry)
	// Select all text by setting cursor position
	b.urlEntry.CursorColumn = len(b.urlEntry.Text)
}

// Run starts the browser UI.
func (b *BrowserUI) Run() {
	b.window.ShowAndRun()
}

// NavigateToURL navigates to the given URL. This is useful for setting an initial URL.
func (b *BrowserUI) NavigateToURL(urlStr string) {
	b.navigate(urlStr)
}

// normalizeURL normalizes a URL string.
func normalizeURL(urlStr string) string {
	urlStr = strings.TrimSpace(urlStr)

	// Add scheme if missing
	if !strings.Contains(urlStr, "://") {
		// Check if it looks like a URL
		if strings.Contains(urlStr, ".") || strings.HasPrefix(urlStr, "localhost") {
			urlStr = "https://" + urlStr
		} else {
			// Treat as search query (for now, just add https)
			urlStr = "https://" + urlStr
		}
	}

	// Parse and normalize
	u, err := url.Parse(urlStr)
	if err != nil {
		return urlStr
	}

	return u.String()
}
