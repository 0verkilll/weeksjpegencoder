// Package weeksjpegencoder provides F5/James-compatible JPEG encoding functionality.
//
// This file implements diff reporting and diagnostics utilities for byte-level
// compatibility testing between Go and Java JPEG encoders.
//
// Features:
//   - Hex dump diff utility with configurable context (bytes before/after)
//   - Marker context identification (which segment contains each difference)
//   - Detailed diff reports saved to testdata/diffs/
//   - Summary reports listing all differences prioritized by segment type
//   - Descriptive file naming (e.g., solid_64x64_q50_420_diff.txt)

package weeksjpegencoder

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"
)

// =============================================================================
// Task 7.1: Directory Structure Management
// =============================================================================

const (
	// diffsDir is the directory for storing diff reports
	diffsDir = "testdata/diffs"
	// hexDumpContextBytes is the number of bytes to show before/after differences
	hexDumpContextBytes = 16
	// hexDumpBytesPerLine is the number of bytes per line in hex dumps
	hexDumpBytesPerLine = 16
)

var _ = diffsDir // Used in various test functions

// EnsureDiffsDirectory creates the testdata/diffs/ directory if it doesn't exist.
func EnsureDiffsDirectory() error {
	return os.MkdirAll(diffsDir, 0755)
}

// GetDiffsDirectory returns the path to the diffs directory.
//
//goland:noinspection GoUnusedExportedFunction
func GetDiffsDirectory() string {
	return diffsDir
}

// =============================================================================
// Task 7.2: Hex Dump Diff Utility
// =============================================================================

// HexDumpLine represents a single line of hex dump output.
type HexDumpLine struct {
	Offset    int
	HexBytes  string
	ASCIIView string
}

// HexDumpRegion represents a hex dump of a specific byte region.
type HexDumpRegion struct {
	StartOffset int
	EndOffset   int
	Lines       []HexDumpLine
	Label       string
}

// FormatHexByte formats a byte as a two-digit hex string with optional highlighting.
func FormatHexByte(b byte) string {
	return fmt.Sprintf("%02X", b)
}

// GenerateHexDumpLines creates hex dump lines for a byte slice.
func GenerateHexDumpLines(data []byte, startOffset int) []HexDumpLine {
	var lines []HexDumpLine

	for i := 0; i < len(data); i += hexDumpBytesPerLine {
		line := HexDumpLine{
			Offset: startOffset + i,
		}

		// Build hex bytes string
		var hexParts []string
		var asciiParts []byte

		for j := 0; j < hexDumpBytesPerLine; j++ {
			idx := i + j
			if idx < len(data) {
				hexParts = append(hexParts, FormatHexByte(data[idx]))
				// ASCII representation
				if data[idx] >= 32 && data[idx] < 127 {
					asciiParts = append(asciiParts, data[idx])
				} else {
					asciiParts = append(asciiParts, '.')
				}
			} else {
				hexParts = append(hexParts, "  ")
				asciiParts = append(asciiParts, ' ')
			}
		}

		// Format with grouping (8 bytes | 8 bytes)
		if len(hexParts) > 8 {
			line.HexBytes = strings.Join(hexParts[:8], " ") + "  " + strings.Join(hexParts[8:], " ")
		} else {
			line.HexBytes = strings.Join(hexParts, " ")
		}
		line.ASCIIView = string(asciiParts)

		lines = append(lines, line)
	}

	return lines
}

// FormatHexDump formats a hex dump region as a string.
func FormatHexDump(region HexDumpRegion) string {
	var sb strings.Builder

	if region.Label != "" {
		fmt.Fprintf(&sb, "--- %s (offset 0x%04X - 0x%04X) ---\n",
			region.Label, region.StartOffset, region.EndOffset)
	}

	for _, line := range region.Lines {
		fmt.Fprintf(&sb, "%08X  %s  |%s|\n",
			line.Offset, line.HexBytes, line.ASCIIView)
	}

	return sb.String()
}

