package server

import (
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/artpar/api2go/v2"
	"github.com/daptin/daptin/server/auth"
	"github.com/daptin/daptin/server/resource"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

const (
	officeDriveRootHash = "d1_Lw"
	officeDriveVolumeID = "d1_"
	officeDrivePageSize = 100
	officeDriveMaxRows  = 10000
)

type officeDriveFile struct {
	Name     string `json:"name"`
	Hash     string `json:"hash"`
	Parent   string `json:"phash,omitempty"`
	MIME     string `json:"mime"`
	Modified int64  `json:"ts"`
	Read     int    `json:"read"`
	Write    int    `json:"write"`
	Locked   int    `json:"locked"`
	Dirs     int    `json:"dirs,omitempty"`
	VolumeID string `json:"volumeid,omitempty"`
}

func officeDriveRootFile() officeDriveFile {
	return officeDriveFile{
		Name: "Drive", Hash: officeDriveRootHash, MIME: "directory",
		Modified: 0, Read: 1, Write: 0, Locked: 1, VolumeID: officeDriveVolumeID,
	}
}

func officeDriveDisabledCommands() []string {
	return []string{
		"archive", "copy", "cut", "download", "duplicate", "edit", "extract",
		"get", "mkdir", "mkfile", "paste", "put", "rename", "resize", "rm",
		"upload", "zipdl",
	}
}

// RegisterOfficeDriveConnector attaches the read-only elFinder boundary to the
// router. Authentication is performed by the router's existing middleware for
// every request; the handler requires its authenticated SessionUser.
func RegisterOfficeDriveConnector(router *gin.Engine, documents *resource.DbResource) {
	handler := func(c *gin.Context) {
		if user, ok := c.Request.Context().Value("user").(*auth.SessionUser); !ok || user == nil {
			c.AbortWithStatus(http.StatusUnauthorized)
			return
		}
		if documents == nil {
			officeDriveError(c, http.StatusServiceUnavailable, "errOpen")
			return
		}
		if err := c.Request.ParseForm(); err != nil {
			officeDriveError(c, http.StatusBadRequest, "errCmdParams")
			return
		}
		commands := c.Request.Form["cmd"]
		if len(commands) != 1 || commands[0] == "" {
			officeDriveError(c, http.StatusBadRequest, "errCmdParams")
			return
		}

		var query string
		var mimes []string
		switch commands[0] {
		case "open":
			init := c.Request.FormValue("init") == "1" || c.Request.FormValue("init") == "true"
			targets := c.Request.Form["target"]
			if len(targets) > 1 || (len(targets) == 0 && !init) ||
				(len(targets) == 1 && targets[0] == "" && !init) {
				officeDriveError(c, http.StatusBadRequest, "errCmdParams")
				return
			}
			if len(targets) == 1 && targets[0] != "" {
				if code := officeDriveRootTargetError(targets[0]); code != "" {
					officeDriveError(c, http.StatusBadRequest, code)
					return
				}
			}
			files, err := officeDriveReadDocuments(c.Request, documents, "", nil)
			if err != nil {
				officeDriveReadError(c, err, "errOpen")
				return
			}
			root := officeDriveRootFile()
			files = append([]officeDriveFile{root}, files...)
			response := gin.H{
				"cwd":   root,
				"files": files,
				"options": gin.H{
					"path": "Drive", "disabled": officeDriveDisabledCommands(),
				},
			}
			if init {
				response["api"] = "2.1"
				response["uplMaxSize"] = "0"
				response["uplMaxFile"] = 0
			}
			c.JSON(http.StatusOK, response)
		case "search":
			targets := c.Request.Form["target"]
			if len(targets) > 1 {
				officeDriveError(c, http.StatusBadRequest, "errCmdParams")
				return
			}
			if len(targets) == 1 && targets[0] != "" {
				if code := officeDriveRootTargetError(targets[0]); code != "" {
					officeDriveError(c, http.StatusBadRequest, code)
					return
				}
			}
			queries := c.Request.Form["q"]
			if len(queries) != 1 {
				officeDriveError(c, http.StatusBadRequest, "errCmdParams")
				return
			}
			query = queries[0]
			mimes = c.Request.Form["mimes[]"]
			if len(mimes) == 0 {
				mimes = c.Request.Form["mimes"]
			}
			files, err := officeDriveReadDocuments(c.Request, documents, query, mimes)
			if err != nil {
				officeDriveReadError(c, err, "errSearch")
				return
			}
			c.JSON(http.StatusOK, gin.H{"files": files})
		default:
			officeDriveError(c, http.StatusBadRequest, "errUnknownCmd")
		}
	}
	router.GET("/office/drive/connector", handler)
	router.POST("/office/drive/connector", handler)
}

func officeDriveError(c *gin.Context, status int, code string) {
	c.JSON(status, gin.H{"error": code})
}

func officeDriveReadError(c *gin.Context, err error, code string) {
	var httpError interface{ Status() int }
	if errors.As(err, &httpError) && httpError.Status() == http.StatusForbidden {
		officeDriveError(c, http.StatusForbidden, "errAccess")
		return
	}
	if errors.Is(err, errOfficeDriveLimit) {
		officeDriveError(c, http.StatusRequestEntityTooLarge, "Drive contains more than 10,000 document rows to scan.")
		return
	}
	if errors.Is(err, errOfficeDriveCanceled) {
		officeDriveError(c, http.StatusRequestTimeout, code)
		return
	}
	if errors.Is(err, errOfficeDriveChanged) {
		officeDriveError(c, http.StatusConflict, code)
		return
	}
	officeDriveError(c, http.StatusInternalServerError, code)
}

var (
	errOfficeDriveLimit    = errors.New("document scan exceeds 10000 candidate rows")
	errOfficeDriveCanceled = errors.New("document scan canceled")
	errOfficeDriveChanged  = errors.New("document scan changed during pagination")
)

func officeDriveRootTargetError(hash string) string {
	if hash == officeDriveRootHash {
		return ""
	}
	// Decode non-root hashes even though this slice cannot open files.
	if _, err := officeDriveDecodeHash(hash); err != nil {
		return "errCmdParams"
	}
	return "errFileNotFound"
}

func officeDriveDecodeHash(hash string) (string, error) {
	if !strings.HasPrefix(hash, officeDriveVolumeID) {
		return "", errors.New("invalid drive volume")
	}
	encoded := strings.TrimPrefix(hash, officeDriveVolumeID)
	decoded, err := base64.RawURLEncoding.DecodeString(encoded)
	if err != nil || base64.RawURLEncoding.EncodeToString(decoded) != encoded {
		return "", errors.New("invalid drive hash")
	}
	id := string(decoded)
	parsed, err := uuid.Parse(id)
	if err != nil || parsed.String() != id {
		return "", errors.New("invalid document reference ID")
	}
	return id, nil
}

func officeDriveReadDocuments(request *http.Request, documents *resource.DbResource, query string, mimes []string) ([]officeDriveFile, error) {
	files := make([]officeDriveFile, 0)
	var initialTotal uint
	for page := 1; page <= officeDriveMaxRows/officeDrivePageSize; page++ {
		if request.Context().Err() != nil {
			return nil, errOfficeDriveCanceled
		}
		readRequest := request.Clone(request.Context())
		readRequest.Method = http.MethodGet
		readRequest.URL = &url.URL{Path: "/api/document"}
		count, response, err := documents.PaginatedFindAll(api2go.Request{
			PlainRequest: readRequest,
			QueryParams: map[string][]string{
				"page[number]": {fmt.Sprint(page)},
				"page[size]":   {fmt.Sprint(officeDrivePageSize)},
				"sort":         {"reference_id"},
				"fields":       {"reference_id,document_name,mime_type,created_at,updated_at"},
			},
		})
		if err != nil {
			return nil, err
		}
		if request.Context().Err() != nil {
			return nil, errOfficeDriveCanceled
		}
		if count > officeDriveMaxRows {
			return nil, errOfficeDriveLimit
		}
		if page == 1 {
			initialTotal = count
		} else if count != initialTotal {
			return nil, errOfficeDriveChanged
		}
		if response == nil {
			return nil, errors.New("document page unavailable")
		}
		models, ok := response.Result().([]api2go.Api2GoModel)
		if !ok || len(models) > officeDrivePageSize {
			return nil, errors.New("invalid document page")
		}
		for _, model := range models {
			file, err := officeDriveMapFile(model)
			if err != nil {
				return nil, err
			}
			if !strings.Contains(strings.ToLower(file.Name), strings.ToLower(query)) || !officeDriveMimeMatches(file.MIME, mimes) {
				continue
			}
			files = append(files, file)
		}
		// The count describes the ordered query before the AfterFindAll row
		// filter. A short visible page is not an end-of-data signal.
		if uint(page*officeDrivePageSize) >= initialTotal {
			return files, nil
		}
	}
	return nil, errOfficeDriveLimit
}

func officeDriveMimeMatches(mime string, filters []string) bool {
	if len(filters) == 0 {
		return true
	}
	mime = strings.ToLower(mime)
	for _, filter := range filters {
		if strings.HasPrefix(mime, strings.ToLower(filter)) {
			return true
		}
	}
	return false
}

func officeDriveMapFile(model api2go.Api2GoModel) (officeDriveFile, error) {
	id := model.GetID()
	parsed, err := uuid.Parse(id)
	if err != nil || parsed.String() != id {
		return officeDriveFile{}, errors.New("invalid document reference ID")
	}
	attributes := model.GetAttributes()
	name, ok := attributes["document_name"].(string)
	if !ok || name == "" {
		return officeDriveFile{}, errors.New("document name unavailable")
	}
	mime, ok := attributes["mime_type"].(string)
	if !ok || mime == "" {
		return officeDriveFile{}, errors.New("document MIME type unavailable")
	}
	modified, err := officeDriveTimestamp(attributes["updated_at"])
	if err != nil {
		modified, err = officeDriveTimestamp(attributes["created_at"])
		if err != nil {
			return officeDriveFile{}, errors.New("document timestamp unavailable")
		}
	}
	return officeDriveFile{
		Name: name, Hash: officeDriveVolumeID + base64.RawURLEncoding.EncodeToString([]byte(id)),
		Parent: officeDriveRootHash, MIME: mime, Modified: modified,
		Read: 1, Write: 0, Locked: 1,
	}, nil
}

func officeDriveTimestamp(value interface{}) (int64, error) {
	switch typed := value.(type) {
	case time.Time:
		if !typed.IsZero() {
			return typed.Unix(), nil
		}
	case string:
		parsed, err := time.Parse(time.RFC3339Nano, typed)
		if err == nil {
			return parsed.Unix(), nil
		}
	}
	return 0, errors.New("invalid document timestamp")
}
