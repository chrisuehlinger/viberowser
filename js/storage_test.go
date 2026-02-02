package js

import (
	"testing"

	"github.com/chrisuehlinger/viberowser/dom"
)

func TestStorageAPI(t *testing.T) {
	// Clear storage before each test
	ClearAllStorage()

	runtime := NewRuntime()
	executor := NewScriptExecutor(runtime)

	doc := dom.NewDocument()
	doc.SetURL("https://example.com/")
	executor.SetupDocument(doc)

	t.Run("localStorage exists on window", func(t *testing.T) {
		result, err := runtime.Execute(`typeof window.localStorage`)
		if err != nil {
			t.Fatalf("Failed: %v", err)
		}
		if result.String() != "object" {
			t.Errorf("Expected 'object', got %s", result.String())
		}
	})

	t.Run("sessionStorage exists on window", func(t *testing.T) {
		result, err := runtime.Execute(`typeof window.sessionStorage`)
		if err != nil {
			t.Fatalf("Failed: %v", err)
		}
		if result.String() != "object" {
			t.Errorf("Expected 'object', got %s", result.String())
		}
	})

	t.Run("localStorage.setItem and getItem work", func(t *testing.T) {
		_, err := runtime.Execute(`localStorage.setItem('testKey', 'testValue')`)
		if err != nil {
			t.Fatalf("setItem failed: %v", err)
		}

		result, err := runtime.Execute(`localStorage.getItem('testKey')`)
		if err != nil {
			t.Fatalf("getItem failed: %v", err)
		}
		if result.String() != "testValue" {
			t.Errorf("Expected 'testValue', got %s", result.String())
		}
	})

	t.Run("localStorage.getItem returns null for non-existent key", func(t *testing.T) {
		result, err := runtime.Execute(`localStorage.getItem('nonExistentKey')`)
		if err != nil {
			t.Fatalf("getItem failed: %v", err)
		}
		if !result.SameAs(runtime.VM().ToValue(nil)) {
			t.Errorf("Expected null, got %v", result)
		}
	})

	t.Run("localStorage.length returns correct count", func(t *testing.T) {
		ClearAllStorage()
		_, _ = runtime.Execute(`localStorage.setItem('key1', 'value1')`)
		_, _ = runtime.Execute(`localStorage.setItem('key2', 'value2')`)

		result, err := runtime.Execute(`localStorage.length`)
		if err != nil {
			t.Fatalf("Failed: %v", err)
		}
		if result.ToInteger() != 2 {
			t.Errorf("Expected 2, got %d", result.ToInteger())
		}
	})

	t.Run("localStorage.key returns key at index", func(t *testing.T) {
		ClearAllStorage()
		_, _ = runtime.Execute(`localStorage.setItem('alpha', 'value1')`)
		_, _ = runtime.Execute(`localStorage.setItem('beta', 'value2')`)

		// Keys are sorted alphabetically
		result, err := runtime.Execute(`localStorage.key(0)`)
		if err != nil {
			t.Fatalf("Failed: %v", err)
		}
		if result.String() != "alpha" {
			t.Errorf("Expected 'alpha', got %s", result.String())
		}

		result, err = runtime.Execute(`localStorage.key(1)`)
		if err != nil {
			t.Fatalf("Failed: %v", err)
		}
		if result.String() != "beta" {
			t.Errorf("Expected 'beta', got %s", result.String())
		}
	})

	t.Run("localStorage.key returns null for out of bounds index", func(t *testing.T) {
		result, err := runtime.Execute(`localStorage.key(999)`)
		if err != nil {
			t.Fatalf("Failed: %v", err)
		}
		if !result.SameAs(runtime.VM().ToValue(nil)) {
			t.Errorf("Expected null, got %v", result)
		}
	})

	t.Run("localStorage.removeItem works", func(t *testing.T) {
		ClearAllStorage()
		_, _ = runtime.Execute(`localStorage.setItem('toRemove', 'value')`)
		_, err := runtime.Execute(`localStorage.removeItem('toRemove')`)
		if err != nil {
			t.Fatalf("removeItem failed: %v", err)
		}

		result, err := runtime.Execute(`localStorage.getItem('toRemove')`)
		if err != nil {
			t.Fatalf("getItem failed: %v", err)
		}
		if !result.SameAs(runtime.VM().ToValue(nil)) {
			t.Errorf("Expected null after removal, got %v", result)
		}
	})

	t.Run("localStorage.clear works", func(t *testing.T) {
		ClearAllStorage()
		_, _ = runtime.Execute(`localStorage.setItem('key1', 'value1')`)
		_, _ = runtime.Execute(`localStorage.setItem('key2', 'value2')`)
		_, err := runtime.Execute(`localStorage.clear()`)
		if err != nil {
			t.Fatalf("clear failed: %v", err)
		}

		result, err := runtime.Execute(`localStorage.length`)
		if err != nil {
			t.Fatalf("Failed: %v", err)
		}
		if result.ToInteger() != 0 {
			t.Errorf("Expected 0 after clear, got %d", result.ToInteger())
		}
	})

	t.Run("localStorage converts values to strings", func(t *testing.T) {
		ClearAllStorage()
		_, err := runtime.Execute(`localStorage.setItem('numKey', 123)`)
		if err != nil {
			t.Fatalf("setItem failed: %v", err)
		}

		result, err := runtime.Execute(`localStorage.getItem('numKey')`)
		if err != nil {
			t.Fatalf("getItem failed: %v", err)
		}
		if result.String() != "123" {
			t.Errorf("Expected '123', got %s", result.String())
		}
	})

	t.Run("sessionStorage works independently of localStorage", func(t *testing.T) {
		ClearAllStorage()
		_, _ = runtime.Execute(`localStorage.setItem('sharedKey', 'localValue')`)
		_, _ = runtime.Execute(`sessionStorage.setItem('sharedKey', 'sessionValue')`)

		result, err := runtime.Execute(`localStorage.getItem('sharedKey')`)
		if err != nil {
			t.Fatalf("Failed: %v", err)
		}
		if result.String() != "localValue" {
			t.Errorf("Expected 'localValue', got %s", result.String())
		}

		result, err = runtime.Execute(`sessionStorage.getItem('sharedKey')`)
		if err != nil {
			t.Fatalf("Failed: %v", err)
		}
		if result.String() != "sessionValue" {
			t.Errorf("Expected 'sessionValue', got %s", result.String())
		}
	})

	t.Run("Storage constructor throws TypeError", func(t *testing.T) {
		_, err := runtime.Execute(`new Storage()`)
		if err == nil {
			t.Error("Expected error for new Storage(), got nil")
		}
	})
}

