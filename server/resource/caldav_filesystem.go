package resource

import (
	"io"
	"os"
	"path"
	"strings"
	"time"

	"github.com/daptin/daptin/server/auth"
	"github.com/daptin/daptin/server/statementbuilder"
	"github.com/doug-martin/goqu/v9"
	"github.com/emersion/go-webdav"
	"github.com/google/uuid"
	log "github.com/sirupsen/logrus"
)

// DaptinCaldavFileSystem implements webdav.FileSystem using database
// Pattern: like DaptinImapUser (imap_user.go:15-24)
type DaptinCaldavFileSystem struct {
	cruds       map[string]*DbResource
	sessionUser *auth.SessionUser
}

// Path structure:
// /users/{user_reference_id}/calendars/{collection_name}/{event.ics}

// parsePath extracts components from CalDAV path
func (dcfs *DaptinCaldavFileSystem) parsePath(name string) (userID, collectionName, resourceName string, isRoot, isUserRoot, isCollectionRoot bool) {
	// Strip /caldav/ or /carddav/ prefix if present
	name = strings.TrimPrefix(name, "/caldav")
	name = strings.TrimPrefix(name, "/carddav")

	parts := strings.Split(strings.Trim(name, "/"), "/")

	// Path formats:
	// / → isRoot=true
	// /users/{user_id} → isUserRoot=true
	// /users/{user_id}/calendars/{collection_name} → isCollectionRoot=true
	// /users/{user_id}/calendars/{collection_name}/event.ics → full path

	if len(parts) == 0 || parts[0] == "" {
		isRoot = true
		return
	}

	if len(parts) == 2 && parts[0] == "users" {
		userID = parts[1]
		isUserRoot = true
		return
	}

	if len(parts) == 4 && parts[0] == "users" && parts[2] == "calendars" {
		userID = parts[1]
		collectionName = parts[3]
		isCollectionRoot = true
		return
	}

	if len(parts) >= 5 && parts[0] == "users" && parts[2] == "calendars" {
		userID = parts[1]
		collectionName = parts[3]
		resourceName = strings.Join(parts[4:], "/")
		return
	}

	return
}

// validateUserOwnership checks that the parsed userID matches the session user
// Returns error if user is trying to access another user's resources
func (dcfs *DaptinCaldavFileSystem) validateUserOwnership(parsedUserID string) error {
	// Convert session user's reference_id to string for comparison
	sessionUserRefID := dcfs.sessionUser.UserReferenceId.String()

	log.Printf("[CALDAV SECURITY DEBUG] validateUserOwnership called: parsed=%s, session=%s", parsedUserID, sessionUserRefID)

	if parsedUserID == "" {
		// Empty user ID in path - could be:
		// 1. Legacy path format without user ID
		// 2. Malformed request
		// For security, we log this but allow it only if we're at a root/directory level
		// (those cases are already handled by checking isRoot/isUserRoot/isCollectionRoot)
		// If we reach this validation, it means we need a user ID
		log.Printf("[CALDAV SECURITY] Empty user ID in path, assuming current user: %s", sessionUserRefID)
		// Allow it to proceed - the database queries will use sessionUser.UserId anyway
		return nil
	}

	if parsedUserID != sessionUserRefID {
		log.Printf("[CALDAV SECURITY] ⚠️  BLOCKED: User %s attempted to access user %s's resources",
			sessionUserRefID, parsedUserID)
		return os.ErrPermission
	}

	log.Printf("[CALDAV SECURITY] ✓ User %s accessing own resources", sessionUserRefID)
	return nil
}

