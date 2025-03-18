package server

import (
	"crypto/md5"
	"fmt"
	"github.com/anthonynsimon/bild/blur"
	"github.com/anthonynsimon/bild/effect"
	"github.com/artpar/api2go"
	"github.com/daptin/daptin/server/resource"
	"github.com/disintegration/gift"
	"image"
	"image/jpeg"
	"image/png"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang/groupcache/lru"
	log "github.com/sirupsen/logrus"
)

const (
	// Cache settings
	MaxCacheSize     = 1000    // Maximum number of files to cache
	MaxFileCacheSize = 5 << 20 // 5MB max file size for caching
)

// FileCache implements a simple file caching system
type FileCache struct {
	cache      *lru.Cache
	cacheMutex sync.RWMutex
}

// CachedFile represents a cached file with its metadata
type CachedFile struct {
	Data     []byte
	ETag     string
	Modtime  time.Time
	MimeType string
	Size     int
}

// NewFileCache creates a new file cache
func NewFileCache(maxEntries int) *FileCache {
	return &FileCache{
		cache: lru.New(maxEntries),
	}
}

// Get retrieves a file from cache if it exists
func (fc *FileCache) Get(key string) (*CachedFile, bool) {
	fc.cacheMutex.RLock()
	defer fc.cacheMutex.RUnlock()

	if val, ok := fc.cache.Get(key); ok {
		return val.(*CachedFile), true
	}
	return nil, false
}

// Set adds a file to the cache
func (fc *FileCache) Set(key string, file *CachedFile) {
	fc.cacheMutex.Lock()
	defer fc.cacheMutex.Unlock()

	fc.cache.Add(key, file)
}

// Generates ETag for content
func generateETag(content []byte) string {
	hash := md5.New()
	hash.Write(content)
	return fmt.Sprintf("\"%x\"", hash.Sum(nil))
}

// GetMimeType determines the MIME type based on file extension
func GetMimeType(filename string) string {
	ext := strings.ToLower(filepath.Ext(filename))

	switch ext {
	case ".png":
		return "image/png"
	case ".jpeg", ".jpg":
		return "image/jpeg"
	case ".gif":
		return "image/gif"
	case ".html", ".htm":
		return "text/html; charset=utf-8"
	case ".css":
		return "text/css; charset=utf-8"
	case ".js":
		return "application/javascript"
	case ".json":
		return "application/json"
	case ".pdf":
		return "application/pdf"
	case ".txt":
		return "text/plain; charset=utf-8"
	case ".xml":
		return "application/xml"
	default:
		return "application/octet-stream"
	}
}

// Global file cache
var fileCache = NewFileCache(MaxCacheSize)

// OptimizedFileHandler is a high-performance static file handler
func OptimizedFileHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		filepath := c.Param("filepath")

		// Sanitize filepath to prevent path traversal
		if strings.Contains(filepath, "..") {
			c.AbortWithStatus(http.StatusForbidden)
			return
		}

		// Check if file is in cache
		cachedFile, found := fileCache.Get(filepath)

		// Get file info for comparison
		fileInfo, err := os.Stat(filepath)
		if err != nil {
			c.AbortWithStatus(http.StatusNotFound)
			return
		}

		// If file exists in cache and hasn't been modified, use cached version
		if found && cachedFile.Modtime.Equal(fileInfo.ModTime()) {
			// Check if client has fresh copy (If-None-Match header)
			if etag := c.GetHeader("If-None-Match"); etag != "" && etag == cachedFile.ETag {
				c.AbortWithStatus(http.StatusNotModified)
				return
			}

			// Set headers and return cached file
			c.Header("Content-Type", cachedFile.MimeType)
			c.Header("ETag", cachedFile.ETag)
			c.Header("Cache-Control", "public, max-age=86400") // 1 day cache
			c.Data(http.StatusOK, cachedFile.MimeType, cachedFile.Data)
			return
		}

		// File not in cache or modified, need to read from disk
		if fileInfo.Size() <= MaxFileCacheSize {
			// Small enough to cache
			file, err := os.Open(filepath)
			if err != nil {
				c.AbortWithStatus(http.StatusInternalServerError)
				return
			}
			defer file.Close()

			data, err := io.ReadAll(file)
			if err != nil {
				c.AbortWithStatus(http.StatusInternalServerError)
				return
			}

			// Create new cached file
			mimeType := GetMimeType(filepath)
			etag := generateETag(data)

			newCachedFile := &CachedFile{
				Data:     data,
				ETag:     etag,
				Modtime:  fileInfo.ModTime(),
				MimeType: mimeType,
				Size:     len(data),
			}

			// Add to cache
			fileCache.Set(filepath, newCachedFile)

			// Set headers and return file
			c.Header("Content-Type", mimeType)
			c.Header("ETag", etag)
			c.Header("Cache-Control", "public, max-age=86400") // 1 day cache
			c.Data(http.StatusOK, mimeType, data)
			return
		}

		// For large files, use http.ServeContent for efficient range requests
		file, err := os.Open(filepath)
		if err != nil {
			c.AbortWithStatus(http.StatusInternalServerError)
			return
		}
		defer file.Close()

		mimeType := GetMimeType(filepath)
		c.Header("Content-Type", mimeType)
		c.Header("Cache-Control", "public, max-age=86400") // 1 day cache

		http.ServeContent(c.Writer, c.Request, filepath, fileInfo.ModTime(), file)
	}
}

