package storage

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestNewS3Client(t *testing.T) {
	tests := []struct {
		name        string
		bucket      string
		prefix      string
		region      string
		expectError bool
	}{
		{
			name:        "valid configuration",
			bucket:      "test-bucket",
			prefix:      "test-prefix",
			region:      "eu-west-1",
			expectError: false,
		},
		{
			name:        "empty bucket",
			bucket:      "",
			prefix:      "test-prefix",
			region:      "eu-west-1",
			expectError: true,
		},
		{
			name:        "empty prefix is valid",
			bucket:      "test-bucket",
			prefix:      "",
			region:      "eu-west-1",
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, err := NewS3Client(tt.bucket, tt.prefix, tt.region)
			if tt.expectError {
				if err == nil {
					t.Errorf("expected error but got none")
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				if client == nil {
					t.Errorf("expected client but got nil")
				}
				if client != nil {
					if client.GetBucket() != tt.bucket {
						t.Errorf("bucket = %v, want %v", client.GetBucket(), tt.bucket)
					}
					if client.GetPrefix() != tt.prefix {
						t.Errorf("prefix = %v, want %v", client.GetPrefix(), tt.prefix)
					}
				}
			}
		})
	}
}

func TestNewS3ClientFromEnv(t *testing.T) {
	// Save original env vars
	origBucket := os.Getenv("S3_BUCKET")
	origPrefix := os.Getenv("S3_PREFIX")
	origRegion := os.Getenv("AWS_REGION")
	defer func() {
		os.Setenv("S3_BUCKET", origBucket)
		os.Setenv("S3_PREFIX", origPrefix)
		os.Setenv("AWS_REGION", origRegion)
	}()

	tests := []struct {
		name        string
		bucket      string
		prefix      string
		region      string
		expectError bool
	}{
		{
			name:        "valid env vars",
			bucket:      "env-bucket",
			prefix:      "env-prefix",
			region:      "us-west-2",
			expectError: false,
		},
		{
			name:        "missing bucket",
			bucket:      "",
			prefix:      "env-prefix",
			region:      "us-west-2",
			expectError: true,
		},
		{
			name:        "default region",
			bucket:      "env-bucket",
			prefix:      "env-prefix",
			region:      "",
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			os.Setenv("S3_BUCKET", tt.bucket)
			os.Setenv("S3_PREFIX", tt.prefix)
			os.Setenv("AWS_REGION", tt.region)

			client, err := NewS3ClientFromEnv()
			if tt.expectError {
				if err == nil {
					t.Errorf("expected error but got none")
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				if client != nil && tt.bucket != "" {
					if client.GetBucket() != tt.bucket {
						t.Errorf("bucket = %v, want %v", client.GetBucket(), tt.bucket)
					}
				}
			}
		})
	}
}

