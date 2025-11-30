package storage

import (
	"context"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"

	appconfig "zipperfly/internal/config"
	"zipperfly/internal/circuitbreaker"
	"zipperfly/internal/metrics"
)

// S3Provider implements Provider for S3-compatible storage
type S3Provider struct {
	client         *s3.Client
	circuitBreaker *circuitbreaker.Breaker
	metrics        *metrics.Metrics
	fetchTimeout   time.Duration
	maxRetries     int
	retryDelay     time.Duration
}

// NewS3Provider creates a new S3-compatible storage provider
func NewS3Provider(ctx context.Context, cfg *appconfig.Config, m *metrics.Metrics, cb *circuitbreaker.Breaker) (*S3Provider, error) {
	region := cfg.S3Region
	if region == "" {
		// Reasonable default; works for MinIO and AWS if caller doesn't care.
		region = "us-east-1"
	}

	cfgOpts := []func(*config.LoadOptions) error{
		config.WithRegion(region),
	}

	// Static credentials (typical for MinIO and many S3-compatible providers)
	if cfg.S3AccessKeyID != "" && cfg.S3SecretAccessKey != "" {
		cfgOpts = append(cfgOpts, config.WithCredentialsProvider(
			credentials.NewStaticCredentialsProvider(
				cfg.S3AccessKeyID,
				cfg.S3SecretAccessKey,
				"",
			),
		))
	}

	// Custom endpoint (MinIO, Wasabi, etc.)
	if cfg.S3Endpoint != "" {
		endpoint := cfg.S3Endpoint
		cfgOpts = append(cfgOpts, config.WithEndpointResolverWithOptions(
			aws.EndpointResolverWithOptionsFunc(
				func(service, region string, _ ...interface{}) (aws.Endpoint, error) {
					if service == s3.ServiceID {
						return aws.Endpoint{
							URL:               endpoint,
							HostnameImmutable: true, // don't rewrite host when using a custom endpoint
						}, nil
					}
					return aws.Endpoint{}, &aws.EndpointNotFoundError{}
				},
			),
		))
	}

	awsCfg, err := config.LoadDefaultConfig(ctx, cfgOpts...)
	if err != nil {
		return nil, err
	}

	usePathStyle := cfg.S3UsePathStyle // <— new config flag; see notes below

	client := s3.NewFromConfig(awsCfg, func(o *s3.Options) {
		o.UsePathStyle = usePathStyle
	})

	return &S3Provider{
		client:         client,
		circuitBreaker: cb,
		metrics:        m,
		fetchTimeout:   cfg.StorageFetchTimeout,
		maxRetries:     cfg.StorageMaxRetries,
		retryDelay:     cfg.StorageRetryDelay,
	}, nil
}

// GetObject retrieves an object from S3
func (s *S3Provider) GetObject(ctx context.Context, bucket, key string) (io.ReadCloser, error) {
	start := time.Now()
	var resultLabel string
	defer func() {
		duration := time.Since(start)
		s.metrics.StorageFetchDuration.WithLabelValues("s3", resultLabel).Observe(duration.Seconds())
	}()

	// Track active file fetches
	s.metrics.ActiveFileFetches.Inc()
	defer s.metrics.ActiveFileFetches.Dec()

	// Execute with circuit breaker
	result, err := s.circuitBreaker.Execute(func() (interface{}, error) {
		// Retry loop with exponential backoff
		var lastErr error
		for attempt := 0; attempt <= s.maxRetries; attempt++ {
			if attempt > 0 {
				// Exponential backoff: retryDelay * 2^(attempt-1)
				delay := s.retryDelay * time.Duration(1<<(attempt-1))
				time.Sleep(delay)
			}

			// Apply timeout to this attempt
			fetchCtx, cancel := context.WithTimeout(ctx, s.fetchTimeout)
			defer cancel()

			output, err := s.client.GetObject(fetchCtx, &s3.GetObjectInput{
				Bucket: aws.String(bucket),
				Key:    aws.String(key),
			})

			if err == nil {
				resultLabel = "success"
				return output.Body, nil
			}

			lastErr = err

			// Check if error is retryable
			if !isRetryableError(err) || attempt == s.maxRetries {
				break
			}
		}

		resultLabel = "error"
		return nil, lastErr
	})

	if err != nil {
		return nil, err
	}

	return result.(io.ReadCloser), nil
}

// isRetryableError determines if an error should trigger a retry
func isRetryableError(err error) bool {
	if err == nil {
		return false
	}

	// Check for context errors (timeout/cancellation)
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
		return false
	}

	// Most S3 errors are retryable (network issues, throttling, etc.)
	// Non-retryable errors like NoSuchKey, AccessDenied will fail fast
	// This is a simplified check - could be enhanced with AWS error type checking
	return true
}

// HealthCheck performs a lightweight connectivity check to S3
func (s *S3Provider) HealthCheck(ctx context.Context) error {
	// Use ListBuckets as a lightweight operation to verify S3 connectivity
	// This doesn't require knowing a specific bucket name
	checkCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	_, err := s.client.ListBuckets(checkCtx, &s3.ListBucketsInput{})
	if err != nil {
		return fmt.Errorf("s3 connectivity check failed: %w", err)
	}
	return nil
}