// CreateDbAssetHandler optimized for static file serving
func CreateDbAssetHandler(cruds map[string]*resource.DbResource) func(*gin.Context) {
	// Pre-allocate common response headers
	const cacheControl = "public, max-age=86400" // 1 day cache

	return func(c *gin.Context) {
		typeName := c.Param("typename")
		resourceUuid := c.Param("resource_id")
		columnNameWithExt := c.Param("columnname")

		parts := strings.SplitN(columnNameWithExt, ".", 2)
		if len(parts) == 0 {
			c.AbortWithStatus(http.StatusBadRequest)
			return
		}
		columnName := parts[0]

		// Fast path: check if the table exists
		table, ok := cruds[typeName]
		if !ok || table == nil {
			log.Errorf("table not found [%v]", typeName)
			c.AbortWithStatus(http.StatusNotFound)
			return
		}

		// Fast path: check if the column exists
		colInfo, ok := table.TableInfo().GetColumnByName(columnName)
		if !ok || colInfo == nil || (!colInfo.IsForeignKey && colInfo.ColumnType != "markdown") {
			log.Errorf("column [%v] info not found", columnName)
			c.AbortWithStatus(http.StatusNotFound)
			return
		}

		// Handle markdown directly (simple case)
		if colInfo.ColumnType == "markdown" {
			// Fetch data
			pr := &http.Request{
				Method: "GET",
				URL:    c.Request.URL,
			}
			pr = pr.WithContext(c.Request.Context())

			req := api2go.Request{
				PlainRequest: pr,
			}

			obj, err := cruds[typeName].FindOne(resourceUuid, req)
			if err != nil {
				c.AbortWithStatus(http.StatusInternalServerError)
				return
			}

			row := obj.Result().(api2go.Api2GoModel)
			colData := row.GetAttributes()[columnName]
			if colData == nil {
				c.AbortWithStatus(http.StatusNotFound)
				return
			}

			// Return markdown as HTML
			c.Header("Content-Type", "text/html")
			c.Header("Cache-Control", cacheControl)
			c.String(http.StatusOK, "<pre>%s</pre>", colData.(string))
			return
		}

		// Handle foreign key (file data)
		if colInfo.IsForeignKey {
			// Get cache for this path
			assetCache, ok := cruds["world"].AssetFolderCache[typeName][columnName]
			if !ok {
				c.AbortWithStatus(http.StatusNotFound)
				return
			}

			// Find the file to serve
			pr := &http.Request{
				Method: "GET",
				URL:    c.Request.URL,
			}
			pr = pr.WithContext(c.Request.Context())

			req := api2go.Request{
				PlainRequest: pr,
			}

			obj, err := cruds[typeName].FindOne(resourceUuid, req)
			if err != nil {
				c.AbortWithStatus(http.StatusInternalServerError)
				return
			}

			row := obj.Result().(api2go.Api2GoModel)
			colData := row.GetAttributes()[columnName]
			if colData == nil {
				c.AbortWithStatus(http.StatusNotFound)
				return
			}

			// Find the correct file
			fileNameToServe := ""
			fileType := "application/octet-stream"

			for _, fileData := range colData.([]map[string]interface{}) {
				fileName := fileData["name"].(string)
				queryFile := c.Query("file")

				if queryFile == fileName || queryFile == "" {
					// Determine filename
					if fileData["path"] != nil && len(fileData["path"].(string)) > 0 {
						fileNameToServe = fileData["path"].(string) + "/" + fileName
					} else {
						fileNameToServe = fileName
					}

					// Determine mime type
					if typFromData, ok := fileData["type"]; ok {
						if typeStr, isStr := typFromData.(string); isStr {
							fileType = typeStr
						} else {
							fileType = GetMimeType(fileNameToServe)
						}
					} else {
						fileType = GetMimeType(fileNameToServe)
					}

					break
				}
			}

			if fileNameToServe == "" {
				c.AbortWithStatus(http.StatusNotFound)
				return
			}

			// Get file path
			filePath := assetCache.LocalSyncPath + string(os.PathSeparator) + fileNameToServe

			// Check if it's an image that needs processing
			if isImage := strings.HasPrefix(fileType, "image/"); isImage && c.Query("processImage") == "true" {
				// Use separate function for image processing to keep this path fast
				file, err := cruds["world"].AssetFolderCache[typeName][columnName].GetFileByName(fileNameToServe)
				if err != nil {
					c.AbortWithError(500, err)
					return
				}
				defer file.Close()
				handleImageProcessing(c, file)
				return
			}

			// Check if client already has this file (via ETag)
			_, err = os.Stat(filePath)
			if err != nil {
				c.AbortWithStatus(http.StatusNotFound)
				return
			}

			// Set response headers
			c.Header("Content-Type", fileType)
			c.Header("Cache-Control", cacheControl)

			// For downloads, add content disposition
			if strings.HasPrefix(fileType, "application/") {
				c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=\"%v\"", fileNameToServe))
			}

			// Serve file with proper range support
			http.ServeFile(c.Writer, c.Request, filePath)
		}
	}
}

