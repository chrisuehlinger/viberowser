// Package js provides JavaScript execution capabilities for the browser.
// This file implements the Web Storage API (localStorage and sessionStorage).
package js

import (
	"net/url"
	"sort"
	"strings"
	"sync"

	"github.com/dop251/goja"
)

// StorageType represents whether storage is local or session-based.
type StorageType int

const (
	// LocalStorage persists across browser sessions (in-memory for this implementation).
	LocalStorage StorageType = iota
	// SessionStorage persists only for the tab's lifetime.
	SessionStorage
)

// StorageManager manages the Web Storage API (localStorage and sessionStorage).
// It implements per-origin storage isolation as required by the spec.
type StorageManager struct {
	runtime     *Runtime
	documentURL *url.URL
	eventBinder *EventBinder

	mu sync.RWMutex
}

// storageData holds the actual storage data.
// This is a package-level variable to allow localStorage to persist across
// page loads within the same browser session.
var (
	localStorageData   = make(map[string]map[string]string) // origin -> key-value pairs
	sessionStorageData = make(map[string]map[string]string) // origin -> key-value pairs
	storageMu          sync.RWMutex
)

// NewStorageManager creates a new storage manager.
func NewStorageManager(runtime *Runtime, documentURL *url.URL, eventBinder *EventBinder) *StorageManager {
	return &StorageManager{
		runtime:     runtime,
		documentURL: documentURL,
		eventBinder: eventBinder,
	}
}

// SetupStorage installs localStorage and sessionStorage on the window.
func (m *StorageManager) SetupStorage() {
	vm := m.runtime.VM()
	window := vm.Get("window")
	if window == nil || goja.IsUndefined(window) {
		return
	}
	windowObj := window.ToObject(vm)
	if windowObj == nil {
		return
	}

	// Create localStorage
	localStorage := m.createStorageObject(LocalStorage)
	windowObj.Set("localStorage", localStorage)
	vm.Set("localStorage", localStorage)

	// Create sessionStorage
	sessionStorage := m.createStorageObject(SessionStorage)
	windowObj.Set("sessionStorage", sessionStorage)
	vm.Set("sessionStorage", sessionStorage)

	// Set up Storage constructor on window
	m.setupStorageConstructor(windowObj)
}

// setupStorageConstructor creates the Storage constructor.
func (m *StorageManager) setupStorageConstructor(window *goja.Object) {
	vm := m.runtime.VM()

	// Storage should not be directly constructable
	storageConstructor := vm.ToValue(func(call goja.ConstructorCall) *goja.Object {
		panic(vm.NewTypeError("Illegal constructor"))
	})

	window.Set("Storage", storageConstructor)
	vm.Set("Storage", storageConstructor)
}

// createStorageObject creates a Storage object with the required interface.
func (m *StorageManager) createStorageObject(storageType StorageType) *goja.Object {
	vm := m.runtime.VM()
	storage := vm.NewObject()

	// Get origin for this storage
	origin := m.getOrigin()

	// length - returns the number of items in storage
	storage.DefineAccessorProperty("length", vm.ToValue(func(call goja.FunctionCall) goja.Value {
		return vm.ToValue(m.getLength(storageType, origin))
	}), nil, goja.FLAG_FALSE, goja.FLAG_TRUE)

	// key(index) - returns the key at the given index
	storage.Set("key", func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) < 1 {
			return goja.Null()
		}
		index := int(call.Arguments[0].ToInteger())
		key := m.key(storageType, origin, index)
		if key == "" {
			return goja.Null()
		}
		return vm.ToValue(key)
	})

	// getItem(key) - returns the value for the given key
	storage.Set("getItem", func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) < 1 {
			return goja.Null()
		}
		key := call.Arguments[0].String()
		value, ok := m.getItem(storageType, origin, key)
		if !ok {
			return goja.Null()
		}
		return vm.ToValue(value)
	})

	// setItem(key, value) - sets the value for the given key
	storage.Set("setItem", func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) < 2 {
			return goja.Undefined()
		}
		key := call.Arguments[0].String()
		value := call.Arguments[1].String()
		oldValue := m.setItem(storageType, origin, key, value)
		m.fireStorageEvent(key, oldValue, value, storageType, storage)
		return goja.Undefined()
	})

	// removeItem(key) - removes the item with the given key
	storage.Set("removeItem", func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) < 1 {
			return goja.Undefined()
		}
		key := call.Arguments[0].String()
		oldValue, existed := m.removeItem(storageType, origin, key)
		if existed {
			m.fireStorageEvent(key, oldValue, "", storageType, storage)
		}
		return goja.Undefined()
	})

	// clear() - removes all items from storage
	storage.Set("clear", func(call goja.FunctionCall) goja.Value {
		m.clear(storageType, origin)
		m.fireStorageEvent("", "", "", storageType, storage)
		return goja.Undefined()
	})

	return storage
}