// GenerateHexDumpAroundDiff creates a hex dump showing bytes around a difference.
func GenerateHexDumpAroundDiff(data []byte, diffOffset int, contextBytes int, label string) HexDumpRegion {
	// Align to line boundary for cleaner output
	startLine := (diffOffset - contextBytes) / hexDumpBytesPerLine * hexDumpBytesPerLine
	if startLine < 0 {
		startLine = 0
	}

	endLine := (diffOffset + contextBytes) / hexDumpBytesPerLine * hexDumpBytesPerLine
	endLine += hexDumpBytesPerLine // Include the line containing the difference
	if endLine > len(data) {
		endLine = len(data)
	}

	region := HexDumpRegion{
		StartOffset: startLine,
		EndOffset:   endLine,
		Label:       label,
	}

	if startLine < len(data) {
		region.Lines = GenerateHexDumpLines(data[startLine:endLine], startLine)
	}

	return region
}

// GenerateComparativeHexDump creates a side-by-side hex dump comparison.
func GenerateComparativeHexDump(expected, actual []byte, diffOffset int, contextBytes int) string {
	var sb strings.Builder

	expectedRegion := GenerateHexDumpAroundDiff(expected, diffOffset, contextBytes, "Expected (Java)")
	actualRegion := GenerateHexDumpAroundDiff(actual, diffOffset, contextBytes, "Actual (Go)")

	sb.WriteString(FormatHexDump(expectedRegion))
	sb.WriteString("\n")
	sb.WriteString(FormatHexDump(actualRegion))

	return sb.String()
}

// =============================================================================
// Task 7.3: Marker Context Identification
// =============================================================================

// MarkerContextInfo provides detailed information about which marker segment
// contains a specific byte offset.
type MarkerContextInfo struct {
	MarkerName      string // e.g., "SOI", "APP0", "DQT", "DHT", "SOS", "EntropyData", "EOI"
	MarkerType      byte   // The marker type byte (e.g., 0xD8 for SOI)
	SegmentStart    int    // Start offset of the segment
	SegmentEnd      int    // End offset of the segment
	OffsetInSegment int    // Offset within the segment
	Description     string // Human-readable description
}

// IdentifyMarkerContext identifies which JPEG marker segment contains the given offset.
func IdentifyMarkerContext(data []byte, offset int) MarkerContextInfo {
	markers, err := ParseJPEGMarkers(data)
	if err != nil {
		return MarkerContextInfo{
			MarkerName:  "ParseError",
			Description: fmt.Sprintf("Failed to parse markers: %v", err),
		}
	}

	// Build segment ranges
	for i, m := range markers {
		var segStart, segEnd int
		segStart = m.Offset

		switch m.Type {
		case testMarkerSOI, testMarkerEOI:
			segEnd = m.Offset + 2
		case testMarkerSOS:
			// SOS segment includes header plus entropy data until EOI
			segEnd = m.Offset + 2 + m.Length
			eoi := FindMarker(markers, testMarkerEOI)
			if eoi != nil {
				// Check if offset is in SOS header or entropy data
				sosHeaderEnd := m.Offset + 2 + m.Length
				if offset >= segStart && offset < sosHeaderEnd {
					return MarkerContextInfo{
						MarkerName:      "SOS",
						MarkerType:      testMarkerSOS,
						SegmentStart:    segStart,
						SegmentEnd:      sosHeaderEnd,
						OffsetInSegment: offset - segStart,
						Description:     "Start of Scan header (component selectors, Huffman table assignments)",
					}
				}
				if offset >= sosHeaderEnd && offset < eoi.Offset {
					return MarkerContextInfo{
						MarkerName:      "EntropyData",
						MarkerType:      testMarkerSOS,
						SegmentStart:    sosHeaderEnd,
						SegmentEnd:      eoi.Offset,
						OffsetInSegment: offset - sosHeaderEnd,
						Description:     "Entropy-coded data (Huffman-encoded DCT coefficients)",
					}
				}
			}
		default:
			segEnd = m.Offset + 2 + m.Length
		}

		if offset >= segStart && offset < segEnd {
			return MarkerContextInfo{
				MarkerName:      m.TypeName(),
				MarkerType:      m.Type,
				SegmentStart:    segStart,
				SegmentEnd:      segEnd,
				OffsetInSegment: offset - segStart,
				Description:     getMarkerDescription(m.Type),
			}
		}

		// Handle gap between markers (shouldn't happen in valid JPEG)
		if i+1 < len(markers) {
			nextStart := markers[i+1].Offset
			if offset >= segEnd && offset < nextStart {
				return MarkerContextInfo{
					MarkerName:  "Unknown",
					Description: fmt.Sprintf("Gap between %s and %s", m.TypeName(), markers[i+1].TypeName()),
				}
			}
		}
	}

	return MarkerContextInfo{
		MarkerName:  "Unknown",
		Description: "Offset outside known marker segments",
	}
}