func TestBuildKey(t *testing.T) {
	tests := []struct {
		name   string
		prefix string
		key    string
		want   string
	}{
		{
			name:   "with prefix",
			prefix: "reports",
			key:    "job_metrics/data.txt",
			want:   "reports/job_metrics/data.txt",
		},
		{
			name:   "empty prefix",
			prefix: "",
			key:    "job_metrics/data.txt",
			want:   "job_metrics/data.txt",
		},
		{
			name:   "key with leading slash",
			prefix: "reports",
			key:    "/job_metrics/data.txt",
			want:   "reports/job_metrics/data.txt",
		},
		{
			name:   "nested prefix",
			prefix: "prod/reports",
			key:    "data.txt",
			want:   "prod/reports/data.txt",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := &S3Client{
				bucket: "test-bucket",
				prefix: tt.prefix,
			}
			got := client.buildKey(tt.key)
			if got != tt.want {
				t.Errorf("buildKey() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGetS3URI(t *testing.T) {
	client := &S3Client{
		bucket: "my-bucket",
		prefix: "reports",
	}

	tests := []struct {
		name string
		key  string
		want string
	}{
		{
			name: "simple key",
			key:  "data.txt",
			want: "s3://my-bucket/reports/data.txt",
		},
		{
			name: "nested key",
			key:  "job_metrics/api-service.txt",
			want: "s3://my-bucket/reports/job_metrics/api-service.txt",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := client.GetS3URI(tt.key)
			if got != tt.want {
				t.Errorf("GetS3URI() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestFileExists(t *testing.T) {
	// This test would require mocking AWS S3 API
	// For now, we'll test the basic structure
	t.Skip("Requires AWS S3 mock server")
}

func TestCopyFile(t *testing.T) {
	// Create temp directory
	tmpDir, err := os.MkdirTemp("", "s3-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create source file
	srcFile := filepath.Join(tmpDir, "source.txt")
	content := []byte("test content")
	if err := os.WriteFile(srcFile, content, 0644); err != nil {
		t.Fatalf("failed to write source file: %v", err)
	}

	// Test copy
	dstFile := filepath.Join(tmpDir, "subdir", "dest.txt")
	if err := CopyFile(srcFile, dstFile); err != nil {
		t.Errorf("CopyFile() error = %v", err)
	}

	// Verify destination file
	gotContent, err := os.ReadFile(dstFile)
	if err != nil {
		t.Errorf("failed to read destination file: %v", err)
	}
	if string(gotContent) != string(content) {
		t.Errorf("content = %v, want %v", string(gotContent), string(content))
	}
}

func TestCopyFile_NonExistentSource(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "s3-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	srcFile := filepath.Join(tmpDir, "nonexistent.txt")
	dstFile := filepath.Join(tmpDir, "dest.txt")

	err = CopyFile(srcFile, dstFile)
	if err == nil {
		t.Errorf("expected error for non-existent source file")
	}
}

// Mock S3 server for integration-style tests
func setupMockS3Server(t *testing.T) *httptest.Server {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == "PUT":
			// Upload
			body, _ := io.ReadAll(r.Body)
			t.Logf("Mock S3: PUT %s (%d bytes)", r.URL.Path, len(body))
			w.WriteHeader(http.StatusOK)
		case r.Method == "GET":
			// Download
			t.Logf("Mock S3: GET %s", r.URL.Path)
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("mock file content"))
		case r.Method == "HEAD":
			// Check existence
			t.Logf("Mock S3: HEAD %s", r.URL.Path)
			w.WriteHeader(http.StatusOK)
		default:
			t.Logf("Mock S3: %s %s (not implemented)", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusNotImplemented)
		}
	})
	return httptest.NewServer(handler)
}

func TestS3ClientIntegration(t *testing.T) {
	// This would require actual AWS credentials or localstack
	// Skip for unit tests
	t.Skip("Integration test - requires AWS credentials or localstack")
}

func TestContainsHelper(t *testing.T) {
	tests := []struct {
		name  string
		slice []string
		item  string
		want  bool
	}{
		{
			name:  "item exists",
			slice: []string{"html", "json", "text"},
			item:  "json",
			want:  true,
		},
		{
			name:  "item does not exist",
			slice: []string{"html", "json"},
			item:  "xml",
			want:  false,
		},
		{
			name:  "empty slice",
			slice: []string{},
			item:  "json",
			want:  false,
		},
		{
			name:  "case insensitive match",
			slice: []string{"HTML", "JSON"},
			item:  "json",
			want:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := contains(tt.slice, tt.item)
			if got != tt.want {
				t.Errorf("contains() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestS3ClientGetters(t *testing.T) {
	client := &S3Client{
		bucket: "test-bucket",
		prefix: "test-prefix",
	}

	if got := client.GetBucket(); got != "test-bucket" {
		t.Errorf("GetBucket() = %v, want test-bucket", got)
	}

	if got := client.GetPrefix(); got != "test-prefix" {
		t.Errorf("GetPrefix() = %v, want test-prefix", got)
	}
}

func TestUploadContent(t *testing.T) {
	// This would require mocking S3 API
	t.Skip("Requires AWS S3 mock server")
}

func TestDownloadContent(t *testing.T) {
	// This would require mocking S3 API
	t.Skip("Requires AWS S3 mock server")
}

func TestListFiles(t *testing.T) {
	// This would require mocking S3 API
	t.Skip("Requires AWS S3 mock server")
}