func handleImageProcessing(c *gin.Context, file *os.File) {

	bildFilters := make([]func(image.Image) image.Image, 0)
	filters := make([]gift.Filter, 0)
	for key, values := range c.Request.URL.Query() {
		param := gin.Param{
			Key:   key,
			Value: values[0],
		}

		valueFloat64, floatError := strconv.ParseFloat(param.Value, 32)
		valueFloat32 := float32(valueFloat64)

		switch param.Key {

		case "boxblur":
			bildFilters = append(bildFilters, func(radius float64) func(img image.Image) image.Image {
				return func(img image.Image) image.Image {
					return blur.Box(img, radius)
				}
			}(valueFloat64))
		case "gaussianblur":
			bildFilters = append(bildFilters, func(radius float64) func(img image.Image) image.Image {
				return func(img image.Image) image.Image {
					return blur.Gaussian(img, radius)
				}
			}(valueFloat64))
		case "dilate":
			bildFilters = append(bildFilters, func(radius float64) func(img image.Image) image.Image {
				return func(img image.Image) image.Image {
					return effect.Dilate(img, radius)
				}
			}(valueFloat64))
		case "edgedetection":
			bildFilters = append(bildFilters, func(radius float64) func(img image.Image) image.Image {
				return func(img image.Image) image.Image {
					return effect.EdgeDetection(img, radius)
				}
			}(valueFloat64))
		case "erode":
			bildFilters = append(bildFilters, func(radius float64) func(img image.Image) image.Image {
				return func(img image.Image) image.Image {
					return effect.Erode(img, radius)
				}
			}(valueFloat64))
		case "emboss":
			bildFilters = append(bildFilters, func() func(img image.Image) image.Image {
				return func(img image.Image) image.Image {
					return effect.Emboss(img)
				}
			}())

		case "median":
			bildFilters = append(bildFilters, func(radius float64) func(img image.Image) image.Image {
				return func(img image.Image) image.Image {
					return effect.Median(img, radius)
				}
			}(valueFloat64))

		case "sharpen":
			bildFilters = append(bildFilters, func(radius float64) func(img image.Image) image.Image {
				return func(img image.Image) image.Image {
					return effect.Sharpen(img)
				}
			}(valueFloat64))

		case "brightness":
			filters = append(filters, gift.Brightness(valueFloat32))
			break
		case "colorBalance":
			vals := strings.Split(param.Value, ",")
			if len(vals) != 3 {
				continue
			}

			red, _ := strconv.ParseFloat(vals[0], 32)
			green, _ := strconv.ParseFloat(vals[1], 32)
			blue, _ := strconv.ParseFloat(vals[2], 32)
			filters = append(filters, gift.ColorBalance(float32(red), float32(green), float32(blue)))
			break
		case "colorize":

			vals := strings.Split(param.Value, ",")
			if len(vals) != 3 {
				continue
			}

			hue, _ := strconv.ParseFloat(vals[0], 32)
			saturattion, _ := strconv.ParseFloat(vals[1], 32)
			percent, _ := strconv.ParseFloat(vals[2], 32)
			filters = append(filters, gift.ColorBalance(float32(hue), float32(saturattion), float32(percent)))
			break
		case "colorspaceLinearToSRGB":
			if strings.ToLower(param.Value) == "true" || param.Value == "1" {
				filters = append(filters, gift.ColorspaceLinearToSRGB())
			}
			break
		case "colorspaceSRGBToLinear":
			if strings.ToLower(param.Value) == "true" || param.Value == "1" {
				filters = append(filters, gift.ColorspaceSRGBToLinear())
			}
			break
		case "contrast":
			if floatError == nil {
				filters = append(filters, gift.Contrast(valueFloat32))
			}
			break
		case "crop":

			vals := strings.Split(param.Value, ",")
			if len(vals) != 4 {
				continue
			}

			minX, _ := strconv.ParseInt(vals[0], 10, 32)
			minY, _ := strconv.ParseInt(vals[1], 10, 32)
			maxX, _ := strconv.ParseInt(vals[2], 10, 32)
			maxY, _ := strconv.ParseInt(vals[3], 10, 32)

			rect := image.Rectangle{
				Min: image.Point{
					X: int(minX),
					Y: int(minY),
				},
				Max: image.Point{
					X: int(maxX),
					Y: int(maxY),
				},
			}
			filters = append(filters, gift.Crop(rect))
			break
		case "cropToSize":

			vals := strings.Split(param.Value, ",")
			if len(vals) != 3 {
				continue
			}
			height, _ := strconv.ParseInt(vals[0], 10, 32)
			weight, _ := strconv.ParseInt(vals[1], 10, 32)
			anchor := gift.CenterAnchor

			switch vals[2] {
			case "Center":
				anchor = gift.CenterAnchor
				break
			case "TopLeft":
				anchor = gift.TopLeftAnchor
				break
			case "Top":
				anchor = gift.TopAnchor
				break
			case "TopRight":
				anchor = gift.TopRightAnchor
				break
			case "Left":
				anchor = gift.LeftAnchor
				break
			case "Right":
				anchor = gift.RightAnchor
				break
			case "BottomLeft":
				anchor = gift.BottomLeftAnchor
				break
			case "Bottom":
				anchor = gift.BottomAnchor
				break
			case "BottomRight":
				anchor = gift.BottomRightAnchor
				break
			}
			filters = append(filters, gift.CropToSize(int(height), int(weight), anchor))
			break
		case "flipHorizontal":
			if strings.ToLower(param.Value) == "true" || param.Value == "1" {
				filters = append(filters, gift.FlipHorizontal())
			}
			break
		case "flipVertical":
			if strings.ToLower(param.Value) == "true" || param.Value == "1" {
				filters = append(filters, gift.FlipVertical())
			}
			break
		case "gamma":
			filters = append(filters, gift.Gamma(valueFloat32))
			break
		case "gaussianBlur":
			filters = append(filters, gift.GaussianBlur(valueFloat32))
			break
		case "grayscale":
			if strings.ToLower(param.Value) == "true" || param.Value == "1" {
				filters = append(filters, gift.Grayscale())
			}
			break
		case "hue":
			filters = append(filters, gift.Hue(valueFloat32))
			break
		case "invert":
			if strings.ToLower(param.Value) == "true" || param.Value == "1" {

				filters = append(filters, gift.Invert())
			}
			break
		case "resize":
			vals := strings.Split(param.Value, ",")
			if len(vals) != 3 {
				continue
			}
			height, _ := strconv.ParseInt(vals[0], 10, 32)
			weight, _ := strconv.ParseInt(vals[1], 10, 32)
			resampling := gift.NearestNeighborResampling

			switch vals[2] {
			case "NearestNeighbor":
				resampling = gift.NearestNeighborResampling
				break
			case "Box":
				resampling = gift.BoxResampling
				break
			case "Linear":
				resampling = gift.LinearResampling
				break
			case "Cubic":
				resampling = gift.CubicResampling
				break
			case "Lanczos":
				resampling = gift.LanczosResampling
				break
			}
			filters = append(filters, gift.Resize(int(height), int(weight), resampling))
			break
		case "rotate":

			vals := strings.Split(param.Value, ",")
			if len(vals) != 3 {
				continue
			}
			angle, _ := strconv.ParseFloat(vals[0], 32)
			backgroundColor, _ := ParseHexColor("#" + vals[1])
			interpolation := gift.NearestNeighborInterpolation

			switch vals[2] {
			case "NearestNeighbor":
				interpolation = gift.NearestNeighborInterpolation
				break
			case "Linear":
				interpolation = gift.LinearInterpolation
				break
			case "Cubic":
				interpolation = gift.CubicInterpolation
				break
			}
			filters = append(filters, gift.Rotate(float32(angle), backgroundColor, interpolation))
			break
		case "rotate180":
			if strings.ToLower(param.Value) == "true" || param.Value == "1" {

				filters = append(filters, gift.Rotate180())
			}
			break
		case "rotate270":
			if strings.ToLower(param.Value) == "true" || param.Value == "1" {

				filters = append(filters, gift.Rotate270())
			}
			break
		case "rotate90":
			if strings.ToLower(param.Value) == "true" || param.Value == "1" {

				filters = append(filters, gift.Rotate270())
			}
			break
		case "saturation":
			filters = append(filters, gift.Saturation(valueFloat32))
			break
		case "sepia":
			filters = append(filters, gift.Sepia(valueFloat32))
			break
		case "sobel":
			if strings.ToLower(param.Value) == "true" || param.Value == "1" {

				filters = append(filters, gift.Sobel())
			}
			break
		case "threshold":
			filters = append(filters, gift.Threshold(valueFloat32))

			break
		case "transpose":
			if strings.ToLower(param.Value) == "true" || param.Value == "1" {

				filters = append(filters, gift.Transpose())
			}

			break
		case "transverse":
			if strings.ToLower(param.Value) == "true" || param.Value == "1" {

				filters = append(filters, gift.Transverse())
			}
			break

		}

	}
	f := gift.New(filters...)

	img, formatName, err := image.Decode(file)
	if err != nil {
		c.AbortWithStatus(500)
		return
	}

	for _, f := range bildFilters {

		img = f(img)
	}

	dst := image.NewNRGBA(f.Bounds(img.Bounds()))
	f.Draw(dst, img)

	c.Writer.Header().Set("Content-Type", "image/"+formatName)
	if formatName == "png" {
		err = png.Encode(c.Writer, dst)
	} else {
		err = jpeg.Encode(c.Writer, dst, nil)
	}
	if err != nil {
		log.Printf("failed to write converted image :%v", err)
	}
}