// getMarkerDescription returns a human-readable description for a marker type.
func getMarkerDescription(markerType byte) string {
	descriptions := map[byte]string{
		testMarkerSOI:  "Start of Image marker (file header)",
		testMarkerEOI:  "End of Image marker (file terminator)",
		testMarkerAPP0: "JFIF Application marker (version, density, thumbnail)",
		testMarkerCOM:  "Comment marker (F5/James signature)",
		testMarkerDQT:  "Define Quantization Table (luminance and chrominance tables)",
		testMarkerSOF0: "Start of Frame (baseline DCT) - dimensions, sampling factors",
		testMarkerDHT:  "Define Huffman Table (DC/AC luminance and chrominance tables)",
		testMarkerSOS:  "Start of Scan (component selectors, spectral selection)",
	}

	if desc, ok := descriptions[markerType]; ok {
		return desc
	}
	return fmt.Sprintf("Unknown marker type 0x%02X", markerType)
}

// =============================================================================
// Task 7.4 & 7.5: Detailed Diff Report Generation
// =============================================================================

// DetailedDifference represents a single byte difference with full context.
type DetailedDifference struct {
	FileOffset    int               // Absolute offset in the file
	ExpectedValue byte              // Expected byte value (Java)
	ActualValue   byte              // Actual byte value (Go)
	MarkerContext MarkerContextInfo // Which segment contains this difference
	HexDump       string            // Hex dump context around the difference
}

// DetailedDiffReport contains comprehensive diff information.
type DetailedDiffReport struct {
	TestName       string
	Timestamp      time.Time
	JavaFileName   string
	GoDescription  string
	JavaFileSize   int
	GoFileSize     int
	JavaSHA256     string
	GoSHA256       string
	TotalDiffs     int
	Differences    []DetailedDifference
	SegmentSummary map[string]int // Count of differences by segment
}

// GenerateDetailedDiffReport creates a comprehensive diff report.
func GenerateDetailedDiffReport(testName string, javaData, goData []byte) *DetailedDiffReport {
	report := &DetailedDiffReport{
		TestName:       testName,
		Timestamp:      time.Now(),
		JavaFileName:   testName + ".jpg (Java reference)",
		GoDescription:  "Go weeksjpegencoder output",
		JavaFileSize:   len(javaData),
		GoFileSize:     len(goData),
		JavaSHA256:     ComputeSHA256(javaData),
		GoSHA256:       ComputeSHA256(goData),
		SegmentSummary: make(map[string]int),
	}

	// Find all differences
	maxLen := len(javaData)
	if len(goData) > maxLen {
		maxLen = len(goData)
	}

	for i := 0; i < maxLen; i++ {
		var exp, act byte
		if i < len(javaData) {
			exp = javaData[i]
		}
		if i < len(goData) {
			act = goData[i]
		}

		if exp != act || i >= len(javaData) || i >= len(goData) {
			context := IdentifyMarkerContext(javaData, i)
			diff := DetailedDifference{
				FileOffset:    i,
				ExpectedValue: exp,
				ActualValue:   act,
				MarkerContext: context,
			}

			// Generate hex dump for first 20 differences only (to keep report size manageable)
			if len(report.Differences) < 20 {
				diff.HexDump = GenerateComparativeHexDump(javaData, goData, i, hexDumpContextBytes)
			}

			report.Differences = append(report.Differences, diff)
			report.SegmentSummary[context.MarkerName]++
		}
	}

	report.TotalDiffs = len(report.Differences)
	return report
}

