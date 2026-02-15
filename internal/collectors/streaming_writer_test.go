package collectors

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestStreamingJobFileWriter_Write(t *testing.T) {
	tmpDir := t.TempDir()

	writer, err := NewStreamingJobFileWriter(tmpDir, 10)
	if err != nil {
		t.Fatalf("Failed to create writer: %v", err)
	}
	defer writer.Close()

	// Write test data
	data := JobMetricData{
		Job:         "test-job",
		MetricName:  "test_metric",
		Labels:      []string{"label1", "label2"},
		Cardinality: "100",
	}

	err = writer.Write(data)
	if err != nil {
		t.Errorf("Write() error = %v", err)
	}

	// Flush to ensure data is written
	err = writer.Flush()
	if err != nil {
		t.Errorf("Flush() error = %v", err)
	}

	// Verify file was created
	filePath := filepath.Join(tmpDir, "test-job.txt")
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		t.Errorf("File was not created: %s", filePath)
	}

	// Verify stats
	stats := writer.GetStats()
	if stats.TotalWrites != 1 {
		t.Errorf("TotalWrites = %d, want 1", stats.TotalWrites)
	}
	if stats.TotalFiles != 1 {
		t.Errorf("TotalFiles = %d, want 1", stats.TotalFiles)
	}
}

func TestStreamingJobFileWriter_LRUEviction(t *testing.T) {
	tmpDir := t.TempDir()

	// Create writer with max 2 open files
	writer, err := NewStreamingJobFileWriter(tmpDir, 2)
	if err != nil {
		t.Fatalf("Failed to create writer: %v", err)
	}
	defer writer.Close()

	// Write to 3 different jobs (should trigger LRU eviction)
	jobs := []string{"job1", "job2", "job3"}
	for _, job := range jobs {
		data := JobMetricData{
			Job:         job,
			MetricName:  "test_metric",
			Labels:      []string{"label1"},
			Cardinality: "100",
		}
		err = writer.Write(data)
		if err != nil {
			t.Errorf("Write() error = %v", err)
		}
	}

	// Verify stats
	stats := writer.GetStats()
	if stats.ActiveFiles > 2 {
		t.Errorf("ActiveFiles = %d, want <= 2 (LRU eviction should have occurred)", stats.ActiveFiles)
	}
	if stats.TotalFiles != 3 {
		t.Errorf("TotalFiles = %d, want 3", stats.TotalFiles)
	}

	// Verify all files were created
	for _, job := range jobs {
		filePath := filepath.Join(tmpDir, job+".txt")
		if _, err := os.Stat(filePath); os.IsNotExist(err) {
			t.Errorf("File was not created: %s", filePath)
		}
	}
}

func TestStreamingJobFileWriter_FileContent(t *testing.T) {
	tmpDir := t.TempDir()

	writer, err := NewStreamingJobFileWriter(tmpDir, 10)
	if err != nil {
		t.Fatalf("Failed to create writer: %v", err)
	}
	defer writer.Close()

	// Write test data with label cardinality
	data := JobMetricData{
		Job:         "test-job",
		MetricName:  "test_metric",
		Labels:      []string{"label1", "label2"},
		Cardinality: "100",
		LabelCardinality: map[string]int64{
			"label1": 10,
			"label2": 20,
		},
	}

	err = writer.Write(data)
	if err != nil {
		t.Errorf("Write() error = %v", err)
	}

	err = writer.Close()
	if err != nil {
		t.Errorf("Close() error = %v", err)
	}

	// Read file content
	filePath := filepath.Join(tmpDir, "test-job.txt")
	content, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("Failed to read file: %v", err)
	}

	lines := strings.Split(string(content), "\n")
	if len(lines) < 2 {
		t.Fatalf("Expected at least 2 lines (header + data), got %d", len(lines))
	}

	// Check header
	expectedHeader := "JOB|METRIC_NAME|LABELS|CARDINALITY|LABEL_CARDINALITY"
	if lines[0] != expectedHeader {
		t.Errorf("Header = %s, want %s", lines[0], expectedHeader)
	}

	// Check data line contains expected fields
	dataLine := lines[1]
	if !strings.Contains(dataLine, "test-job") {
		t.Errorf("Data line missing job name: %s", dataLine)
	}
	if !strings.Contains(dataLine, "test_metric") {
		t.Errorf("Data line missing metric name: %s", dataLine)
	}
	if !strings.Contains(dataLine, "label1,label2") {
		t.Errorf("Data line missing labels: %s", dataLine)
	}
	if !strings.Contains(dataLine, "100") {
		t.Errorf("Data line missing cardinality: %s", dataLine)
	}
}

func TestStreamingJobFileWriter_MultipleWrites(t *testing.T) {
	tmpDir := t.TempDir()

	writer, err := NewStreamingJobFileWriter(tmpDir, 10)
	if err != nil {
		t.Fatalf("Failed to create writer: %v", err)
	}
	defer writer.Close()

	// Write multiple metrics to same job
	for i := 0; i < 5; i++ {
		data := JobMetricData{
			Job:         "test-job",
			MetricName:  "test_metric_" + string(rune('0'+i)),
			Labels:      []string{"label1"},
			Cardinality: "100",
		}
		err = writer.Write(data)
		if err != nil {
			t.Errorf("Write() error = %v", err)
		}
	}

	err = writer.Close()
	if err != nil {
		t.Errorf("Close() error = %v", err)
	}

	// Read file and verify all metrics are present
	filePath := filepath.Join(tmpDir, "test-job.txt")
	content, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("Failed to read file: %v", err)
	}

	lines := strings.Split(string(content), "\n")
	// Should have header + 5 data lines + empty line at end
	if len(lines) < 6 {
		t.Errorf("Expected at least 6 lines, got %d", len(lines))
	}

	// Verify stats
	stats := writer.GetStats()
	if stats.TotalWrites != 5 {
		t.Errorf("TotalWrites = %d, want 5", stats.TotalWrites)
	}
}

func TestStreamingJobFileWriter_ConcurrentWrites(t *testing.T) {
	tmpDir := t.TempDir()

	writer, err := NewStreamingJobFileWriter(tmpDir, 10)
	if err != nil {
		t.Fatalf("Failed to create writer: %v", err)
	}
	defer writer.Close()

	// Write concurrently from multiple goroutines
	done := make(chan bool)
	for i := 0; i < 10; i++ {
		go func(id int) {
			data := JobMetricData{
				Job:         "concurrent-job",
				MetricName:  "metric_" + string(rune('0'+id)),
				Labels:      []string{"label1"},
				Cardinality: "100",
			}
			writer.Write(data)
			done <- true
		}(i)
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}

	err = writer.Close()
	if err != nil {
		t.Errorf("Close() error = %v", err)
	}

	// Verify file was created and has correct number of lines
	filePath := filepath.Join(tmpDir, "concurrent-job.txt")
	content, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("Failed to read file: %v", err)
	}

	lines := strings.Split(string(content), "\n")
	// Should have header + 10 data lines
	if len(lines) < 11 {
		t.Errorf("Expected at least 11 lines, got %d", len(lines))
	}
}