func TestStorageIsolation(t *testing.T) {
	// Clear storage before test
	ClearAllStorage()

	// Create two executors with different origins
	runtime1 := NewRuntime()
	executor1 := NewScriptExecutor(runtime1)
	doc1 := dom.NewDocument()
	doc1.SetURL("https://example.com/")
	executor1.SetupDocument(doc1)

	runtime2 := NewRuntime()
	executor2 := NewScriptExecutor(runtime2)
	doc2 := dom.NewDocument()
	doc2.SetURL("https://other.com/")
	executor2.SetupDocument(doc2)

	// Set value in first origin
	_, err := runtime1.Execute(`localStorage.setItem('key', 'value1')`)
	if err != nil {
		t.Fatalf("setItem failed: %v", err)
	}

	// Set value in second origin
	_, err = runtime2.Execute(`localStorage.setItem('key', 'value2')`)
	if err != nil {
		t.Fatalf("setItem failed: %v", err)
	}

	// Verify isolation
	result1, err := runtime1.Execute(`localStorage.getItem('key')`)
	if err != nil {
		t.Fatalf("getItem failed: %v", err)
	}
	if result1.String() != "value1" {
		t.Errorf("Expected 'value1' for example.com, got %s", result1.String())
	}

	result2, err := runtime2.Execute(`localStorage.getItem('key')`)
	if err != nil {
		t.Fatalf("getItem failed: %v", err)
	}
	if result2.String() != "value2" {
		t.Errorf("Expected 'value2' for other.com, got %s", result2.String())
	}
}

func TestStoragePersistenceAcrossPageLoads(t *testing.T) {
	// Clear storage before test
	ClearAllStorage()

	// Simulate first page load
	runtime1 := NewRuntime()
	executor1 := NewScriptExecutor(runtime1)
	doc1 := dom.NewDocument()
	doc1.SetURL("https://example.com/page1")
	executor1.SetupDocument(doc1)

	_, err := runtime1.Execute(`localStorage.setItem('persistent', 'value')`)
	if err != nil {
		t.Fatalf("setItem failed: %v", err)
	}

	// Simulate second page load (same origin)
	runtime2 := NewRuntime()
	executor2 := NewScriptExecutor(runtime2)
	doc2 := dom.NewDocument()
	doc2.SetURL("https://example.com/page2")
	executor2.SetupDocument(doc2)

	result, err := runtime2.Execute(`localStorage.getItem('persistent')`)
	if err != nil {
		t.Fatalf("getItem failed: %v", err)
	}
	if result.String() != "value" {
		t.Errorf("Expected 'value' to persist, got %s", result.String())
	}
}

func TestStorageOverwrite(t *testing.T) {
	ClearAllStorage()

	runtime := NewRuntime()
	executor := NewScriptExecutor(runtime)
	doc := dom.NewDocument()
	doc.SetURL("https://example.com/")
	executor.SetupDocument(doc)

	// Set initial value
	_, _ = runtime.Execute(`localStorage.setItem('key', 'oldValue')`)

	// Overwrite value
	_, err := runtime.Execute(`localStorage.setItem('key', 'newValue')`)
	if err != nil {
		t.Fatalf("setItem failed: %v", err)
	}

	result, err := runtime.Execute(`localStorage.getItem('key')`)
	if err != nil {
		t.Fatalf("getItem failed: %v", err)
	}
	if result.String() != "newValue" {
		t.Errorf("Expected 'newValue', got %s", result.String())
	}

	// Length should still be 1
	result, err = runtime.Execute(`localStorage.length`)
	if err != nil {
		t.Fatalf("Failed: %v", err)
	}
	if result.ToInteger() != 1 {
		t.Errorf("Expected length 1, got %d", result.ToInteger())
	}
}
