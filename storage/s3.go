package storage

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awshttp "github.com/aws/aws-sdk-go-v2/aws/transport/http"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"

	"github.com/Owloops/tfjournal/run"
)

const (
	_s3Timeout            = 30 * time.Second
	_defaultMaxConcurrent = 20
	_defaultMaxIdleConns  = 25
	_defaultSinceDays     = 30
)

type S3Store struct {
	client *s3.Client
	bucket string
	prefix string
}

func NewS3Store(bucket, region, prefix string) (*S3Store, error) {
	return NewS3StoreWithProfile(bucket, region, prefix, "")
}

func NewS3StoreWithProfile(bucket, region, prefix, profile string) (*S3Store, error) {
	ctx, cancel := context.WithTimeout(context.Background(), _s3Timeout)
	defer cancel()

	httpClient := awshttp.NewBuildableClient().WithTransportOptions(func(tr *http.Transport) {
		tr.MaxIdleConnsPerHost = getEnvInt("TFJOURNAL_S3_POOL_SIZE", _defaultMaxIdleConns)
		tr.MaxIdleConns = 100
	})

	var opts []func(*config.LoadOptions) error
	opts = append(opts, config.WithHTTPClient(httpClient))

	if region != "" {
		opts = append(opts, config.WithRegion(region))
	}
	if profile != "" {
		opts = append(opts, config.WithSharedConfigProfile(profile))
	}

	cfg, err := config.LoadDefaultConfig(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to load AWS config: %w", err)
	}

	return &S3Store{
		client: s3.NewFromConfig(cfg),
		bucket: bucket,
		prefix: prefix,
	}, nil
}

func getEnvInt(key string, defaultVal int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			return n
		}
	}
	return defaultVal
}

func (s *S3Store) Close() error {
	return nil
}

func (s *S3Store) SaveRun(r *run.Run) error {
	data, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal run: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), _s3Timeout)
	defer cancel()

	key := s.runKey(r.ID)
	_, err = s.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(s.bucket),
		Key:         aws.String(key),
		Body:        bytes.NewReader(data),
		ContentType: aws.String("application/json"),
	})
	if err != nil {
		return fmt.Errorf("failed to upload run to S3: %w", err)
	}

	return nil
}

func (s *S3Store) GetRun(id string) (*run.Run, error) {
	ctx, cancel := context.WithTimeout(context.Background(), _s3Timeout)
	defer cancel()

	key := s.runKey(id)

	resp, err := s.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get run from S3: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read run data: %w", err)
	}

	var r run.Run
	if err := json.Unmarshal(data, &r); err != nil {
		return nil, fmt.Errorf("failed to parse run: %w", err)
	}

	return &r, nil
}

func (s *S3Store) ListRuns(opts ListOptions) ([]*run.Run, error) {
	ctx, cancel := context.WithTimeout(context.Background(), _s3Timeout*5)
	defer cancel()

	endDate := truncateToDay(time.Now())
	startDate := opts.Since
	if startDate.IsZero() {
		startDate = endDate.AddDate(0, 0, -_defaultSinceDays)
	}
	startDate = truncateToDay(startDate)

	prefixes := s.generateDatePrefixes(startDate, endDate)

	var allRuns []*run.Run
	var mu sync.Mutex
	var stop atomic.Bool

	maxParallelDays := getEnvInt("TFJOURNAL_S3_PARALLEL_DAYS", 7)
	sem := make(chan struct{}, maxParallelDays)
	var wg sync.WaitGroup

	for _, prefix := range prefixes {
		if stop.Load() {
			break
		}

		wg.Add(1)
		sem <- struct{}{}

		go func(prefix string) {
			defer wg.Done()
			defer func() { <-sem }()

			if stop.Load() {
				return
			}

			dayRuns, err := s.listAndFetchPrefix(ctx, prefix, opts)
			if err != nil {
				return
			}

			mu.Lock()
			allRuns = append(allRuns, dayRuns...)
			if opts.Limit > 0 && len(allRuns) >= opts.Limit*2 {
				stop.Store(true)
			}
			mu.Unlock()
		}(prefix)
	}

	wg.Wait()

	sort.Slice(allRuns, func(i, j int) bool {
		return allRuns[i].Timestamp.After(allRuns[j].Timestamp)
	})

	if opts.Limit > 0 && len(allRuns) > opts.Limit {
		allRuns = allRuns[:opts.Limit]
	}

	return allRuns, nil
}