// Open opens a file for reading
// Pattern: Direct SQL query - CalDAV stores raw iCalendar content, not file references
// Note: ForeignKeyData is configured on the column but CalDAV bypasses the JSON API file format
func (dcfs *DaptinCaldavFileSystem) Open(name string) (io.ReadCloser, error) {
	userID, collectionName, resourceName, isRoot, isUserRoot, isCollectionRoot := dcfs.parsePath(name)

	// Validate user ownership of path
	if err := dcfs.validateUserOwnership(userID); err != nil {
		return nil, err
	}

	if isRoot || isUserRoot || isCollectionRoot {
		// Directories cannot be opened for reading
		return nil, os.ErrInvalid
	}

	transaction, err := dcfs.cruds["calendar"].Connection().Beginx()
	if err != nil {
		return nil, err
	}
	defer transaction.Commit()

	// Build rpath: /calendars/{collection_name}/{resource_name}
	rpath := path.Join("/calendars", collectionName, resourceName)

	// Use direct SQL to read raw content (not file references)
	query, args, err := statementbuilder.Squirrel.
		Select("content").
		From("calendar").
		Where(goqu.Ex{
			"rpath":           rpath,
			"user_account_id": dcfs.sessionUser.UserId,
		}).
		Prepared(true).
		ToSQL()

	if err != nil {
		return nil, err
	}

	stmt, err := transaction.Preparex(query)
	if err != nil {
		return nil, err
	}
	defer stmt.Close()

	var content []byte
	err = stmt.QueryRow(args...).Scan(&content)
	if err != nil {
		return nil, os.ErrNotExist
	}

	return &caldavFile{
		content: content,
		offset:  0,
	}, nil
}

// Stat returns file/directory information
func (dcfs *DaptinCaldavFileSystem) Stat(name string) (*webdav.FileInfo, error) {
	userID, collectionName, resourceName, isRoot, isUserRoot, isCollectionRoot := dcfs.parsePath(name)

	// Validate user ownership first
	if err := dcfs.validateUserOwnership(userID); err != nil {
		return nil, err
	}

	if isRoot || isUserRoot || isCollectionRoot {
		// Directory
		return &webdav.FileInfo{
			Path:    name,
			Size:    0,
			ModTime: time.Now(),
			IsDir:   true,
		}, nil
	}

	// File - check if exists
	transaction, err := dcfs.cruds["calendar"].Connection().Beginx()
	if err != nil {
		return nil, err
	}
	defer transaction.Commit()

	rpath := path.Join("/calendars", collectionName, resourceName)

	// Use direct SQL to read raw content and timestamps
	query, args, err := statementbuilder.Squirrel.
		Select("content", goqu.L("COALESCE(updated_at, created_at)").As("mod_time")).
		From("calendar").
		Where(goqu.Ex{
			"rpath":           rpath,
			"user_account_id": dcfs.sessionUser.UserId,
		}).
		Prepared(true).
		ToSQL()

	if err != nil {
		return nil, err
	}

	stmt, err := transaction.Preparex(query)
	if err != nil {
		return nil, err
	}
	defer stmt.Close()

	var content []byte
	var modTimeStr string
	err = stmt.QueryRow(args...).Scan(&content, &modTimeStr)
	if err != nil {
		return nil, os.ErrNotExist
	}

	// Parse the time string from SQLite
	modTime, err := time.Parse("2006-01-02 15:04:05", modTimeStr)
	if err != nil {
		log.Errorf("Failed to parse calendar timestamp %s: %v", modTimeStr, err)
		modTime = time.Now()
	}

	return &webdav.FileInfo{
		Path:    name,
		Size:    int64(len(content)),
		ModTime: modTime,
		IsDir:   false,
	}, nil
}

