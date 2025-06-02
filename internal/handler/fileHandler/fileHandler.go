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
	log.Printf("[FileHandler.DownloadFile] ENTERING DOWNLOAD STREAM for file ID: %s", req.FileId)
	for {
		n, errRead := reader.Read(buf)
		log.Printf("[FileHandler.DownloadFile] IO_READ: n=%d, read_err=%v, file_id=%s", n, errRead, req.FileId)

		if n > 0 {
			log.Printf("[FileHandler.DownloadFile] SEND_CHUNK: size=%d, file_id=%s", n, req.FileId)
			if sendErr := stream.Send(&fileproto.DownloadFileResponse{
				Chunk: buf[:n],
			}); sendErr != nil {
				log.Printf("[FileHandler.DownloadFile] SEND_ERROR: err=%v, file_id=%s", sendErr, req.FileId)
				return status.Error(codes.Internal, sendErr.Error())
			}
		}

		if errRead == io.EOF {
			log.Printf("[FileHandler.DownloadFile] EOF_REACHED: file_id=%s", req.FileId)
			break
		}

		if errRead != nil {
			log.Printf("[FileHandler.DownloadFile] READ_ERROR: err=%v, file_id=%s", errRead, req.FileId)
			return status.Error(codes.Internal, errRead.Error())
		}
	}
	log.Printf("[FileHandler.DownloadFile] EXITING DOWNLOAD STREAM for file ID: %s", req.FileId)
	return nil
}

func (h *FileHandler) ListFiles(ctx context.Context, req *fileproto.ListFilesRequest) (*fileproto.ListFilesResponse, error) {
	log.Printf("[fileHandler.ListFiles] Attempting to list files. IncludeShared: %v", req.IncludeShared)
	userID, ok := ctx.Value("userID").(uint32)
	if !ok {
		log.Printf("[fileHandler.ListFiles] ERROR: userID not found in context")
		return nil, status.Error(codes.Unauthenticated, "authentication required")
	}
	log.Printf("[fileHandler.ListFiles] UserID %d retrieved from context", userID)

	files, err := h.fileService.ListFiles(ctx, req.IncludeShared)
	if err != nil {
		log.Printf("[fileHandler.ListFiles] ERROR calling h.fileService.ListFiles: %v", err)
		return nil, status.Error(codes.Internal, err.Error())
	}
	log.Printf("[fileHandler.ListFiles] h.fileService.ListFiles returned %d files", len(files))

	var fileInfos []*fileproto.FileInfo
	for i, file := range files {
		log.Printf("[fileHandler.ListFiles] Processing file %d / %d, ID: %s", i+1, len(files), file.ID.String())
		fileInfo, fileVers, err := h.fileService.GetFileWithVersion(ctx, file.ID)
		if err != nil {
			log.Printf("[fileHandler.ListFiles] ERROR calling h.fileService.GetFileWithVersion for file ID %s: %v", file.ID.String(), err)
			return nil, status.Error(codes.Internal, err.Error())
		}
		log.Printf("[fileHandler.ListFiles] Successfully processed file ID %s. Name: %s, Version: %d", file.ID.String(), fileInfo.Name, fileVers.VersionNumber)

		fileInfos = append(fileInfos, &fileproto.FileInfo{
			FileId:    file.ID.String(),
			Name:      fileInfo.Name,
			Size:      fileVers.Size,
			Version:   fileVers.VersionNumber,
			CreatedAt: file.CreatedAt.Unix(),
			UpdatedAt: fileInfo.CreatedAt.Unix(),
			IsOwner:   file.OwnerID == userID,
		})
	}
	log.Printf("[fileHandler.ListFiles] Successfully prepared %d FileInfo objects for response", len(fileInfos))
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
