package server

import (
	"crypto/md5"
	"fmt"
	"github.com/anthonynsimon/bild/blur"
	"github.com/anthonynsimon/bild/effect"
	"github.com/artpar/api2go"
	"github.com/daptin/daptin/server/resource"
	"github.com/disintegration/gift"
	"github.com/gin-gonic/gin"
	"image"
	"image/jpeg"
	"image/png"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
)

func Etag(content []byte) (string, error) {
	// calculate etag
	var err error
	err = nil
	hash := md5.New()
	_, err = hash.Write(content)
	if err != nil && err != io.EOF {
		return "", err
	}

	// put inside kv
	var etagFormat = "\"%x\""
	return fmt.Sprintf(etagFormat, hash.Sum(nil)), nil
}

func CreateDbAssetHandler(cruds map[string]*resource.DbResource) func(*gin.Context) {
	return func(c *gin.Context) {
		var typeName = c.Param("typename")
		var resourceId = c.Param("resource_id")
		var columnNameWithExtension = c.Param("columnname")
		//var extension = c.Param("ext")

		var parts = strings.Split(columnNameWithExtension, ".")
		columnName := parts[0]
		//extension := parts[1]

		table, ok := cruds[typeName]

		if !ok || table == nil {
			c.AbortWithStatus(404)
			return
		}

		colInfo, ok := table.TableInfo().GetColumnByName(columnName)

		if !ok || colInfo == nil || (!colInfo.IsForeignKey && colInfo.ColumnType != "markdown") {
			c.AbortWithStatus(404)
			return
		}

		pr := &http.Request{
			Method: "GET",
		}

		pr = pr.WithContext(c.Request.Context())

		req := api2go.Request{
			PlainRequest: pr,
		}

		obj, err := cruds[typeName].FindOne(resourceId, req)
		if err != nil {
			c.AbortWithStatus(500)
			return
		}

		row := obj.Result().(*api2go.Api2GoModel)
		colData := row.Data[columnName]
		if colData == nil {
			c.AbortWithStatus(404)
			return
		}

		if colInfo.IsForeignKey {

			colType := strings.Split(colInfo.ColumnType, ".")[0]
			//var filesData []interface{}
			//err = json.Unmarshal([]byte(colData.(string)), &filesData)
			//resource.CheckErr(err, "Failed to unmarshal file metadata")

			fileToServe := ""
			for _, fileData := range colData.([]map[string]interface{}) {
				//fileData := fileInterface.(map[string]interface{})
				fileName := fileData["name"].(string)
				if c.Query(fileName) == fileName || fileToServe == "" {
					fileToServe = fileName
				}
			}

			file, err := cruds["world"].AssetFolderCache[typeName][columnName].GetFileByName(fileToServe)

			if err != nil {
				c.AbortWithStatus(404)
				return
			}

			switch colType {

			case "image":

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
				log.Printf("Image format name: %v", formatName)
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

				c.AbortWithStatus(200)

			default:
				c.File(cruds["world"].AssetFolderCache[typeName][columnName].LocalSyncPath + string(os.PathSeparator) + fileToServe)

			}
		} else if colInfo.ColumnType == "markdown" {

			c.Writer.Header().Set("Content-Type", "text/html")
			c.Writer.Write([]byte("<pre>" + colData.(string) + "</pre>"))
			c.AbortWithStatus(200)

		}

	}
}