// Readdir lists directory contents
// Pattern: like IMAP ListMailboxes (imap_user.go:35-95)
func (dcfs *DaptinCaldavFileSystem) Readdir(name string, recursive bool) ([]webdav.FileInfo, error) {
	userID, collectionName, _, isRoot, isUserRoot, isCollectionRoot := dcfs.parsePath(name)

	// Validate user ownership of path
	if err := dcfs.validateUserOwnership(userID); err != nil {
		return nil, err
	}

	if isRoot {
		// List /users/{user_reference_id}/
		return []webdav.FileInfo{
			{
				Path:    "/users",
				Size:    0,
				ModTime: time.Now(),
				IsDir:   true,
			},
		}, nil
	}

	if isUserRoot {
		// List /users/{user_id}/calendars/
		return []webdav.FileInfo{
			{
				Path:    path.Join(name, "calendars"),
				Size:    0,
				ModTime: time.Now(),
				IsDir:   true,
			},
		}, nil
	}

	if isCollectionRoot {
		// List calendar events in collection
		transaction, err := dcfs.cruds["calendar"].Connection().Beginx()
		if err != nil {
			return nil, err
		}
		defer transaction.Commit()

		// Get collection_id first (use ORM for this - it doesn't involve file columns)
		collections, err := dcfs.cruds["collection"].GetAllObjectsWithWhereWithTransaction(
			"collection",
			transaction,
			goqu.Ex{
				"name":            collectionName,
				"user_account_id": dcfs.sessionUser.UserId,
			},
		)

		if err != nil || len(collections) == 0 {
			return nil, os.ErrNotExist
		}

		collectionID := collections[0]["id"].(int64)

		// List all calendar items using direct SQL to read raw content
		query, args, err := statementbuilder.Squirrel.
			Select("rpath", "content", goqu.L("COALESCE(updated_at, created_at)").As("mod_time")).
			From("calendar").
			Where(goqu.Ex{"collection_id": collectionID}).
			Prepared(true).
			ToSQL()

		if err != nil {
			return nil, err
		}

		stmt, err := transaction.Preparex(query)
		if err != nil {
			return nil, err
		}
		defer stmt.Close()

		rows, err := stmt.Queryx(args...)
		if err != nil {
			return nil, err
		}
		defer rows.Close()

		var fileInfos []webdav.FileInfo
		for rows.Next() {
			var rpath string
			var content []byte
			var modTimeStr string

			err = rows.Scan(&rpath, &content, &modTimeStr)
			if err != nil {
				log.Errorf("Failed to scan calendar row: %v", err)
				continue
			}

			// Parse the time string from SQLite
			modTime, err := time.Parse("2006-01-02 15:04:05", modTimeStr)
			if err != nil {
				log.Errorf("Failed to parse time %s: %v, using current time", modTimeStr, err)
				modTime = time.Now()
			}

			// Extract filename from rpath: /calendars/{collection}/filename.ics
			parts := strings.Split(strings.Trim(rpath, "/"), "/")
			filename := parts[len(parts)-1]

			fileInfos = append(fileInfos, webdav.FileInfo{
				Path:    path.Join(name, filename),
				Size:    int64(len(content)),
				ModTime: modTime,
				IsDir:   false,
			})
		}

		return fileInfos, nil
	}

	return nil, os.ErrNotExist
}

// Create creates a new file and returns a writer
func (dcfs *DaptinCaldavFileSystem) Create(name string) (io.WriteCloser, error) {
	userID, collectionName, resourceName, _, _, _ := dcfs.parsePath(name)

	// Validate user ownership of path
	if err := dcfs.validateUserOwnership(userID); err != nil {
		return nil, err
	}

	if collectionName == "" || resourceName == "" {
		return nil, os.ErrInvalid
	}

	return &caldavFileWriter{
		fs:             dcfs,
		collectionName: collectionName,
		resourceName:   resourceName,
		buffer:         make([]byte, 0),
	}, nil
}