func (s *S3Store) generateDatePrefixes(since, until time.Time) []string {
	var prefixes []string
	for d := until; !d.Before(since); d = d.AddDate(0, 0, -1) {
		prefix := fmt.Sprintf("%s%s/%s/", s.prefix, _runsDir, d.Format("2006/01/02"))
		prefixes = append(prefixes, prefix)
	}
	return prefixes
}

func (s *S3Store) listAndFetchPrefix(ctx context.Context, prefix string, opts ListOptions) ([]*run.Run, error) {
	paginator := s3.NewListObjectsV2Paginator(s.client, &s3.ListObjectsV2Input{
		Bucket: aws.String(s.bucket),
		Prefix: aws.String(prefix),
	})

	var ids []string
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to list S3 objects: %w", err)
		}
		for _, obj := range page.Contents {
			key := aws.ToString(obj.Key)
			if strings.HasSuffix(key, ".json") {
				id := strings.TrimSuffix(filepath.Base(key), ".json")
				ids = append(ids, id)
			}
		}
	}

	maxConcurrent := getEnvInt("TFJOURNAL_S3_CONCURRENCY", _defaultMaxConcurrent)
	sem := make(chan struct{}, maxConcurrent)
	var mu sync.Mutex
	var runs []*run.Run
	var wg sync.WaitGroup

	for _, id := range ids {
		wg.Add(1)
		sem <- struct{}{}
		go func(id string) {
			defer wg.Done()
			defer func() { <-sem }()

			r, err := s.GetRun(id)
			if err != nil {
				return
			}

			if !matchesFilter(r, opts) {
				return
			}

			mu.Lock()
			runs = append(runs, r)
			mu.Unlock()
		}(id)
	}

	wg.Wait()
	return runs, nil
}

func truncateToDay(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())
}

func (s *S3Store) SaveOutput(runID string, output []byte) error {
	ctx, cancel := context.WithTimeout(context.Background(), _s3Timeout)
	defer cancel()

	key := s.outputKey(runID)

	_, err := s.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(s.bucket),
		Key:         aws.String(key),
		Body:        bytes.NewReader(output),
		ContentType: aws.String("text/plain"),
	})
	if err != nil {
		return fmt.Errorf("failed to upload output to S3: %w", err)
	}

	return nil
}

func (s *S3Store) GetOutput(runID string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), _s3Timeout)
	defer cancel()

	key := s.outputKey(runID)

	resp, err := s.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get output from S3: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	return io.ReadAll(resp.Body)
}

func (s *S3Store) OutputPath(runID string) string {
	return fmt.Sprintf("s3://%s/%s", s.bucket, s.outputKey(runID))
}

func (s *S3Store) DeleteRun(id string) error {
	ctx, cancel := context.WithTimeout(context.Background(), _s3Timeout)
	defer cancel()

	_, err := s.client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(s.runKey(id)),
	})
	if err != nil {
		return err
	}

	_, err = s.client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(s.outputKey(id)),
	})
	return err
}

func (s *S3Store) ListRunIDs() (map[string]bool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), _s3Timeout*3)
	defer cancel()

	prefix := s.prefix + _runsDir + "/"

	paginator := s3.NewListObjectsV2Paginator(s.client, &s3.ListObjectsV2Input{
		Bucket: aws.String(s.bucket),
		Prefix: aws.String(prefix),
	})

	ids := make(map[string]bool)

	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to list S3 objects: %w", err)
		}

		for _, obj := range page.Contents {
			key := aws.ToString(obj.Key)
			if !strings.HasSuffix(key, ".json") {
				continue
			}
			id := strings.TrimSuffix(filepath.Base(key), ".json")
			ids[id] = true
		}
	}

	return ids, nil
}

func (s *S3Store) runKey(id string) string {
	date, err := run.ParseDateFromID(id)
	if err != nil {
		return s.prefix + _runsDir + "/" + id + ".json"
	}
	return fmt.Sprintf("%s%s/%s/%s.json", s.prefix, _runsDir, date.Format("2006/01/02"), id)
}

func (s *S3Store) outputKey(id string) string {
	date, err := run.ParseDateFromID(id)
	if err != nil {
		return s.prefix + _outputsDir + "/" + id + ".txt"
	}
	return fmt.Sprintf("%s%s/%s/%s.txt", s.prefix, _outputsDir, date.Format("2006/01/02"), id)
}
