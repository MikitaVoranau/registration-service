package fileHandler

import (
	"bytes"
	"context"
	"io"
	"log"
	fileproto "registration-service/api/fileproto/proto-generate"
	"registration-service/internal/model/fileInfo"
	"registration-service/internal/service/fileService"

	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type FileHandler struct {
	fileService *fileService.FileService
	fileproto.UnimplementedFileServiceServer
}

func NewFileHandler(fileService *fileService.FileService) *FileHandler {
	return &FileHandler{fileService: fileService}
}

func (h *FileHandler) UploadFile(stream fileproto.FileService_UploadFileServer) error {
	ctx := stream.Context()
	var metadata *fileproto.FileMetadata
	var fileData []byte
	for {
		req, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			return status.Error(codes.Internal, err.Error())
		}
		switch data := req.Data.(type) {
		case *fileproto.UploadFileRequest_Metadata:
			metadata = data.Metadata
		case *fileproto.UploadFileRequest_Chunk:
			fileData = append(fileData, data.Chunk...)
		}
	}
	if metadata == nil {
		return status.Error(codes.InvalidArgument, "metadata is required")
	}
	file, err := h.fileService.UploadFile(ctx, metadata.Name, metadata.ContentType, bytes.NewReader(fileData), int64(len(fileData)))
	if err != nil {
		return status.Error(codes.Internal, err.Error())
	}
	return stream.SendAndClose(&fileproto.UploadFileResponse{
		FileId:  file.ID.String(),
		Message: "File uploaded successfully",
	})
}

func (h *FileHandler) DownloadFile(req *fileproto.DownloadFileRequest, stream fileproto.FileService_DownloadFileServer) error {
	ctx := stream.Context()
	fileID, err := uuid.Parse(req.FileId)
	if err != nil {
		return status.Error(codes.InvalidArgument, "invalid file id")
	}
	reader, _, err := h.fileService.DownloadFile(ctx, fileID)
	if err != nil {
		return status.Error(codes.Internal, "cannot downlaod file")
	}
	defer reader.(io.Closer).Close()
	buf := make([]byte, 1024*32)
	for {
		n, err := reader.Read(buf)
		if err == io.EOF {
			break
		}
		if err != nil {
			return status.Error(codes.Internal, err.Error())
		}
		if err = stream.Send(&fileproto.DownloadFileResponse{
			Chunk: buf[:n],
		}); err != nil {
			return status.Error(codes.Internal, err.Error())

		}
	}
	return nil
}

func (h *FileHandler) ListFiles(ctx context.Context, req *fileproto.ListFilesRequest) (*fileproto.ListFilesResponse, error) {
	userID, ok := ctx.Value("userID").(uint32)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "authentication required")
	}

	files, err := h.fileService.ListFiles(ctx, req.IncludeShared)
	if err != nil {
		log.Printf("ERROR: ListFiles - h.fileService.ListFiles failed: %v", err)
		return nil, status.Error(codes.Internal, "failed to retrieve file list")
	}

	var fileInfos []*fileproto.FileInfo
	for _, file := range files {
		_, currentVersionInfo, err := h.fileService.GetFileWithVersion(ctx, file.ID)
		if err != nil {
			log.Printf("ERROR: ListFiles - GetFileWithVersion for file ID %s failed: %v", file.ID.String(), err)
			return nil, status.Error(codes.Internal, "failed to retrieve details for a file")
		}

		if currentVersionInfo == nil {
			log.Printf("WARN: ListFiles - GetFileWithVersion for file ID %s returned nil version info without error. Skipping.", file.ID.String())
			continue
		}

		fileInfos = append(fileInfos, &fileproto.FileInfo{
			FileId:    file.ID.String(),
			Name:      file.Name,
			Size:      currentVersionInfo.Size,
			Version:   uint32(file.CurrentVersion),
			CreatedAt: file.CreatedAt.Unix(),
			UpdatedAt: currentVersionInfo.CreatedAt.Unix(),
			IsOwner:   file.OwnerID == userID,
		})
	}

	return &fileproto.ListFilesResponse{Files: fileInfos}, nil
}