// RemoveAll deletes a file or directory
// Pattern: uses SQL DELETE (dbresource.go:689-701)
func (dcfs *DaptinCaldavFileSystem) RemoveAll(name string) error {
	userID, collectionName, resourceName, _, _, isCollectionRoot := dcfs.parsePath(name)

	// Validate user ownership of path
	if err := dcfs.validateUserOwnership(userID); err != nil {
		return err
	}

	transaction, err := dcfs.cruds["calendar"].Connection().Beginx()
	if err != nil {
		return err
	}
	defer transaction.Commit()

	if isCollectionRoot {
		// Delete collection - first delete all items, then collection
		collections, err := dcfs.cruds["collection"].GetAllObjectsWithWhereWithTransaction(
			"collection",
			transaction,
			goqu.Ex{
				"name":            collectionName,
				"user_account_id": dcfs.sessionUser.UserId,
			},
		)

		if err != nil || len(collections) == 0 {
			return os.ErrNotExist
		}

		collectionID := collections[0]["id"].(int64)

		// Delete all calendar items in this collection
		// Pattern: statementbuilder.Squirrel.Delete (dbresource.go:689-701)
		query, args, err := statementbuilder.Squirrel.Delete("calendar").Prepared(true).Where(goqu.Ex{
			"collection_id": collectionID,
		}).ToSQL()

		if err != nil {
			return err
		}

		_, err = transaction.Exec(query, args...)
		if err != nil {
			return err
		}

		// Delete collection
		query, args, err = statementbuilder.Squirrel.Delete("collection").Prepared(true).Where(goqu.Ex{
			"id": collectionID,
		}).ToSQL()

		if err != nil {
			return err
		}

		_, err = transaction.Exec(query, args...)
		return err
	} else {
		// Delete single file
		rpath := path.Join("/calendars", collectionName, resourceName)

		query, args, err := statementbuilder.Squirrel.Delete("calendar").Prepared(true).Where(goqu.Ex{
			"rpath":           rpath,
			"user_account_id": dcfs.sessionUser.UserId,
		}).ToSQL()

		if err != nil {
			return err
		}

		_, err = transaction.Exec(query, args...)
		return err
	}
}

// Mkdir creates a directory (collection)
// Pattern: like IMAP CreateMailbox (imap_user.go:265-286)
func (dcfs *DaptinCaldavFileSystem) Mkdir(name string) error {
	log.Printf("[CALDAV] Mkdir called with name=%s", name)
	userID, collectionName, _, _, _, _ := dcfs.parsePath(name)
	log.Printf("[CALDAV] Parsed path: collectionName=%s", collectionName)

	// Validate user ownership of path
	if err := dcfs.validateUserOwnership(userID); err != nil {
		return err
	}

	if collectionName == "" {
		log.Printf("[CALDAV] Collection name empty, returning ErrInvalid")
		return os.ErrInvalid
	}

	transaction, err := dcfs.cruds["collection"].Connection().Beginx()
	if err != nil {
		return err
	}
	defer transaction.Commit()

	// Check if collection already exists
	// Pattern: GetAllObjectsWithWhereWithTransaction (imap_user.go:49-52)
	existing, err := dcfs.cruds["collection"].GetAllObjectsWithWhereWithTransaction(
		"collection",
		transaction,
		goqu.Ex{
			"name":            collectionName,
			"user_account_id": dcfs.sessionUser.UserId,
		},
	)

	if len(existing) > 0 {
		return os.ErrExist
	}

	// Create collection using SQL INSERT
	// Pattern: similar to statementbuilder.Squirrel.Insert
	u := uuid.New()
	referenceId := u[:] // Convert to binary (16 bytes)
	query, args, err := statementbuilder.Squirrel.Insert("collection").
		Prepared(true).
		Cols("reference_id", "name", "description", "user_account_id", "permission").
		Vals([]interface{}{referenceId, collectionName, "", dcfs.sessionUser.UserId, auth.DEFAULT_PERMISSION}).
		ToSQL()

	if err != nil {
		return err
	}

	_, err = transaction.Exec(query, args...)
	return err
}

