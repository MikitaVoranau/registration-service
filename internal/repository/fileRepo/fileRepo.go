package fileRepo

import (
	"context"
	"errors"
	"registration-service/internal/model/fileInfo"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type FileRepository struct {
	conn *pgx.Conn
}

func New(db *pgx.Conn) *FileRepository {
	return &FileRepository{conn: db}
}

func (r *FileRepository) CreateFile(ctx context.Context, file *fileInfo.File) error {
	_, err := r.conn.Exec(ctx,
		`INSERT INTO files (id, owner_id, name, current_version, created_at) 
		 VALUES ($1, $2, $3, $4, $5)`,
		file.ID, file.OwnerID, file.Name, file.CurrentVersion, file.CreatedAt)
	return err
}

func (r *FileRepository) GetFileByID(ctx context.Context, fileID uuid.UUID) (*fileInfo.File, error) {
	var file fileInfo.File
	err := r.conn.QueryRow(ctx,
		`SELECT id, owner_id, name, current_version, created_at 
		 FROM files WHERE id = $1`, fileID).
		Scan(&file.ID, &file.OwnerID, &file.Name, &file.CurrentVersion, &file.CreatedAt)

	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	return &file, err
}

func (r *FileRepository) DeleteFile(ctx context.Context, fileID uuid.UUID) error {
	tx, err := r.conn.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	_, err = tx.Exec(ctx, "DELETE FROM file_permissions WHERE file_id = $1", fileID)
	if err != nil {
		return err
	}

	rows, err := tx.Query(ctx, "SELECT storage_key FROM file_versions WHERE file_id = $1", fileID)
	if err != nil {
		return err
	}
	defer rows.Close()

	var storageKeys []string
	for rows.Next() {
		var key string
		if err := rows.Scan(&key); err != nil {
			return err
		}
		storageKeys = append(storageKeys, key)
	}

	_, err = tx.Exec(ctx, "DELETE FROM file_versions WHERE file_id = $1", fileID)
	if err != nil {
		return err
	}

	_, err = tx.Exec(ctx, "DELETE FROM files WHERE id = $1", fileID)
	if err != nil {
		return err
	}

	return tx.Commit(ctx)
}

func (r *FileRepository) ListFilesByOwner(ctx context.Context, ownerID int) ([]*fileInfo.File, error) {
	rows, err := r.conn.Query(ctx,
		`SELECT id, owner_id, name, current_version, created_at
		 FROM files WHERE owner_id = $1`, ownerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var files []*fileInfo.File
	for rows.Next() {
		var file fileInfo.File
		if err := rows.Scan(
			&file.ID, &file.OwnerID, &file.Name, &file.CurrentVersion, &file.CreatedAt,
		); err != nil {
			return nil, err
		}
		files = append(files, &file)
	}
	return files, nil
}

func (r *FileRepository) CreateFileVersion(ctx context.Context, version *fileInfo.FileVersion) error {
	_, err := r.conn.Exec(ctx,
		`INSERT INTO file_versions (file_id, version_number, storage_key, size, created_at)
		 VALUES ($1, $2, $3, $4, $5)`,
		version.FileID, version.VersionNumber, version.StorageKey, version.Size, version.CreatedAt)
	return err
}

func (r *FileRepository) GetFileVersion(ctx context.Context, fileID uuid.UUID, version int) (*fileInfo.FileVersion, error) {
	var fv fileInfo.FileVersion
	err := r.conn.QueryRow(ctx,
		`SELECT id, file_id, version_number, storage_key, size, created_at
		 FROM file_versions 
		 WHERE file_id = $1 AND version_number = $2`,
		fileID, version).
		Scan(&fv.ID, &fv.FileID, &fv.VersionNumber, &fv.StorageKey, &fv.Size, &fv.CreatedAt)

	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	return &fv, err
}

func (r *FileRepository) GetLatestFileVersion(ctx context.Context, fileID uuid.UUID) (*fileInfo.FileVersion, error) {
	var fv fileInfo.FileVersion
	err := r.conn.QueryRow(ctx,
		`SELECT id, file_id, version_number, storage_key, size, created_at
		 FROM file_versions 
		 WHERE file_id = $1
		 ORDER BY version_number DESC
		 LIMIT 1`,
		fileID).
		Scan(&fv.ID, &fv.FileID, &fv.VersionNumber, &fv.StorageKey, &fv.Size, &fv.CreatedAt)

	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	return &fv, err
}

func (r *FileRepository) UpdateCurrentVersion(ctx context.Context, fileID uuid.UUID, newVersion int) error {
	_, err := r.conn.Exec(ctx,
		"UPDATE files SET current_version = $1 WHERE id = $2",
		newVersion, fileID)
	return err
}

func (r *FileRepository) GetFileVersions(ctx context.Context, fileID uuid.UUID) ([]*fileInfo.FileVersion, error) {
	rows, err := r.conn.Query(ctx,
		`SELECT id, file_id, version_number, storage_key, size, created_at
		 FROM file_versions 
		 WHERE file_id = $1
		 ORDER BY version_number DESC`,
		fileID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var versions []*fileInfo.FileVersion
	for rows.Next() {
		var v fileInfo.FileVersion
		if err := rows.Scan(&v.ID, &v.FileID, &v.VersionNumber, &v.StorageKey, &v.Size, &v.CreatedAt); err != nil {
			return nil, err
		}
		versions = append(versions, &v)
	}
	return versions, nil
}

func (r *FileRepository) SetFilePermissions(ctx context.Context, fileID uuid.UUID, permissions []fileInfo.FilePermission) error {
	tx, err := r.conn.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	_, err = tx.Exec(ctx, "DELETE FROM file_permissions WHERE file_id = $1", fileID)
	if err != nil {
		return err
	}

	for _, perm := range permissions {
		_, err = tx.Exec(ctx,
			`INSERT INTO file_permissions (file_id, user_id, permission)
			 VALUES ($1, $2, $3)`,
			perm.FileID, perm.UserID, perm.Permission)
		if err != nil {
			return err
		}
	}

	return tx.Commit(ctx)
}

func (r *FileRepository) GetFilePermissions(ctx context.Context, fileID uuid.UUID) ([]fileInfo.FilePermission, error) {
	rows, err := r.conn.Query(ctx,
		`SELECT file_id, user_id, permission 
		 FROM file_permissions 
		 WHERE file_id = $1`,
		fileID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var permissions []fileInfo.FilePermission
	for rows.Next() {
		var p fileInfo.FilePermission
		if err := rows.Scan(&p.FileID, &p.UserID, &p.Permission); err != nil {
			return nil, err
		}
		permissions = append(permissions, p)
	}
	return permissions, nil
}

func (r *FileRepository) CheckUserPermission(ctx context.Context, fileID uuid.UUID, userID int) (int, error) {
	var permission int
	err := r.conn.QueryRow(ctx,
		`SELECT permission 
		 FROM file_permissions 
		 WHERE file_id = $1 AND user_id = $2`,
		fileID, userID).Scan(&permission)

	if errors.Is(err, pgx.ErrNoRows) {
		return 0, nil
	}
	return permission, err
}

func (r *FileRepository) GetSharedFiles(ctx context.Context, userID int) ([]*fileInfo.File, error) {
	rows, err := r.conn.Query(ctx,
		`SELECT f.id, f.owner_id, f.name, f.current_version, f.created_at
		 FROM files f
		 JOIN file_permissions fp ON f.id = fp.file_id
		 WHERE fp.user_id = $1`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var files []*fileInfo.File
	for rows.Next() {
		var file fileInfo.File
		if err := rows.Scan(
			&file.ID, &file.OwnerID, &file.Name, &file.CurrentVersion, &file.CreatedAt,
		); err != nil {
			return nil, err
		}
		files = append(files, &file)
	}
	return files, nil
}

func (r *FileRepository) RenameFile(ctx context.Context, fileID uuid.UUID, newName string) error {
	_, err := r.conn.Exec(ctx,
		"UPDATE files SET name = $1 WHERE id = $2",
		newName, fileID)
	return err
}

func (r *FileRepository) FileExists(ctx context.Context, fileID uuid.UUID) (bool, error) {
	var exists bool
	err := r.conn.QueryRow(ctx,
		"SELECT EXISTS(SELECT 1 FROM files WHERE id = $1)",
		fileID).Scan(&exists)
	return exists, err
}