// FormatDetailedReport formats the detailed diff report as a string.
func (r *DetailedDiffReport) FormatDetailedReport() string {
	var sb strings.Builder

	// Header
	sb.WriteString("================================================================================\n")
	fmt.Fprintf(&sb, "JPEG Byte-Level Diff Report: %s\n", r.TestName)
	sb.WriteString("================================================================================\n")
	fmt.Fprintf(&sb, "Generated: %s\n\n", r.Timestamp.Format(time.RFC3339))

	// File Information
	sb.WriteString("FILE INFORMATION\n")
	sb.WriteString("----------------\n")
	fmt.Fprintf(&sb, "Java Reference: %s\n", r.JavaFileName)
	fmt.Fprintf(&sb, "Go Output:      %s\n", r.GoDescription)
	fmt.Fprintf(&sb, "Java Size:      %d bytes\n", r.JavaFileSize)
	fmt.Fprintf(&sb, "Go Size:        %d bytes\n", r.GoFileSize)
	fmt.Fprintf(&sb, "Size Delta:     %+d bytes\n\n", r.GoFileSize-r.JavaFileSize)

	// Checksums
	sb.WriteString("SHA-256 CHECKSUMS\n")
	sb.WriteString("-----------------\n")
	fmt.Fprintf(&sb, "Java: %s\n", r.JavaSHA256)
	fmt.Fprintf(&sb, "Go:   %s\n", r.GoSHA256)
	if r.JavaSHA256 == r.GoSHA256 {
		sb.WriteString("Status: MATCH\n\n")
	} else {
		sb.WriteString("Status: MISMATCH\n\n")
	}

	// Summary by Segment (Task 7.6 - prioritized by segment)
	sb.WriteString("DIFFERENCES BY SEGMENT (Prioritized)\n")
	sb.WriteString("------------------------------------\n")
	fmt.Fprintf(&sb, "Total Differences: %d\n\n", r.TotalDiffs)

	// Sort segments by priority for debugging
	segmentPriority := []string{
		"SOI", "APP0", "COM", "DQT", "SOF0", "DHT", "SOS", "EntropyData", "EOI", "Unknown",
	}

	for _, seg := range segmentPriority {
		if count, ok := r.SegmentSummary[seg]; ok {
			percentage := 100.0 * float64(count) / float64(r.TotalDiffs)
			fmt.Fprintf(&sb, "  %-15s: %5d differences (%5.1f%%)\n", seg, count, percentage)
		}
	}
	// Any segments not in priority list
	for seg, count := range r.SegmentSummary {
		found := false
		for _, p := range segmentPriority {
			if seg == p {
				found = true
				break
			}
		}
		if !found {
			percentage := 100.0 * float64(count) / float64(r.TotalDiffs)
			fmt.Fprintf(&sb, "  %-15s: %5d differences (%5.1f%%)\n", seg, count, percentage)
		}
	}
	sb.WriteString("\n")

	// Detailed Differences
	sb.WriteString("DETAILED DIFFERENCES\n")
	sb.WriteString("--------------------\n")
	sb.WriteString("(Showing first 100 differences with hex context for first 20)\n\n")

	for i, diff := range r.Differences {
		if i >= 100 {
			fmt.Fprintf(&sb, "\n... and %d more differences not shown\n", len(r.Differences)-100)
			break
		}

		fmt.Fprintf(&sb, "Difference #%d\n", i+1)
		fmt.Fprintf(&sb, "  File Offset:    0x%08X (%d)\n", diff.FileOffset, diff.FileOffset)
		fmt.Fprintf(&sb, "  Expected:       0x%02X\n", diff.ExpectedValue)
		fmt.Fprintf(&sb, "  Actual:         0x%02X\n", diff.ActualValue)
		fmt.Fprintf(&sb, "  Segment:        %s\n", diff.MarkerContext.MarkerName)
		fmt.Fprintf(&sb, "  Segment Offset: 0x%04X (%d)\n",
			diff.MarkerContext.OffsetInSegment, diff.MarkerContext.OffsetInSegment)
		fmt.Fprintf(&sb, "  Description:    %s\n", diff.MarkerContext.Description)

		if diff.HexDump != "" {
			sb.WriteString("\n  Hex Dump Context:\n")
			// Indent hex dump
			for _, line := range strings.Split(diff.HexDump, "\n") {
				if line != "" {
					sb.WriteString("    " + line + "\n")
				}
			}
		}
		sb.WriteString("\n")
	}

	return sb.String()
}

