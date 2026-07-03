package main

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

// TestSuite represents a test suite configuration
type TestSuite struct {
	Name        string
	Path        string
	Tags        []string
	Timeout     time.Duration
	Description string
}

func main() {
	fmt.Println("🚀 PlatformCTL Test Suite Runner")
	fmt.Println("================================")

	// Define test suites
	testSuites := []TestSuite{
		{
			Name:        "Unit Tests - Domain Models",
			Path:        "./internal/models",
			Tags:        []string{"unit", "models"},
			Timeout:     2 * time.Minute,
			Description: "Tests for App, Environment, and Context domain models including JSON marshaling, validation tags, and field relationships",
		},
		{
			Name:        "Unit Tests - Validation Logic",
			Path:        "./internal/validation",
			Tags:        []string{"unit", "validation"},
			Timeout:     2 * time.Minute,
			Description: "Tests for manifest validation including DNS names, semantic versions, emails, URLs, and business logic validation",
		},
		{
			Name:        "Integration Tests - Storage Layer",
			Path:        "./internal/storage",
			Tags:        []string{"integration", "storage"},
			Timeout:     5 * time.Minute,
			Description: "Tests for CRUD operations with PostgreSQL database including concurrent operations and complex data structures",
		},
		{
			Name:        "End-to-End Integration Tests",
			Path:        "./internal/integration",
			Tags:        []string{"integration", "e2e"},
			Timeout:     10 * time.Minute,
			Description: "Comprehensive tests for complete App+Environment+Context CRUD workflow demonstrating three-manifest architecture",
		},
	}

	// Parse command line arguments
	args := os.Args[1:]
	runAll := len(args) == 0 || contains(args, "all")
	verbose := contains(args, "-v") || contains(args, "--verbose")
	short := contains(args, "-short")

	if contains(args, "-h") || contains(args, "--help") {
		printUsage()
		return
	}

	fmt.Printf("Test Configuration:\n")
	fmt.Printf("- Verbose: %v\n", verbose)
	fmt.Printf("- Short mode: %v\n", short)
	fmt.Printf("- Database: PostgreSQL (required for integration tests)\n")
	fmt.Println()

	// Check if we should run specific test suites
	var suitesToRun []TestSuite
	if runAll {
		suitesToRun = testSuites
	} else {
		for _, arg := range args {
			for _, suite := range testSuites {
				if strings.Contains(strings.ToLower(suite.Name), strings.ToLower(arg)) ||
					contains(suite.Tags, strings.ToLower(arg)) {
					suitesToRun = append(suitesToRun, suite)
					break
				}
			}
		}
	}

	if len(suitesToRun) == 0 {
		fmt.Println("❌ No matching test suites found")
		printUsage()
		return
	}

	// Run test suites
	fmt.Printf("Running %d test suite(s):\n", len(suitesToRun))
	for i, suite := range suitesToRun {
		fmt.Printf("%d. %s\n", i+1, suite.Name)
	}
	fmt.Println()

	totalStart := time.Now()
	results := make([]TestResult, 0, len(suitesToRun))

	for _, suite := range suitesToRun {
		result := runTestSuite(suite, verbose, short)
		results = append(results, result)
	}

	// Print summary
	printSummary(results, time.Since(totalStart))
}

type TestResult struct {
	Suite    TestSuite
	Success  bool
	Duration time.Duration
	Output   string
	Error    string
}

func runTestSuite(suite TestSuite, verbose, short bool) TestResult {
	start := time.Now()
	fmt.Printf("🧪 Running: %s\n", suite.Name)
	fmt.Printf("   Path: %s\n", suite.Path)
	fmt.Printf("   Description: %s\n", suite.Description)

	// Build test command
	args := []string{"test"}

	if verbose {
		args = append(args, "-v")
	}

	if short {
		args = append(args, "-short")
	}

	// Add timeout
	args = append(args, "-timeout", suite.Timeout.String())

	// Add coverage for unit tests
	if contains(suite.Tags, "unit") {
		args = append(args, "-cover")
	}

	// Add race detection
	args = append(args, "-race")

	// Add path
	args = append(args, suite.Path)

	cmd := exec.Command("go", args...)

	// Set environment variables
	cmd.Env = append(os.Environ(),
		"TEST_DATABASE_URL=postgres://postgres:password@localhost:5432/platformctl_test?sslmode=disable",
	)

	output, err := cmd.CombinedOutput()
	duration := time.Since(start)

	result := TestResult{
		Suite:    suite,
		Success:  err == nil,
		Duration: duration,
		Output:   string(output),
	}

	if err != nil {
		result.Error = err.Error()
		fmt.Printf("   ❌ FAILED (%v): %s\n", duration.Round(time.Millisecond), err.Error())
	} else {
		fmt.Printf("   ✅ PASSED (%v)\n", duration.Round(time.Millisecond))
	}

	if verbose || !result.Success {
		fmt.Println("   Output:")
		for _, line := range strings.Split(result.Output, "\n") {
			if strings.TrimSpace(line) != "" {
				fmt.Printf("     %s\n", line)
			}
		}
	}

	fmt.Println()
	return result
}

func printSummary(results []TestResult, totalDuration time.Duration) {
	fmt.Println("📊 Test Summary")
	fmt.Println("===============")

	passed := 0
	failed := 0

	for _, result := range results {
		status := "✅ PASS"
		if !result.Success {
			status = "❌ FAIL"
			failed++
		} else {
			passed++
		}

		fmt.Printf("%s %-40s (%v)\n",
			status,
			result.Suite.Name,
			result.Duration.Round(time.Millisecond))

		if !result.Success {
			fmt.Printf("     Error: %s\n", result.Error)
		}
	}

	fmt.Println()
	fmt.Printf("Results: %d passed, %d failed, %d total\n", passed, failed, len(results))
	fmt.Printf("Total time: %v\n", totalDuration.Round(time.Millisecond))

	if failed > 0 {
		fmt.Println("\n❌ Some tests failed. Check the output above for details.")
		os.Exit(1)
	} else {
		fmt.Println("\n🎉 All tests passed!")
	}
}

func printUsage() {
	fmt.Println("\nUsage: go run test/run_tests.go [options] [test-suite-filters...]")
	fmt.Println("\nOptions:")
	fmt.Println("  -h, --help     Show this help message")
	fmt.Println("  -v, --verbose  Enable verbose output")
	fmt.Println("  -short         Run tests in short mode (skip integration tests)")
	fmt.Println("\nTest Suite Filters:")
	fmt.Println("  all            Run all test suites (default)")
	fmt.Println("  unit           Run only unit tests")
	fmt.Println("  integration    Run only integration tests")
	fmt.Println("  e2e            Run only end-to-end tests")
	fmt.Println("  models         Run domain model tests")
	fmt.Println("  validation     Run validation tests")
	fmt.Println("  storage        Run storage layer tests")
	fmt.Println("\nExamples:")
	fmt.Println("  go run test/run_tests.go                    # Run all tests")
	fmt.Println("  go run test/run_tests.go unit               # Run unit tests only")
	fmt.Println("  go run test/run_tests.go -v integration     # Run integration tests with verbose output")
	fmt.Println("  go run test/run_tests.go -short models      # Run model tests in short mode")
	fmt.Println("\nPrerequisites:")
	fmt.Println("  - PostgreSQL database running on localhost:5432")
	fmt.Println("  - Database user 'postgres' with password 'password'")
	fmt.Println("  - Test database 'platformctl_test' (will be created if not exists)")
}

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}