// Copy copies a file
func (dcfs *DaptinCaldavFileSystem) Copy(src, dst string, recursive, overwrite bool) (bool, error) {
	// Read source
	srcFile, err := dcfs.Open(src)
	if err != nil {
		return false, err
	}
	defer srcFile.Close()

	content, err := io.ReadAll(srcFile)
	if err != nil {
		return false, err
	}

	// Write destination
	dstFile, err := dcfs.Create(dst)
	if err != nil {
		return false, err
	}
	defer dstFile.Close()

	_, err = dstFile.Write(content)
	return true, err
}

// MoveAll moves/renames a file
func (dcfs *DaptinCaldavFileSystem) MoveAll(src, dst string, overwrite bool) (bool, error) {
	created, err := dcfs.Copy(src, dst, false, overwrite)
	if err != nil {
		return created, err
	}

	err = dcfs.RemoveAll(src)
	return created, err
}

// caldavFileWriter implements io.WriteCloser for writing files
type caldavFileWriter struct {
	fs             *DaptinCaldavFileSystem
	collectionName string
	resourceName   string
	buffer         []byte
}

func (w *caldavFileWriter) Write(p []byte) (int, error) {
	w.buffer = append(w.buffer, p...)
	return len(p), nil
}

func (w *caldavFileWriter) Close() error {
	// Write to database using ORM
	transaction, err := w.fs.cruds["calendar"].Connection().Beginx()
	if err != nil {
		return err
	}
	defer transaction.Commit()

	// Get collection_id
	collections, err := w.fs.cruds["collection"].GetAllObjectsWithWhereWithTransaction(
		"collection",
		transaction,
		goqu.Ex{
			"name":            w.collectionName,
			"user_account_id": w.fs.sessionUser.UserId,
		},
	)

	if err != nil || len(collections) == 0 {
		return os.ErrNotExist
	}

	collectionID := collections[0]["id"].(int64)
	rpath := path.Join("/calendars", w.collectionName, w.resourceName)

	// Check if file already exists (update vs create) using ORM
	existing, err := w.fs.cruds["calendar"].GetAllObjectsWithWhereWithTransaction(
		"calendar",
		transaction,
		goqu.Ex{"rpath": rpath},
	)

	if err != nil {
		return err
	}

	if len(existing) > 0 {
		// Update existing using SQL UPDATE
		// Note: ORM doesn't handle file columns in updates well, use SQL directly
		existingID := existing[0]["id"].(int64)
		// Verify the item belongs to this collection (defense in depth)
		query, args, err := statementbuilder.Squirrel.Update("calendar").
			Prepared(true).
			Where(goqu.Ex{
				"id":              existingID,
				"collection_id":   collectionID,
				"user_account_id": w.fs.sessionUser.UserId,
			}).
			Set(goqu.Record{"content": w.buffer}).
			ToSQL()

		if err != nil {
			return err
		}

		_, err = transaction.Exec(query, args...)
		return err
	} else {
		// Create new using SQL INSERT
		// Note: ORM doesn't handle file columns in inserts well, use SQL directly
		u := uuid.New()
		referenceId := u[:] // Convert to binary (16 bytes)
		query, args, err := statementbuilder.Squirrel.Insert("calendar").
			Prepared(true).
			Cols("reference_id", "rpath", "content", "collection_id", "user_account_id", "permission").
			Vals([]any{referenceId, rpath, w.buffer, collectionID, w.fs.sessionUser.UserId, auth.DEFAULT_PERMISSION}).
			ToSQL()

		if err != nil {
			return err
		}

		_, err = transaction.Exec(query, args...)
		return err
	}
}

// caldavFile implements io.ReadCloser for reading
type caldavFile struct {
	content []byte
	offset  int64
}

func (f *caldavFile) Read(p []byte) (int, error) {
	if f.offset >= int64(len(f.content)) {
		return 0, io.EOF
	}

	n := copy(p, f.content[f.offset:])
	f.offset += int64(n)
	return n, nil
}

func (f *caldavFile) Close() error {
	return nil
}