// =============================================================================
// Task 7.6: Summary Report
// =============================================================================

// DiffSummaryEntry represents one test case in the summary report.
type DiffSummaryEntry struct {
	TestName       string
	TotalDiffs     int
	SegmentSummary map[string]int
	JavaSize       int
	GoSize         int
	PrimarySegment string // Segment with most differences
}

// DiffSummaryReport aggregates diff information across multiple tests.
type DiffSummaryReport struct {
	Timestamp   time.Time
	Entries     []DiffSummaryEntry
	TotalTests  int
	FailedTests int
	PassedTests int
}

// NewDiffSummaryReport creates a new summary report.
func NewDiffSummaryReport() *DiffSummaryReport {
	return &DiffSummaryReport{
		Timestamp: time.Now(),
	}
}

// AddEntry adds a test result to the summary.
func (r *DiffSummaryReport) AddEntry(entry DiffSummaryEntry) {
	r.Entries = append(r.Entries, entry)
	r.TotalTests++
	if entry.TotalDiffs > 0 {
		r.FailedTests++
	} else {
		r.PassedTests++
	}
}

// FormatSummaryReport formats the summary report as a string.
func (r *DiffSummaryReport) FormatSummaryReport() string {
	var sb strings.Builder

	sb.WriteString("================================================================================\n")
	sb.WriteString("JPEG Byte-Level Compatibility Summary Report\n")
	sb.WriteString("================================================================================\n")
	fmt.Fprintf(&sb, "Generated: %s\n\n", r.Timestamp.Format(time.RFC3339))

	// Overall Statistics
	sb.WriteString("OVERALL STATISTICS\n")
	sb.WriteString("------------------\n")
	fmt.Fprintf(&sb, "Total Tests:  %d\n", r.TotalTests)
	fmt.Fprintf(&sb, "Passed:       %d (%.1f%%)\n", r.PassedTests, 100.0*float64(r.PassedTests)/float64(r.TotalTests))
	fmt.Fprintf(&sb, "Failed:       %d (%.1f%%)\n\n", r.FailedTests, 100.0*float64(r.FailedTests)/float64(r.TotalTests))

	// Aggregate segment statistics
	segmentTotals := make(map[string]int)
	for _, entry := range r.Entries {
		for seg, count := range entry.SegmentSummary {
			segmentTotals[seg] += count
		}
	}

	sb.WriteString("DIFFERENCES BY SEGMENT (All Tests Combined)\n")
	sb.WriteString("-------------------------------------------\n")

	// Sort by count descending
	type segCount struct {
		name  string
		count int
	}
	var sorted []segCount
	for name, count := range segmentTotals {
		sorted = append(sorted, segCount{name, count})
	}
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].count > sorted[j].count
	})

	totalDiffs := 0
	for _, sc := range sorted {
		totalDiffs += sc.count
	}

	for _, sc := range sorted {
		percentage := 0.0
		if totalDiffs > 0 {
			percentage = 100.0 * float64(sc.count) / float64(totalDiffs)
		}
		fmt.Fprintf(&sb, "  %-15s: %6d differences (%5.1f%%)\n", sc.name, sc.count, percentage)
	}
	fmt.Fprintf(&sb, "  %-15s: %6d total\n\n", "TOTAL", totalDiffs)

	// Failed Tests Detail
	if r.FailedTests > 0 {
		sb.WriteString("FAILED TESTS DETAIL\n")
		sb.WriteString("-------------------\n")

		// Sort by total diffs descending
		sortedEntries := make([]DiffSummaryEntry, len(r.Entries))
		copy(sortedEntries, r.Entries)
		sort.Slice(sortedEntries, func(i, j int) bool {
			return sortedEntries[i].TotalDiffs > sortedEntries[j].TotalDiffs
		})

		for _, entry := range sortedEntries {
			if entry.TotalDiffs > 0 {
				fmt.Fprintf(&sb, "\n%s\n", entry.TestName)
				fmt.Fprintf(&sb, "  Total Diffs:    %d\n", entry.TotalDiffs)
				fmt.Fprintf(&sb, "  Primary Segment: %s\n", entry.PrimarySegment)
				fmt.Fprintf(&sb, "  Size (Java/Go): %d / %d bytes\n", entry.JavaSize, entry.GoSize)
				sb.WriteString("  By Segment:\n")
				for seg, count := range entry.SegmentSummary {
					fmt.Fprintf(&sb, "    %-12s: %d\n", seg, count)
				}
			}
		}
	}

	return sb.String()
}

