package fileInfo

import (
	"github.com/google/uuid"
	"time"
)

type File struct {
	ID             uuid.UUID `json:"id"`
	OwnerID        uint32    `json:"owner_id"`
	Name           string    `json:"name"`
	CurrentVersion int       `json:"current_version"`
	CreatedAt      time.Time `json:"created_at"`
}

type FileVersion struct {
	ID            uint32    `json:"id"`
	FileID        uuid.UUID `json:"file_id"`
	VersionNumber uint32    `json:"version_number"`
	StorageKey    string    `json:"storage_key"`
	Size          int64     `json:"size"`
	CreatedAt     time.Time `json:"created_at"`
}

type FilePermission struct {
	FileID     uuid.UUID `json:"file_id"`
	UserID     int32     `json:"user_id"`
	Permission int       `json:"permission"`
}
