// Package ui provides the browser user interface.
package ui

// Window represents the main browser window.
type Window struct {
	Width  int
	Height int
	Title  string
}

// Tab represents a browser tab.
type Tab struct {
	URL     string
	Title   string
	Loading bool
}

// Browser represents the browser application state.
type Browser struct {
	Window *Window
	Tabs   []*Tab
	Active int
}

// NewBrowser creates a new browser instance.
func NewBrowser(width, height int) *Browser {
	return &Browser{
		Window: &Window{
			Width:  width,
			Height: height,
			Title:  "Viberowser",
		},
		Tabs:   make([]*Tab, 0),
		Active: -1,
	}
}

// NewTab creates a new tab and makes it active.
func (b *Browser) NewTab(url string) *Tab {
	tab := &Tab{
		URL:     url,
		Title:   "New Tab",
		Loading: false,
	}
	b.Tabs = append(b.Tabs, tab)
	b.Active = len(b.Tabs) - 1
	return tab
}

// CloseTab closes the tab at the given index.
func (b *Browser) CloseTab(index int) {
	if index < 0 || index >= len(b.Tabs) {
		return
	}
	b.Tabs = append(b.Tabs[:index], b.Tabs[index+1:]...)
	if b.Active >= len(b.Tabs) {
		b.Active = len(b.Tabs) - 1
	}
}

// ActiveTab returns the currently active tab, or nil if none.
func (b *Browser) ActiveTab() *Tab {
	if b.Active < 0 || b.Active >= len(b.Tabs) {
		return nil
	}
	return b.Tabs[b.Active]
}
