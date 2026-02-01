// Command wptrun runs Web Platform Tests against Viberowser.
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/chrisuehlinger/viberowser/wpt"
)

func main() {
	wptPath := flag.String("wpt-path", "", "Path to WPT repository")
	baseURL := flag.String("base-url", "http://localhost:8000", "Base URL for WPT server")
	timeout := flag.Duration("timeout", 30e9, "Test timeout duration")
	jsonOutput := flag.Bool("json", false, "Output results as JSON")
	flag.Parse()

	if flag.NArg() < 1 {
		fmt.Fprintf(os.Stderr, "Usage: %s [options] <test-path>...\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "\nOptions:\n")
		flag.PrintDefaults()
		fmt.Fprintf(os.Stderr, "\nExamples:\n")
		fmt.Fprintf(os.Stderr, "  %s -wpt-path=/path/to/wpt dom/nodes/Node-textContent.html\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  %s -base-url=http://localhost:8000 /dom/nodes/Node-textContent.html\n", os.Args[0])
		os.Exit(1)
	}

	runner := wpt.NewRunner(*wptPath)
	if *baseURL != "" {
		runner.SetBaseURL(*baseURL)
	}
	runner.Timeout = *timeout

	// Run each test file
	for _, testPath := range flag.Args() {
		fmt.Fprintf(os.Stderr, "Running: %s\n", testPath)
		result := runner.RunTestFile(testPath)
		runner.Results = append(runner.Results, result)

		if !*jsonOutput {
			printResult(result)
		}
	}

	// Output summary or JSON
	if *jsonOutput {
		jsonData, err := runner.ExportJSON()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error exporting JSON: %v\n", err)
			os.Exit(1)
		}
		fmt.Println(string(jsonData))
	} else {
		passed, failed, skipped := runner.Summary()
		fmt.Printf("\nSummary: %d passed, %d failed, %d skipped\n", passed, failed, skipped)
	}

	// Exit with error code if any tests failed
	_, failed, _ := runner.Summary()
	if failed > 0 {
		os.Exit(1)
	}
}

func printResult(result wpt.TestSuiteResult) {
	fmt.Printf("\n%s (%s, %.2fs)\n", result.TestFile, result.HarnessStatus, result.Duration.Seconds())
	if result.Error != "" {
		fmt.Printf("  ERROR: %s\n", result.Error)
	}
	for _, test := range result.Tests {
		status := statusSymbol(test.Status)
		fmt.Printf("  %s %s\n", status, test.Name)
		if test.Message != "" && test.Status != wpt.StatusPass {
			// Indent the message
			for _, line := range strings.Split(test.Message, "\n") {
				fmt.Printf("      %s\n", line)
			}
		}
	}
}

func statusSymbol(status wpt.TestStatus) string {
	switch status {
	case wpt.StatusPass:
		return "✓"
	case wpt.StatusFail:
		return "✗"
	case wpt.StatusTimeout:
		return "⏱"
	case wpt.StatusError:
		return "!"
	case wpt.StatusSkip:
		return "-"
	default:
		return "?"
	}
}

// findTests finds test files matching a pattern.
func findTests(wptPath, pattern string) ([]string, error) {
	if wptPath == "" {
		return nil, fmt.Errorf("WPT path required for pattern matching")
	}

	matches, err := filepath.Glob(filepath.Join(wptPath, pattern))
	if err != nil {
		return nil, err
	}

	// Convert to relative paths
	var tests []string
	for _, match := range matches {
		rel, err := filepath.Rel(wptPath, match)
		if err != nil {
			continue
		}
		// Only include .html files that look like tests
		if strings.HasSuffix(rel, ".html") && !strings.Contains(rel, "/support/") {
			tests = append(tests, "/"+rel)
		}
	}
	return tests, nil
}
