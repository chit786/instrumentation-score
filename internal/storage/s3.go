package storage

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
)

type S3Client struct {
	bucket   string
	prefix   string
	uploader *s3manager.Uploader
	s3Svc    *s3.S3
}

func NewS3Client(bucket, prefix, region string) (*S3Client, error) {
	if bucket == "" {
		return nil, fmt.Errorf("S3 bucket name is required")
	}

	sess, err := session.NewSession(&aws.Config{
		Region: aws.String(region),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create AWS session: %w", err)
	}

	return &S3Client{
		bucket:   bucket,
		prefix:   prefix,
		uploader: s3manager.NewUploader(sess),
		s3Svc:    s3.New(sess),
	}, nil
}

func NewS3ClientFromEnv() (*S3Client, error) {
	bucket := os.Getenv("S3_BUCKET")
	prefix := os.Getenv("S3_PREFIX")
	region := os.Getenv("AWS_REGION")

	if region == "" {
		region = "eu-west-1"
	}

	return NewS3Client(bucket, prefix, region)
}

func (c *S3Client) UploadFile(localPath, s3Key string) error {
	file, err := os.Open(localPath)
	if err != nil {
		return fmt.Errorf("failed to open file %s: %w", localPath, err)
	}
	defer file.Close()

	key := c.buildKey(s3Key)
	_, err = c.uploader.Upload(&s3manager.UploadInput{
		Bucket: aws.String(c.bucket),
		Key:    aws.String(key),
		Body:   file,
	})
	if err != nil {
		return fmt.Errorf("failed to upload file to s3://%s/%s: %w", c.bucket, key, err)
	}

	return nil
}

func (c *S3Client) UploadDirectory(localDir, s3Prefix string) ([]string, error) {
	var uploadedFiles []string

	err := filepath.Walk(localDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() {
			return nil
		}

		relPath, err := filepath.Rel(localDir, path)
		if err != nil {
			return fmt.Errorf("failed to get relative path: %w", err)
		}

		s3Key := filepath.Join(s3Prefix, relPath)
		s3Key = strings.ReplaceAll(s3Key, "\\", "/")

		if err := c.UploadFile(path, s3Key); err != nil {
			return err
		}

		uploadedFiles = append(uploadedFiles, s3Key)
		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to upload directory: %w", err)
	}

	return uploadedFiles, nil
}

func (c *S3Client) DownloadFile(s3Key, localPath string) error {
	key := c.buildKey(s3Key)

	if err := os.MkdirAll(filepath.Dir(localPath), 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	file, err := os.Create(localPath)
	if err != nil {
		return fmt.Errorf("failed to create file %s: %w", localPath, err)
	}
	defer file.Close()

	downloader := s3manager.NewDownloaderWithClient(c.s3Svc)
	_, err = downloader.Download(file, &s3.GetObjectInput{
		Bucket: aws.String(c.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return fmt.Errorf("failed to download file from s3://%s/%s: %w", c.bucket, key, err)
	}

	return nil
}

func (c *S3Client) DownloadDirectory(s3Prefix, localDir string) ([]string, error) {
	var downloadedFiles []string

	prefix := c.buildKey(s3Prefix)
	err := c.s3Svc.ListObjectsV2Pages(&s3.ListObjectsV2Input{
		Bucket: aws.String(c.bucket),
		Prefix: aws.String(prefix),
	}, func(page *s3.ListObjectsV2Output, lastPage bool) bool {
		for _, obj := range page.Contents {
			s3Key := aws.StringValue(obj.Key)

			relPath := strings.TrimPrefix(s3Key, prefix)
			relPath = strings.TrimPrefix(relPath, "/")
			if relPath == "" {
				continue
			}

			localPath := filepath.Join(localDir, relPath)

			if err := c.DownloadFile(strings.TrimPrefix(s3Key, c.prefix+"/"), localPath); err != nil {
				fmt.Fprintf(os.Stderr, "Warning: failed to download %s: %v\n", s3Key, err)
				continue
			}

			downloadedFiles = append(downloadedFiles, localPath)
		}
		return true
	})

	if err != nil {
		return nil, fmt.Errorf("failed to list objects in s3://%s/%s: %w", c.bucket, prefix, err)
	}

	if len(downloadedFiles) == 0 {
		return nil, fmt.Errorf("no files found in s3://%s/%s", c.bucket, prefix)
	}

	return downloadedFiles, nil
}

func (c *S3Client) ListFiles(s3Prefix string) ([]string, error) {
	var files []string

	prefix := c.buildKey(s3Prefix)
	err := c.s3Svc.ListObjectsV2Pages(&s3.ListObjectsV2Input{
		Bucket: aws.String(c.bucket),
		Prefix: aws.String(prefix),
	}, func(page *s3.ListObjectsV2Output, lastPage bool) bool {
		for _, obj := range page.Contents {
			files = append(files, aws.StringValue(obj.Key))
		}
		return true
	})

	if err != nil {
		return nil, fmt.Errorf("failed to list files: %w", err)
	}

	return files, nil
}

func (c *S3Client) FileExists(s3Key string) (bool, error) {
	key := c.buildKey(s3Key)
	_, err := c.s3Svc.HeadObject(&s3.HeadObjectInput{
		Bucket: aws.String(c.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		if strings.Contains(err.Error(), "NotFound") {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func (c *S3Client) UploadContent(content []byte, s3Key string) error {
	key := c.buildKey(s3Key)
	_, err := c.uploader.Upload(&s3manager.UploadInput{
		Bucket: aws.String(c.bucket),
		Key:    aws.String(key),
		Body:   bytes.NewReader(content),
	})
	if err != nil {
		return fmt.Errorf("failed to upload content to s3://%s/%s: %w", c.bucket, key, err)
	}
	return nil
}

func (c *S3Client) DownloadContent(s3Key string) ([]byte, error) {
	key := c.buildKey(s3Key)

	buff := &aws.WriteAtBuffer{}
	downloader := s3manager.NewDownloaderWithClient(c.s3Svc)
	_, err := downloader.Download(buff, &s3.GetObjectInput{
		Bucket: aws.String(c.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to download content from s3://%s/%s: %w", c.bucket, key, err)
	}

	return buff.Bytes(), nil
}

func (c *S3Client) GetBucket() string {
	return c.bucket
}

func (c *S3Client) GetPrefix() string {
	return c.prefix
}

func (c *S3Client) buildKey(key string) string {
	if c.prefix == "" {
		return key
	}
	key = strings.TrimPrefix(key, "/")
	return filepath.Join(c.prefix, key)
}

func (c *S3Client) GetS3URI(key string) string {
	fullKey := c.buildKey(key)
	return fmt.Sprintf("s3://%s/%s", c.bucket, fullKey)
}

func CopyFile(src, dst string) error {
	sourceFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer sourceFile.Close()

	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return err
	}

	destFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer destFile.Close()

	_, err = io.Copy(destFile, sourceFile)
	return err
}