// getOrigin returns the origin string for the current document.
func (m *StorageManager) getOrigin() string {
	if m.documentURL == nil {
		return "null" // opaque origin
	}

	// Origin is scheme://host:port
	port := m.documentURL.Port()
	if port == "" {
		// Use default ports
		switch m.documentURL.Scheme {
		case "http":
			port = "80"
		case "https":
			port = "443"
		}
	}

	return strings.ToLower(m.documentURL.Scheme) + "://" + strings.ToLower(m.documentURL.Hostname()) + ":" + port
}

// getStorageData returns the appropriate storage map for the given type.
func (m *StorageManager) getStorageData(storageType StorageType) map[string]map[string]string {
	if storageType == LocalStorage {
		return localStorageData
	}
	return sessionStorageData
}

// getLength returns the number of items in storage for the given origin.
func (m *StorageManager) getLength(storageType StorageType, origin string) int {
	storageMu.RLock()
	defer storageMu.RUnlock()

	data := m.getStorageData(storageType)
	if originData, ok := data[origin]; ok {
		return len(originData)
	}
	return 0
}

// key returns the key at the given index for the given origin.
// Keys are returned in an implementation-defined order (we use sorted order for consistency).
func (m *StorageManager) key(storageType StorageType, origin string, index int) string {
	storageMu.RLock()
	defer storageMu.RUnlock()

	data := m.getStorageData(storageType)
	originData, ok := data[origin]
	if !ok {
		return ""
	}

	// Get sorted keys for consistent ordering
	keys := make([]string, 0, len(originData))
	for k := range originData {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	if index < 0 || index >= len(keys) {
		return ""
	}
	return keys[index]
}

// getItem returns the value for the given key.
func (m *StorageManager) getItem(storageType StorageType, origin, key string) (string, bool) {
	storageMu.RLock()
	defer storageMu.RUnlock()

	data := m.getStorageData(storageType)
	originData, ok := data[origin]
	if !ok {
		return "", false
	}
	value, ok := originData[key]
	return value, ok
}

// setItem sets the value for the given key and returns the old value (if any).
func (m *StorageManager) setItem(storageType StorageType, origin, key, value string) string {
	storageMu.Lock()
	defer storageMu.Unlock()

	data := m.getStorageData(storageType)
	if _, ok := data[origin]; !ok {
		data[origin] = make(map[string]string)
	}

	oldValue := data[origin][key]
	data[origin][key] = value
	return oldValue
}

// removeItem removes the item with the given key and returns its old value.
func (m *StorageManager) removeItem(storageType StorageType, origin, key string) (string, bool) {
	storageMu.Lock()
	defer storageMu.Unlock()

	data := m.getStorageData(storageType)
	originData, ok := data[origin]
	if !ok {
		return "", false
	}

	oldValue, existed := originData[key]
	if existed {
		delete(originData, key)
	}
	return oldValue, existed
}

// clear removes all items from storage for the given origin.
func (m *StorageManager) clear(storageType StorageType, origin string) {
	storageMu.Lock()
	defer storageMu.Unlock()

	data := m.getStorageData(storageType)
	delete(data, origin)
}

// fireStorageEvent fires a storage event on other windows with the same origin.
// Per spec, the event should NOT fire on the window that made the change.
func (m *StorageManager) fireStorageEvent(key, oldValue, newValue string, storageType StorageType, storageArea *goja.Object) {
	// For now, we don't have multiple windows, so we skip firing events.
	// In a full implementation, this would iterate over all windows with the
	// same origin and dispatch the event to each one (except the current window).
	//
	// The implementation would:
	// 1. Get all windows with the same origin
	// 2. For each window (except current):
	//    - Create a StorageEvent with the appropriate properties
	//    - Dispatch it to that window

	// TODO: When multi-window support is added, implement cross-window event firing
	_ = key
	_ = oldValue
	_ = newValue
	_ = storageType
	_ = storageArea
}

// ClearAllStorage clears all localStorage and sessionStorage data.
// This is useful for testing.
func ClearAllStorage() {
	storageMu.Lock()
	defer storageMu.Unlock()

	localStorageData = make(map[string]map[string]string)
	sessionStorageData = make(map[string]map[string]string)
}