// =============================================================================
// File Saving Utilities
// =============================================================================

// SaveDetailedDiffReport saves a detailed diff report to the diffs directory.
func SaveDetailedDiffReport(report *DetailedDiffReport) error {
	if err := EnsureDiffsDirectory(); err != nil {
		return err
	}

	// Generate descriptive filename: solid_64x64_q50_420_diff.txt
	filename := fmt.Sprintf("%s_diff.txt", report.TestName)
	fullPath := filepath.Join(diffsDir, filename)

	content := report.FormatDetailedReport()
	return os.WriteFile(fullPath, []byte(content), 0644)
}

// SaveSummaryReport saves the summary report to the diffs directory.
func SaveSummaryReport(report *DiffSummaryReport) error {
	if err := EnsureDiffsDirectory(); err != nil {
		return err
	}

	fullPath := filepath.Join(diffsDir, "summary_report.txt")
	content := report.FormatSummaryReport()
	return os.WriteFile(fullPath, []byte(content), 0644)
}

// =============================================================================
// Test Functions
// =============================================================================

// TestEnsureDiffsDirectory verifies the diffs directory can be created.
func TestEnsureDiffsDirectory(t *testing.T) {
	err := EnsureDiffsDirectory()
	if err != nil {
		t.Fatalf("Failed to create diffs directory: %v", err)
	}

	// Verify directory exists
	info, err := os.Stat(diffsDir)
	if err != nil {
		t.Fatalf("Diffs directory does not exist: %v", err)
	}
	if !info.IsDir() {
		t.Fatalf("Diffs path is not a directory")
	}

	t.Logf("Diffs directory verified: %s", diffsDir)
}

