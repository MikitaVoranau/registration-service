package MinIO

import (
	"context"
	"fmt"
	"io"
	"mime"
	"path/filepath"
	"strings"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

type Config struct {
	MinioEndpoint     string `env:"MINIO_ENDPOINT" envDefault:"minio:9000"`
	BucketName        string `env:"MINIO_BUCKET_NAME" envDefault:"storage"`
	MinioRootUser     string `env:"MINIO_ROOT_USER" envDefault:"admin"`
	MinioRootPassword string `env:"MINIO_ROOT_PASSWORD" envDefault:"Study2005@"`
	MinioUseSSL       bool   `env:"MINIO_USE_SSL" envDefault:"false"`
	MinioAccessKey    string `env:"MINIO_ACCESS_KEY"`
	MinioSecretKey    string `env:"MINIO_SECRET_KEY"`
}

type MinIOClient struct {
	Client *minio.Client
	Bucket string
}

func New(cfg Config) (*MinIOClient, error) {
	// Проверяем обязательные параметры
	if cfg.MinioEndpoint == "" {
		return nil, fmt.Errorf("MINIO_ENDPOINT is required")
	}
	if cfg.BucketName == "" {
		return nil, fmt.Errorf("MINIO_BUCKET_NAME is required")
	}

	// Используем access credentials, если они предоставлены, иначе используем root credentials
	accessKey := cfg.MinioAccessKey
	secretKey := cfg.MinioSecretKey
	if accessKey == "" || secretKey == "" {
		accessKey = cfg.MinioRootUser
		secretKey = cfg.MinioRootPassword
	}

	if accessKey == "" || secretKey == "" {
		return nil, fmt.Errorf("either MINIO_ACCESS_KEY/MINIO_SECRET_KEY or MINIO_ROOT_USER/MINIO_ROOT_PASSWORD must be provided")
	}

	// Создаем клиент MinIO
	client, err := minio.New(cfg.MinioEndpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(accessKey, secretKey, ""),
		Secure: cfg.MinioUseSSL,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create MinIO client: %v", err)
	}

	// Проверяем подключение
	ctx := context.Background()
	exists, errBucketExists := client.BucketExists(ctx, cfg.BucketName)
	if errBucketExists != nil {
		return nil, fmt.Errorf("failed to check if bucket exists: %v", errBucketExists)
	}

	if !exists {
		err = client.MakeBucket(ctx, cfg.BucketName, minio.MakeBucketOptions{})
		if err != nil {
			return nil, fmt.Errorf("failed to create bucket: %v", err)
		}
	}

	return &MinIOClient{
		Client: client,
		Bucket: cfg.BucketName,
	}, nil
}

func (m *MinIOClient) UploadFile(ctx context.Context, key string, reader io.Reader, size int64) error {
	// Определяем Content-Type на основе расширения файла
	contentType := "application/octet-stream"
	if ext := strings.ToLower(filepath.Ext(key)); ext != "" {
		if mimeType := mime.TypeByExtension(ext); mimeType != "" {
			contentType = mimeType
		}
	}

	// Загружаем файл с указанным Content-Type
	_, err := m.Client.PutObject(ctx, m.Bucket, key, reader, size, minio.PutObjectOptions{
		ContentType: contentType,
	})
	if err != nil {
		return fmt.Errorf("failed to upload file: %v", err)
	}
	return nil
}

func (m *MinIOClient) DownloadFile(ctx context.Context, key string) (io.Reader, error) {
	obj, err := m.Client.GetObject(ctx, m.Bucket, key, minio.GetObjectOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to download file: %v", err)
	}
	return obj, nil
}

func (m *MinIOClient) DeleteFile(ctx context.Context, key string) error {
	err := m.Client.RemoveObject(ctx, m.Bucket, key, minio.RemoveObjectOptions{})
	if err != nil {
		return fmt.Errorf("failed to delete file: %v", err)
	}
	return nil
}

// Добавляем метод для получения публичного URL
func (m *MinIOClient) GetPublicURL(key string) string {
	return fmt.Sprintf("/api/files/%s", key)
}
