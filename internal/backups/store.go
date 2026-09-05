package backups

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path"
	"sort"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

type Store interface {
	PutFile(context.Context, string, string, string) error
	PutBytes(context.Context, string, []byte, string) error
	GetFile(context.Context, string, string) error
	GetBytes(context.Context, string) ([]byte, error)
	List(context.Context, string) ([]string, error)
}

type S3Store struct {
	client *s3.Client
	bucket string
	prefix string
}

func NewS3Store(ctx context.Context, config Config) (*S3Store, error) {
	awsConfig, err := awsconfig.LoadDefaultConfig(ctx, awsconfig.WithRegion(config.Region))
	if err != nil {
		return nil, fmt.Errorf("load S3 configuration: %w", err)
	}
	client := s3.NewFromConfig(awsConfig, func(options *s3.Options) {
		options.UsePathStyle = config.ForcePathStyle
		if config.Endpoint != "" {
			options.BaseEndpoint = aws.String(config.Endpoint)
		}
	})
	return &S3Store{client: client, bucket: config.Bucket, prefix: strings.Trim(config.Prefix, "/")}, nil
}

func (s *S3Store) key(relative string) string { return path.Join(s.prefix, relative) }

func (s *S3Store) PutFile(ctx context.Context, key, filename, contentType string) error {
	file, err := os.Open(filename)
	if err != nil {
		return err
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return err
	}
	_, err = s.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket: aws.String(s.bucket), Key: aws.String(s.key(key)), Body: file,
		ContentLength: aws.Int64(info.Size()), ContentType: aws.String(contentType),
	})
	return err
}

func (s *S3Store) PutBytes(ctx context.Context, key string, value []byte, contentType string) error {
	_, err := s.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket: aws.String(s.bucket), Key: aws.String(s.key(key)), Body: strings.NewReader(string(value)),
		ContentLength: aws.Int64(int64(len(value))), ContentType: aws.String(contentType),
	})
	return err
}

func (s *S3Store) GetFile(ctx context.Context, key, filename string) error {
	output, err := s.client.GetObject(ctx, &s3.GetObjectInput{Bucket: aws.String(s.bucket), Key: aws.String(s.key(key))})
	if err != nil {
		return err
	}
	defer output.Body.Close()
	file, err := os.OpenFile(filename, os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(file, output.Body)
	closeErr := file.Close()
	if copyErr != nil {
		return copyErr
	}
	return closeErr
}

func (s *S3Store) GetBytes(ctx context.Context, key string) ([]byte, error) {
	output, err := s.client.GetObject(ctx, &s3.GetObjectInput{Bucket: aws.String(s.bucket), Key: aws.String(s.key(key))})
	if err != nil {
		return nil, err
	}
	defer output.Body.Close()
	return io.ReadAll(io.LimitReader(output.Body, 1024*1024))
}

func (s *S3Store) List(ctx context.Context, prefix string) ([]string, error) {
	paginator := s3.NewListObjectsV2Paginator(s.client, &s3.ListObjectsV2Input{
		Bucket: aws.String(s.bucket), Prefix: aws.String(s.key(prefix)),
	})
	var keys []string
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, err
		}
		for _, object := range page.Contents {
			key := strings.TrimPrefix(aws.ToString(object.Key), s.prefix+"/")
			keys = append(keys, key)
		}
	}
	sort.Strings(keys)
	return keys, nil
}

func putManifest(ctx context.Context, store Store, key string, manifest Manifest) error {
	encoded, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return err
	}
	encoded = append(encoded, '\n')
	return store.PutBytes(ctx, key, encoded, "application/json")
}