// TestHexDumpGeneration tests the hex dump utility.
func TestHexDumpGeneration(t *testing.T) {
	// Test data
	testData := []byte{
		0x00, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07,
		0x08, 0x09, 0x0A, 0x0B, 0x0C, 0x0D, 0x0E, 0x0F,
		0xFF, 0xD8, 0xFF, 0xE0, 'J', 'F', 'I', 'F',
		0x00, 0x01, 0x02, 0x48, 0x65, 0x6C, 0x6C, 0x6F,
	}

	lines := GenerateHexDumpLines(testData, 0)

	if len(lines) != 2 {
		t.Errorf("Expected 2 lines, got %d", len(lines))
	}

	// Verify first line
	if lines[0].Offset != 0 {
		t.Errorf("First line offset: got %d, want 0", lines[0].Offset)
	}

	// Log output for visual verification
	region := HexDumpRegion{
		StartOffset: 0,
		EndOffset:   len(testData),
		Lines:       lines,
		Label:       "Test Data",
	}
	t.Log("\n" + FormatHexDump(region))
}

// TestMarkerContextIdentification tests marker context identification.
func TestMarkerContextIdentification(t *testing.T) {
	// Generate a test JPEG
	goData, err := generateGoImage(PatternSolid, 64, 64, 75)
	if err != nil {
		t.Fatalf("Failed to generate test image: %v", err)
	}

	// Test various offsets
	testCases := []struct {
		offset       int
		expectedName string
	}{
		{0, "SOI"},
		{2, "APP0"},
		// Test more offsets if markers are found
	}

	markers, _ := ParseJPEGMarkers(goData)
	t.Logf("Found %d markers in test image", len(markers))
	for _, m := range markers {
		t.Logf("  %s at offset %d (length %d)", m.TypeName(), m.Offset, m.Length)
	}

	for _, tc := range testCases {
		context := IdentifyMarkerContext(goData, tc.offset)
		if context.MarkerName != tc.expectedName {
			t.Errorf("Offset %d: expected %s, got %s", tc.offset, tc.expectedName, context.MarkerName)
		}
		t.Logf("Offset %d: %s - %s", tc.offset, context.MarkerName, context.Description)
	}
}

// TestDetailedDiffReportGeneration tests detailed diff report generation.
func TestDetailedDiffReportGeneration(t *testing.T) {
	// Load Java reference
	javaData, err := loadReferenceImage("solid", 64, 64, 75)
	if err != nil {
		t.Skipf("Reference image not found: %v", err)
		return
	}

	// Generate Go output
	goData, err := generateGoImage(PatternSolid, 64, 64, 75)
	if err != nil {
		t.Fatalf("Failed to generate Go image: %v", err)
	}

	// Generate detailed report
	report := GenerateDetailedDiffReport("solid_64x64_q75_420", javaData, goData)

	t.Logf("Generated report for %s", report.TestName)
	t.Logf("  Total differences: %d", report.TotalDiffs)
	t.Logf("  Java size: %d, Go size: %d", report.JavaFileSize, report.GoFileSize)

	for seg, count := range report.SegmentSummary {
		t.Logf("  %s: %d differences", seg, count)
	}

	// Save the report
	err = SaveDetailedDiffReport(report)
	if err != nil {
		t.Errorf("Failed to save report: %v", err)
	} else {
		t.Logf("Report saved to %s/%s_diff.txt", diffsDir, report.TestName)
	}
}

