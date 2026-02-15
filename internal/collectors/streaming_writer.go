package collectors

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// MetricDataWriter interface for writing metric data
type MetricDataWriter interface {
	Write(data JobMetricData) error
	Flush() error
	Close() error
	GetStats() WriterStats
}

// WriterStats tracks write statistics
type WriterStats struct {
	TotalWrites  int64
	TotalFlushes int64
	ActiveFiles  int
	TotalFiles   int
	BytesWritten int64
}

// StreamingJobFileWriter writes metric data to disk with file handle pooling
type StreamingJobFileWriter struct {
	outputDir     string
	activeFiles   map[string]*bufio.Writer
	fileHandles   map[string]*os.File
	maxOpenFiles  int
	lruQueue      []string
	mu            sync.Mutex
	stats         WriterStats
	flushInterval int
}

// NewStreamingJobFileWriter creates a new streaming writer
func NewStreamingJobFileWriter(outputDir string, maxOpenFiles int) (*StreamingJobFileWriter, error) {
	if err := os.MkdirAll(outputDir, 0700); err != nil {
		return nil, fmt.Errorf("failed to create output directory: %w", err)
	}

	return &StreamingJobFileWriter{
		outputDir:     outputDir,
		activeFiles:   make(map[string]*bufio.Writer),
		fileHandles:   make(map[string]*os.File),
		maxOpenFiles:  maxOpenFiles,
		lruQueue:      make([]string, 0),
		flushInterval: 100, // Flush every 100 writes
	}, nil
}

// Write writes a single JobMetricData entry to the appropriate file
func (w *StreamingJobFileWriter) Write(data JobMetricData) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	writer, err := w.getOrCreateWriter(data.Job)
	if err != nil {
		return err
	}

	// Format line
	labelsStr := strings.Join(data.Labels, ",")
	var labelCardinalityStr string
	if len(data.LabelCardinality) > 0 {
		var parts []string
		for _, label := range data.Labels {
			if count, ok := data.LabelCardinality[label]; ok {
				parts = append(parts, fmt.Sprintf("%s:%d", label, count))
			}
		}
		labelCardinalityStr = strings.Join(parts, ",")
	}

	line := fmt.Sprintf("%s|%s|%s|%s|%s\n",
		data.Job, data.MetricName, labelsStr, data.Cardinality, labelCardinalityStr)

	n, err := writer.WriteString(line)
	if err != nil {
		return err
	}

	w.stats.TotalWrites++
	w.stats.BytesWritten += int64(n)

	// Periodic flush
	if w.stats.TotalWrites%int64(w.flushInterval) == 0 {
		writer.Flush()
		w.stats.TotalFlushes++
	}

	return nil
}

// getOrCreateWriter gets or creates a writer for a job (with LRU eviction)
func (w *StreamingJobFileWriter) getOrCreateWriter(job string) (*bufio.Writer, error) {
	// Check if already open
	if writer, exists := w.activeFiles[job]; exists {
		w.updateLRU(job)
		return writer, nil
	}

	// At limit? Close least recently used
	if len(w.activeFiles) >= w.maxOpenFiles {
		if err := w.closeLRU(); err != nil {
			return nil, err
		}
	}

	// Open new file
	return w.openFile(job)
}

// openFile opens a new file for a job
func (w *StreamingJobFileWriter) openFile(job string) (*bufio.Writer, error) {
	safeJobName := sanitizeJobName(job)
	filePath := filepath.Join(w.outputDir, fmt.Sprintf("%s.txt", safeJobName))

	// Check if file exists (append mode) or create new
	fileExists := false
	if _, err := os.Stat(filePath); err == nil {
		fileExists = true
	}

	file, err := os.OpenFile(filePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
	if err != nil {
		return nil, fmt.Errorf("failed to open file for job %s: %w", job, err)
	}

	writer := bufio.NewWriter(file)

	// Write header if new file
	if !fileExists {
		if _, err := writer.WriteString("JOB|METRIC_NAME|LABELS|CARDINALITY|LABEL_CARDINALITY\n"); err != nil {
			file.Close()
			return nil, err
		}
		w.stats.TotalFiles++
	}

	w.activeFiles[job] = writer
	w.fileHandles[job] = file
	w.lruQueue = append(w.lruQueue, job)
	w.stats.ActiveFiles = len(w.activeFiles)

	return writer, nil
}

// closeLRU closes the least recently used file
func (w *StreamingJobFileWriter) closeLRU() error {
	if len(w.lruQueue) == 0 {
		return nil
	}

	// Get least recently used job
	lruJob := w.lruQueue[0]
	w.lruQueue = w.lruQueue[1:]

	// Flush and close
	if writer, exists := w.activeFiles[lruJob]; exists {
		writer.Flush()
		delete(w.activeFiles, lruJob)
	}

	if file, exists := w.fileHandles[lruJob]; exists {
		file.Close()
		delete(w.fileHandles, lruJob)
	}

	w.stats.ActiveFiles = len(w.activeFiles)

	return nil
}

// updateLRU moves a job to the end of the LRU queue
func (w *StreamingJobFileWriter) updateLRU(job string) {
	// Remove from current position
	for i, j := range w.lruQueue {
		if j == job {
			w.lruQueue = append(w.lruQueue[:i], w.lruQueue[i+1:]...)
			break
		}
	}
	// Add to end (most recently used)
	w.lruQueue = append(w.lruQueue, job)
}

// Flush flushes all active writers
func (w *StreamingJobFileWriter) Flush() error {
	w.mu.Lock()
	defer w.mu.Unlock()

	for _, writer := range w.activeFiles {
		if err := writer.Flush(); err != nil {
			return err
		}
	}
	w.stats.TotalFlushes++

	return nil
}

// Close closes all open files
func (w *StreamingJobFileWriter) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()

	// Flush all writers
	for _, writer := range w.activeFiles {
		writer.Flush()
	}

	// Close all files
	for _, file := range w.fileHandles {
		file.Close()
	}

	w.activeFiles = make(map[string]*bufio.Writer)
	w.fileHandles = make(map[string]*os.File)
	w.lruQueue = make([]string, 0)
	w.stats.ActiveFiles = 0

	return nil
}

// GetStats returns current writer statistics
func (w *StreamingJobFileWriter) GetStats() WriterStats {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.stats
}
