package fileService

import (
	"context"
	"errors"
	"fmt"
	"io"
	auth "registration-service/api/authproto/proto-generate"
	"registration-service/internal/MinIO"
	"registration-service/internal/model/fileInfo"
	"registration-service/internal/repository/fileRepo"
	"strconv"
	"time"

	"log"

	"github.com/google/uuid"
)

type FileService struct {
	fileRepo   *fileRepo.FileRepository
	authClient auth.AuthServiceClient
	minIO      *MinIO.MinIOClient
}

func New(fileRepo *fileRepo.FileRepository, authClient auth.AuthServiceClient, minIO *MinIO.MinIOClient) *FileService {
	return &FileService{
		fileRepo:   fileRepo,
		authClient: authClient,
		minIO:      minIO,
	}
}

// rpc SetFilePermissions(SetFilePermissionsRequest) returns (SetFilePermissionsResponse);
// rpc GetFileVersions(GetFileVersionsRequest) returns (GetFileVersionsResponse);
// rpc RevertFileVersion(RevertFileRequest) returns (RevertFileResponse);
func getUserIDFromContext(ctx context.Context) (uint32, error) {
	userIDVal := ctx.Value("userID")
	if userIDVal == nil {
		return 0, errors.New("userID not found in context")
	}
	userID, ok := userIDVal.(uint32)
	if !ok {
		if userIDStr, strOk := userIDVal.(string); strOk {
			parsedID, err := strconv.ParseUint(userIDStr, 10, 32)
			if err == nil {
				return uint32(parsedID), nil
			}
		}
		return 0, errors.New("userID in context is not of type uint32 or valid string representation")
	}
	return userID, nil
}

func (s *FileService) UploadFile(ctx context.Context, name string, content_type string, fileData io.Reader, size int64) (*fileInfo.File, error) {
	if userIDVal := ctx.Value("userID"); userIDVal != nil {
		log.Printf("[FileService.UploadFile] Received context. userID found. Value: %v, Type: %T", userIDVal, userIDVal)
	} else {
		log.Printf("[FileService.UploadFile] Received context. ERROR: userID NOT FOUND with key 'userID'!")
	}

	userID, err := getUserIDFromContext(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get user ID: %v", err)
	}

	fileID := uuid.New()
	version := 1
	storageKey := fmt.Sprintf("%s/v%d", fileID, version)
	if err := s.minIO.UploadFile(ctx, storageKey, fileData, size, content_type); err != nil {
		return nil, errors.New("upload file to minio error")
	}
	file := &fileInfo.File{
		ID:             fileID,
		OwnerID:        userID,
		Name:           name,
		CurrentVersion: version,
		CreatedAt:      time.Now(),
	}
	if err := s.fileRepo.CreateFile(ctx, file); err != nil {
		_ = s.minIO.DeleteFile(ctx, storageKey)
		return nil, fmt.Errorf("create file entry error: %w", err)
	}

	initialFileVersion := &fileInfo.FileVersion{
		FileID:        fileID,
		VersionNumber: uint32(version),
		StorageKey:    storageKey,
		Size:          size,
		CreatedAt:     time.Now(),
	}
	if err := s.fileRepo.CreateFileVersion(ctx, initialFileVersion); err != nil {
		_ = s.minIO.DeleteFile(ctx, storageKey)
		_ = s.fileRepo.DeleteFile(ctx, fileID)
		return nil, fmt.Errorf("failed to create initial file version: %w", err)
	}

	return file, nil
}

func (s *FileService) DownloadFile(ctx context.Context, fileID uuid.UUID) (io.Reader, *fileInfo.File, error) {
	userID, err := getUserIDFromContext(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get user ID: %v", err)
	}
	file, err := s.fileRepo.GetFileByID(ctx, fileID)
	if err != nil {
		return nil, nil, errors.New("get file error")
	}
	if file == nil {
		return nil, nil, errors.New("file not found")
	}
	hasAccess, err := s.checkFileAccess(ctx, fileID, int(userID))
	if err != nil || !hasAccess {
		return nil, nil, errors.New("access denied")
	}
	versionNum, err := s.fileRepo.GetLatestFileVersion(ctx, fileID)
	if err != nil {
		return nil, nil, errors.New("get latest file version error")
	}
	if versionNum == nil {
		return nil, nil, errors.New("file version not found")
	}
	reader, err := s.minIO.DownloadFile(ctx, versionNum.StorageKey)
	if err != nil {
		return nil, nil, errors.New("download file to minio error")
	}
	return reader, file, nil
}