func (h *FileHandler) DeleteFile(ctx context.Context, req *fileproto.DeleteFileRequest) (*fileproto.DeleteFileResponse, error) {
	fileID, err := uuid.Parse(req.FileId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid file id")
	}
	if err = h.fileService.DeleteFile(ctx, fileID); err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	return &fileproto.DeleteFileResponse{
		Success: true,
		Message: "File deleted successfully",
	}, nil
}

//rpc RevertFileVersion(RevertFileRequest) returns (RevertFileResponse);

func (h *FileHandler) GetFileInfo(ctx context.Context, req *fileproto.GetFileInfoRequest) (*fileproto.GetFileInfoResponse, error) {
	fileID, err := uuid.Parse(req.FileId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid file id")
	}
	fileInfo, fileVers, err := h.fileService.GetFileInfo(ctx, fileID)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	userID := ctx.Value("userID").(uint32)
	return &fileproto.GetFileInfoResponse{
		File: &fileproto.FileInfo{
			FileId:      fileID.String(),
			Name:        fileInfo.Name,
			Size:        fileVers.Size,
			Version:     uint32(fileInfo.CurrentVersion),
			ContentType: "application/octet-stream",
			CreatedAt:   fileInfo.CreatedAt.Unix(),
			UpdatedAt:   fileVers.CreatedAt.Unix(),
			IsOwner:     fileInfo.OwnerID == userID,
		},
	}, nil
}

func (h *FileHandler) RenameFile(ctx context.Context, req *fileproto.RenameFileRequest) (*fileproto.RenameFileResponse, error) {
	fileID, err := uuid.Parse(req.FileId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid file id")
	}
	if err := h.fileService.RenameFile(ctx, fileID, req.NewName); err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	return &fileproto.RenameFileResponse{
		Success: true,
	}, nil
}

func (h *FileHandler) SetFilePermissions(ctx context.Context, req *fileproto.SetFilePermissionsRequest) (*fileproto.SetFilePermissionsResponse, error) {
	fileID, err := uuid.Parse(req.FileId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid file id")
	}
	var permissions []fileInfo.FilePermission
	for _, permission := range req.Permissions {
		permissions = append(permissions, fileInfo.FilePermission{
			UserID:     permission.UserId,
			Permission: int(permission.PermissionType),
		})
	}
	if err := h.fileService.SetFilePermissions(ctx, fileID, permissions); err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	return &fileproto.SetFilePermissionsResponse{Success: true}, nil
}

func (h *FileHandler) GetFileVersions(ctx context.Context, req *fileproto.GetFileVersionsRequest) (*fileproto.GetFileVersionsResponse, error) {
	fileID, err := uuid.Parse(req.FileId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid file id")
	}
	versions, err := h.fileService.GetFileVersions(ctx, fileID)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	var fileVers []*fileproto.FileVersionInfo
	for _, version := range versions {
		fileVers = append(fileVers, &fileproto.FileVersionInfo{
			VersionNumber: uint32(version.VersionNumber),
			Size:          version.Size,
			CreatedAt:     version.CreatedAt.Unix(),
		})
	}
	return &fileproto.GetFileVersionsResponse{Versions: fileVers}, nil
}

func (h *FileHandler) RevertFileVersion(ctx context.Context, req *fileproto.RevertFileRequest) (*fileproto.RevertFileResponse, error) {
	fileID, err := uuid.Parse(req.FileId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid file ID")
	}

	newFile, err := h.fileService.RevertFileVersion(ctx, fileID, int(req.Version))
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	return &fileproto.RevertFileResponse{
		Success:   true,
		NewFileId: newFile.ID.String(),
	}, nil
}
