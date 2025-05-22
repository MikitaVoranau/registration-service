package MinIO

import (
	"context"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"io"
)

type Config struct {
	MinioEndpoint     string `env:"MINIO_ENDPOINT" envDefault:"localhost:9000"`
	BucketName        string `env:"MINIO_BUCKET_NAME" envDefault:"storage"`
	MinioRootUser     string `env:"MINIO_ROOT_USER" envDefault:"admin"`
	MinioRootPassword string `env:"MINIO_ROOT_PASSWORD" envDefault:"Study2005"`
	MinioUseSSL       bool   `env:"MINIO_USE_SSL" envDefault:"false"`

	MinioAccessKey string `env:"MINIO_ACCESS_KEY" envDefault:""`
	MinioSecretKey string `env:"MINIO_SECRET_KEY" envDefault:""`
}

type MinIOClient struct {
	Client *minio.Client
	Bucket string
}

func New(cfg Config) *MinIOClient {
	client, err := minio.New(cfg.MinioEndpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(cfg.MinioAccessKey, cfg.MinioSecretKey, ""),
		Secure: true,
	})
	if err != nil {
		return nil
	}

	ctx := context.Background()
	err = client.MakeBucket(ctx, cfg.BucketName, minio.MakeBucketOptions{})
	if err != nil {
		exists, errBucketExists := client.BucketExists(ctx, cfg.BucketName)
		if !(errBucketExists == nil && exists) {
			return nil
		}
	}

	return &MinIOClient{
		Client: client,
		Bucket: cfg.BucketName,
	}
}

func (m *MinIOClient) UploadFile(ctx context.Context, key string, reader io.Reader, size int64) error {
	_, err := m.Client.PutObject(ctx, m.Bucket, key, reader, size, minio.PutObjectOptions{})
	return err
}

func (m *MinIOClient) DownloadFile(ctx context.Context, key string) (io.Reader, error) {
	obj, err := m.Client.GetObject(ctx, m.Bucket, key, minio.GetObjectOptions{})
	if err != nil {
		return nil, err
	}
	return obj, nil
}

func (m *MinIOClient) DeleteFile(ctx context.Context, key string) error {
	return m.Client.RemoveObject(ctx, m.Bucket, key, minio.RemoveObjectOptions{})
}