func (s *FileService) DeleteFile(ctx context.Context, fileID uuid.UUID) error {
	userID, err := getUserIDFromContext(ctx)
	if err != nil {
		return fmt.Errorf("failed to get user ID: %v", err)
	}
	file, err := s.fileRepo.GetFileByID(ctx, fileID)
	if err != nil {
		return fmt.Errorf("failed to get file: %w", err)
	}
	if file == nil {
		return errors.New("file not found")
	}
	if file.OwnerID != userID {
		return errors.New("only owner can delete file")
	}
	versions, err := s.fileRepo.GetFileVersions(ctx, fileID)
	if err != nil {
		return fmt.Errorf("failed to get file versions: %w", err)
	}
	if err := s.fileRepo.DeleteFile(ctx, fileID); err != nil {
		return fmt.Errorf("failed to delete file: %w", err)
	}
	for _, versionToDelete := range versions {
		if err := s.minIO.DeleteFile(ctx, versionToDelete.StorageKey); err != nil {
			return fmt.Errorf("failed to delete file: %w", err)
		}
	}
	return nil
}

func (s *FileService) ListFiles(ctx context.Context, includeShared bool) ([]*fileInfo.File, error) {
	userID, err := getUserIDFromContext(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get user ID: %v", err)
	}

	files, err := s.fileRepo.ListFilesByOwner(ctx, int(userID))
	if err != nil {
		return nil, fmt.Errorf("failed to list user files: %w", err)
	}

	if includeShared {
		sharedFiles, err := s.fileRepo.GetSharedFiles(ctx, int(userID))
		if err != nil {
			return nil, fmt.Errorf("failed to list shared files: %w", err)
		}
		files = append(files, sharedFiles...)
	}

	return files, nil
}

func (s *FileService) GetFileInfo(ctx context.Context, fileID uuid.UUID) (*fileInfo.File, *fileInfo.FileVersion, error) {
	userID, err := getUserIDFromContext(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get user ID: %v", err)
	}
	hasAccess, err := s.checkFileAccess(ctx, fileID, int(userID))
	if err != nil || !hasAccess {
		return nil, nil, errors.New("access denied")
	}
	file, err := s.fileRepo.GetFileByID(ctx, fileID)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get file: %w", err)
	}
	if file == nil {
		return nil, nil, errors.New("file not found")
	}
	version, err := s.fileRepo.GetLatestFileVersion(ctx, fileID)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get latest version: %w", err)
	}
	if version == nil {
		return nil, nil, errors.New("file version not found")
	}
	return file, version, nil
}

func (s *FileService) RenameFile(ctx context.Context, fileID uuid.UUID, newName string) error {
	userID, err := getUserIDFromContext(ctx)
	if err != nil {
		return fmt.Errorf("failed to get user ID: %v", err)
	}
	file, err := s.fileRepo.GetFileByID(ctx, fileID)
	if err != nil {
		return fmt.Errorf("failed to get file: %w", err)
	}
	if file == nil {
		return errors.New("file not found")
	}
	if file.OwnerID != userID {
		return errors.New("only owner can rename file")
	}
	if err := s.fileRepo.RenameFile(ctx, fileID, newName); err != nil {
		return fmt.Errorf("failed to rename file: %w", err)
	}
	return nil
}

func (s *FileService) SetFilePermissions(ctx context.Context, fileID uuid.UUID, permissions []fileInfo.FilePermission) error {
	userID, err := getUserIDFromContext(ctx)
	if err != nil {
		return fmt.Errorf("failed to get user ID: %v", err)
	}
	file, err := s.fileRepo.GetFileByID(ctx, fileID)
	if err != nil {
		return fmt.Errorf("failed to get file: %w", err)
	}
	if file == nil {
		return errors.New("file not found")
	}
	if file.OwnerID != userID {
		return errors.New("only owner can set file permissions")
	}
	if err := s.fileRepo.SetFilePermissions(ctx, fileID, permissions); err != nil {
		return fmt.Errorf("failed to set file permissions: %w", err)
	}
	return nil
}

