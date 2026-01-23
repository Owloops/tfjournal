package storage

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"

	"github.com/Owloops/tfjournal/run"
)

const _s3Timeout = 30 * time.Second

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

	var opts []func(*config.LoadOptions) error

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
	ctx, cancel := context.WithTimeout(context.Background(), _s3Timeout*3)
	defer cancel()

	prefix := s.prefix + _runsDir + "/"

	paginator := s3.NewListObjectsV2Paginator(s.client, &s3.ListObjectsV2Input{
		Bucket: aws.String(s.bucket),
		Prefix: aws.String(prefix),
	})

	var runs []*run.Run

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

			id := strings.TrimSuffix(strings.TrimPrefix(key, prefix), ".json")
			r, err := s.GetRun(id)
			if err != nil {
				continue
			}

			if !matchesFilter(r, opts) {
				continue
			}

			runs = append(runs, r)
		}
	}

	sort.Slice(runs, func(i, j int) bool {
		return runs[i].Timestamp.After(runs[j].Timestamp)
	})

	if opts.Limit > 0 && len(runs) > opts.Limit {
		runs = runs[:opts.Limit]
	}

	return runs, nil
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

func (s *S3Store) runKey(id string) string {
	return s.prefix + _runsDir + "/" + id + ".json"
}

func (s *S3Store) outputKey(id string) string {
	return s.prefix + _outputsDir + "/" + id + ".txt"
}
