package server

import (
	"github.com/anthonynsimon/bild/blur"
	"github.com/anthonynsimon/bild/effect"
	"github.com/bep/gowebp/libwebp/webpoptions"
	"github.com/disintegration/gift"
	"github.com/gin-gonic/gin"
	"github.com/gohugoio/hugo/resources/images/webp"
	log "github.com/sirupsen/logrus"
	"image"
	"image/jpeg"
	"image/png"
	"io"
	"math"
	"net/http"
	"os"
	"strconv"
	"strings"
)

// maxImageProcessingDimension bounds decoded images, filter outputs and resize
// scratch buffers (target width x source height). GIFT uses 8-byte pixels for
// intermediate buffers, so a final-output-only check is insufficient.
const (
	maxImageProcessingDimension = 4096
	maxImageProcessingRadius    = 32
	// Bound cumulative neighborhood visits, including morphology sorting work.
	maxImageProcessingKernelWork uint64 = 64 * 1024 * 1024
)

func imageProcessingBounds(bounds image.Rectangle, filters []gift.Filter, kernelWorkPerPixel uint64, padding int) (image.Rectangle, bool) {
	valid := func(b image.Rectangle) bool {
		return b.Dx() > 0 && b.Dy() > 0 &&
			b.Dx() <= maxImageProcessingDimension && b.Dy() <= maxImageProcessingDimension &&
			(kernelWorkPerPixel == 0 || uint64(b.Dx())*uint64(b.Dy()) <= maxImageProcessingKernelWork/kernelWorkPerPixel)
	}
	if !valid(bounds) || bounds.Dx()+2*padding > maxImageProcessingDimension || bounds.Dy()+2*padding > maxImageProcessingDimension {
		return image.Rectangle{}, false
	}
	for _, filter := range filters {
		bounds = filter.Bounds(bounds)
		if !valid(bounds) {
			return image.Rectangle{}, false
		}
	}
	return bounds, true
}

func HandleImageProcessing(c *gin.Context, file *os.File) {

	bildFilters := make([]func(image.Image) image.Image, 0)
	filters := make([]gift.Filter, 0)
	var kernelWorkPerPixel uint64
	maxPadding := 0
	for key, values := range c.Request.URL.Query() {
		param := gin.Param{
			Key:   key,
			Value: values[0],
		}

		valueFloat64, floatError := strconv.ParseFloat(param.Value, 32)
		valueFloat32 := float32(valueFloat64)

		// Validate before constructing filters. These libraries allocate kernels
		// and padded images using the caller's radius, independent of output size.
		switch param.Key {
		case "boxblur", "gaussianblur", "edgedetection", "dilate", "erode", "median", "gaussianBlur":
			valueFloat64, floatError = strconv.ParseFloat(param.Value, 64)
			if floatError != nil || math.IsNaN(valueFloat64) || math.IsInf(valueFloat64, 0) ||
				valueFloat64 < 0 || valueFloat64 > maxImageProcessingRadius {
				c.AbortWithStatus(http.StatusBadRequest)
				return
			}
			valueFloat32 = float32(valueFloat64)
			if valueFloat64 > 0 {
				kernelSize := int(math.Ceil(2*valueFloat64 + 1))
				padding := kernelSize / 2
				work := uint64(kernelSize * kernelSize)
				switch param.Key {
				case "dilate", "erode", "median":
					padding = int(valueFloat64 + 0.5)
					kernelSize = int(2*valueFloat64 + 1 + 0.5)
					// BILD rounds padding and kernels differently for some
					// fractional radii, which would panic in its workers.
					if padding < kernelSize/2 {
						c.AbortWithStatus(http.StatusBadRequest)
						return
					}
					work = uint64(kernelSize * kernelSize)
					// All three BILD morphology filters sort every window
					// with quicksort; account for its quadratic worst case.
					work *= work
				case "gaussianBlur":
					// Match GIFT's float32 radius calculation and two passes.
					kernelSize = 2*int(math.Ceil(float64(valueFloat32*3))) + 1
					work = uint64(2 * kernelSize)
					padding = 0
				}
				kernelWorkPerPixel += work
				if padding > maxPadding {
					maxPadding = padding
				}
			}
		}

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
				c.AbortWithStatus(http.StatusBadRequest)
				return
			}
			width, widthErr := strconv.ParseInt(vals[0], 10, 32)
			height, heightErr := strconv.ParseInt(vals[1], 10, 32)
			if widthErr != nil || heightErr != nil || width < 0 || height < 0 ||
				(width == 0 && height == 0) || width > maxImageProcessingDimension || height > maxImageProcessingDimension {
				c.AbortWithStatus(http.StatusBadRequest)
				return
			}
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
			filters = append(filters, gift.Resize(int(width), int(height), resampling))
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
	// Inspect dimensions and every intermediate before decoding pixels or running
	// either filter library. BILD filters preserve the source dimensions. Charge
	// all radius filters at every stage's size, conservatively bounding work even
	// when GIFT runs after a resize; a later crop must not hide an expensive stage.
	config, _, err := image.DecodeConfig(file)
	if err != nil {
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}
	sourceBounds := image.Rect(0, 0, config.Width, config.Height)
	bounds, ok := imageProcessingBounds(sourceBounds, filters, kernelWorkPerPixel, maxPadding)
	if !ok {
		c.AbortWithStatus(http.StatusBadRequest)
		return
	}
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}

	img, formatName, err := image.Decode(file)
	if err != nil {
		c.AbortWithStatus(500)
		return
	}

	// Some formats decode a frame whose bounds differ from the canvas metadata.
	if img.Bounds() != sourceBounds {
		bounds, ok = imageProcessingBounds(img.Bounds(), filters, kernelWorkPerPixel, maxPadding)
		if !ok {
			c.AbortWithStatus(http.StatusBadRequest)
			return
		}
	}

	for _, f := range bildFilters {

		img = f(img)
	}

	dst := image.NewNRGBA(bounds)
	gift.New(filters...).Draw(dst, img)

	c.Writer.Header().Set("Content-Type", "image/"+formatName)
	if formatName == "png" {
		err = png.Encode(c.Writer, dst)
	} else if formatName == "webp" {
		encodingOptions := webpoptions.EncodingOptions{
			Quality:        10,
			EncodingPreset: 0,
			UseSharpYuv:    false,
		}
		err = webp.Encode(c.Writer, dst, encodingOptions)
	} else {
		err = jpeg.Encode(c.Writer, dst, nil)
	}
	if err != nil {
		log.Errorf("failed to write converted image :%v", err)
	}
}