func (s *FileService) GetFileVersions(ctx context.Context, fileID uuid.UUID) ([]*fileInfo.FileVersion, error) {
	usersID, ok := ctx.Value("userID").(uint32)
	if !ok {
		return nil, errors.New("user not authenticated")
	}
	file, err := s.fileRepo.GetFileByID(ctx, fileID)
	if err != nil {
		return nil, fmt.Errorf("failed to get file: %w", err)
	}
	if file == nil {
		return nil, errors.New("file not found")
	}
	hasAccess, err := s.checkFileAccess(ctx, fileID, int(usersID))
	if err != nil || !hasAccess {
		return nil, errors.New("access denied")
	}
	versions, err := s.fileRepo.GetFileVersions(ctx, fileID)
	if err != nil {
		return nil, fmt.Errorf("failed to get file versions: %w", err)
	}
	return versions, nil
}

func (s *FileService) RevertFileVersion(ctx context.Context, fileID uuid.UUID, versionNum int) (*fileInfo.File, error) {
	userID, err := getUserIDFromContext(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get user ID: %v", err)
	}
	file, err := s.fileRepo.GetFileByID(ctx, fileID)
	if err != nil {
		return nil, fmt.Errorf("failed to get file: %w", err)
	}
	if file == nil {
		return nil, errors.New("file not found")
	}
	if file.OwnerID != userID {
		return nil, errors.New("only owner can revert file")
	}
	oldVersion, err := s.fileRepo.GetFileVersion(ctx, fileID, versionNum)
	if err != nil {
		return nil, fmt.Errorf("failed to get file version: %w", err)
	}
	if oldVersion == nil {
		return nil, errors.New("file version not found")
	}
	newVersion := file.CurrentVersion + 1
	newStorageKey := fmt.Sprintf("%s/v%d", fileID, newVersion)

	reader, err := s.minIO.DownloadFile(ctx, oldVersion.StorageKey)
	if err != nil {
		return nil, fmt.Errorf("download file to minio error: %w", err)
	}
	if err := s.minIO.UploadFile(ctx, newStorageKey, reader, oldVersion.Size, ""); err != nil {
		return nil, fmt.Errorf("upload file to minio error: %w", err)
	}

	file.CurrentVersion = newVersion
	if err := s.fileRepo.UpdateCurrentVersion(ctx, file.ID, file.CurrentVersion); err != nil {
		_ = s.minIO.DeleteFile(ctx, newStorageKey)
		return nil, fmt.Errorf("failed to update file record: %w", err)
	}

	newFileVers := &fileInfo.FileVersion{
		FileID:        fileID,
		VersionNumber: uint32(newVersion),
		StorageKey:    newStorageKey,
		Size:          oldVersion.Size,
		CreatedAt:     time.Now(),
	}
	if err := s.fileRepo.CreateFileVersion(ctx, newFileVers); err != nil {
		_ = s.minIO.DeleteFile(ctx, newStorageKey)
		return nil, fmt.Errorf("failed to create new file version: %w", err)
	}
	return file, nil
}

func (s *FileService) checkFileAccess(ctx context.Context, fileID uuid.UUID, userID int) (bool, error) {
	file, err := s.fileRepo.GetFileByID(ctx, fileID)
	if err != nil {
		return false, fmt.Errorf("failed to get file: %w", err)
	}
	if file == nil {
		return false, errors.New("file not found")
	}
	if file.OwnerID == uint32(userID) {
		return true, nil
	}
	permission, err := s.fileRepo.CheckUserPermission(ctx, fileID, userID)
	if err != nil {
		return false, fmt.Errorf("failed to check user permissions: %w", err)
	}
	return permission > 0, nil
}

func (s *FileService) GetFileWithVersion(ctx context.Context, fileID uuid.UUID) (*fileInfo.File, *fileInfo.FileVersion, error) {
	file, err := s.fileRepo.GetFileByID(ctx, fileID)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get file: %w", err)
	}
	if file == nil {
		return nil, nil, errors.New("file not found")
	}

	version, err := s.fileRepo.GetLatestFileVersion(ctx, fileID)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get file version: %w", err)
	}

	if version == nil {
		return nil, nil, fmt.Errorf("latest file version not found for file ID %s", fileID.String())
	}

	return file, version, nil
}