// TestGenerateAllDiffReports generates diff reports for all test cases with differences.
func TestGenerateAllDiffReports(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping in short mode")
	}

	summaryReport := NewDiffSummaryReport()

	patterns := AllPatternTypes()
	dimensions := []TestDimension{
		{8, 8},
		{64, 64},
		{256, 256},
		{33, 33},
		{100, 75},
	}
	qualities := []int{1, 10, 25, 50, 75, 90, 95, 100}

	for _, pattern := range patterns {
		for _, dim := range dimensions {
			for _, quality := range qualities {
				testName := fmt.Sprintf("%s_%dx%d_q%02d_420",
					pattern.PatternName(), dim.Width, dim.Height, quality)

				// Load Java reference
				javaData, err := loadReferenceImage(pattern.PatternName(), dim.Width, dim.Height, quality)
				if err != nil {
					continue // Skip if reference not found
				}

				// Generate Go output
				goData, err := generateGoImage(pattern, dim.Width, dim.Height, quality)
				if err != nil {
					t.Logf("Failed to generate %s: %v", testName, err)
					continue
				}

				// Check if files differ
				if ComputeSHA256(javaData) == ComputeSHA256(goData) {
					// Files match - add to summary as passed
					summaryReport.AddEntry(DiffSummaryEntry{
						TestName:       testName,
						TotalDiffs:     0,
						SegmentSummary: map[string]int{},
						JavaSize:       len(javaData),
						GoSize:         len(goData),
					})
					continue
				}

				// Generate and save detailed report
				report := GenerateDetailedDiffReport(testName, javaData, goData)
				err = SaveDetailedDiffReport(report)
				if err != nil {
					t.Logf("Failed to save report for %s: %v", testName, err)
				}

				// Find primary segment (most differences)
				primarySeg := ""
				maxCount := 0
				for seg, count := range report.SegmentSummary {
					if count > maxCount {
						maxCount = count
						primarySeg = seg
					}
				}

				// Add to summary
				summaryReport.AddEntry(DiffSummaryEntry{
					TestName:       testName,
					TotalDiffs:     report.TotalDiffs,
					SegmentSummary: report.SegmentSummary,
					JavaSize:       report.JavaFileSize,
					GoSize:         report.GoFileSize,
					PrimarySegment: primarySeg,
				})
			}
		}
	}

	// Save summary report
	err := SaveSummaryReport(summaryReport)
	if err != nil {
		t.Errorf("Failed to save summary report: %v", err)
	} else {
		t.Logf("Summary report saved to %s/summary_report.txt", diffsDir)
	}

	// Log summary
	t.Logf("Total tests: %d, Passed: %d, Failed: %d",
		summaryReport.TotalTests, summaryReport.PassedTests, summaryReport.FailedTests)
}

// TestComparativeHexDump tests the comparative hex dump generation.
func TestComparativeHexDump(t *testing.T) {
	// Create test data with a known difference
	expected := []byte{
		0xFF, 0xD8, 0xFF, 0xE0, 0x00, 0x10, 0x4A, 0x46,
		0x49, 0x46, 0x00, 0x01, 0x01, 0x00, 0x00, 0x01,
		0x00, 0x01, 0x00, 0x00, 0xFF, 0xDB, 0x00, 0x43,
	}

	actual := make([]byte, len(expected))
	copy(actual, expected)
	actual[10] = 0x01 // Create a difference at offset 10

	hexDump := GenerateComparativeHexDump(expected, actual, 10, 8)

	// Verify output is not empty
	if len(hexDump) == 0 {
		t.Error("Hex dump is empty")
	}

	t.Log("Comparative hex dump:\n" + hexDump)
}

// TestSummaryReportGeneration tests summary report generation.
func TestSummaryReportGeneration(t *testing.T) {
	summary := NewDiffSummaryReport()

	// Add some test entries
	summary.AddEntry(DiffSummaryEntry{
		TestName:   "test1",
		TotalDiffs: 0,
	})

	summary.AddEntry(DiffSummaryEntry{
		TestName:       "test2",
		TotalDiffs:     100,
		SegmentSummary: map[string]int{"DHT": 50, "EntropyData": 50},
		PrimarySegment: "DHT",
		JavaSize:       1000,
		GoSize:         900,
	})

	report := summary.FormatSummaryReport()

	if !strings.Contains(report, "Total Tests:  2") {
		t.Error("Summary report missing total tests count")
	}

	if !strings.Contains(report, "Passed:       1") {
		t.Error("Summary report missing passed count")
	}

	if !strings.Contains(report, "Failed:       1") {
		t.Error("Summary report missing failed count")
	}

	t.Log(report)
}
