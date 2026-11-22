package server

import (
	"bytes"
	"encoding/binary"
	"hash/crc32"
	"image"
	"image/color"
	"image/gif"
	"image/jpeg"
	"image/png"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/disintegration/gift"
	"github.com/gin-gonic/gin"
)

func imageProcessingPNG(t *testing.T, width, height int) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := png.Encode(&buf, image.NewNRGBA(image.Rect(0, 0, width, height))); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// An oversized IHDR with a valid CRC proves rejection before pixel decoding:
// the IDAT still describes only the original tiny image.
func imageProcessingPNGHeader(t *testing.T, width, height uint32) []byte {
	t.Helper()
	data := imageProcessingPNG(t, 1, 1)
	binary.BigEndian.PutUint32(data[16:20], width)
	binary.BigEndian.PutUint32(data[20:24], height)
	binary.BigEndian.PutUint32(data[29:33], crc32.ChecksumIEEE(data[12:29]))
	return data
}

func processImageForTest(t *testing.T, data []byte, query string) *httptest.ResponseRecorder {
	t.Helper()
	path := filepath.Join(t.TempDir(), "image")
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	file, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/?processImage=true&"+query, nil)
	HandleImageProcessing(ctx, file)
	return recorder
}

func TestImageProcessingRejectsUnsafeDimensions(t *testing.T) {
	tiny := imageProcessingPNG(t, 1, 2)
	for _, query := range []string{
		"resize=30000,30000,Box", "resize=4097,1,Box", "resize=1,4097,Box",
		"resize=2147483648,1,Box", "resize=999999999999999999999999,1,Box",
		"resize=-1,1,Box", "resize=1,-1,Box", "resize=0,0,Box",
		"resize=NaN,1,Box", "resize=Inf,1,Box", "resize=1.5,1,Box",
		"resize=oops,1,Box", "resize=1,1", "resize=", "resize=1,,Box",
		"resize=4096,0,Box", // inferred height is 8192
		"resize=30000,30000,Box&cropToSize=1,1,Center",
	} {
		t.Run(query, func(t *testing.T) {
			if response := processImageForTest(t, tiny, query); response.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400", response.Code)
			}
		})
	}
	// Explicit invalid dimensions must be rejected even before reading the image.
	if response := processImageForTest(t, nil, "resize=30000,30000,Box"); response.Code != http.StatusBadRequest {
		t.Fatalf("invalid resize reached decoding: status = %d", response.Code)
	}
	for _, size := range [][2]uint32{{4097, 1}, {1, 4097}, {30000, 30000}} {
		if response := processImageForTest(t, imageProcessingPNGHeader(t, size[0], size[1]), "resize=1,1,Box"); response.Code != http.StatusBadRequest {
			t.Fatalf("source %v: status = %d, want 400 before decoding", size, response.Code)
		}
	}
	if response := processImageForTest(t, imageProcessingPNGHeader(t, 3000, 3000), "rotate=45,000000,Linear"); response.Code != http.StatusBadRequest {
		t.Fatalf("expanding rotation: status = %d, want 400 before decoding", response.Code)
	}
}

func TestImageProcessingValidDimensions(t *testing.T) {
	for _, method := range []string{"NearestNeighbor", "Box", "Linear", "Cubic", "Lanczos"} {
		t.Run(method, func(t *testing.T) {
			response := processImageForTest(t, imageProcessingPNG(t, 2, 4), "resize=3,5,"+method)
			assertProcessedImage(t, response, 3, 5)
		})
	}
	for _, tc := range []struct {
		query         string
		width, height int
	}{
		{"resize=4096,1,Box", 4096, 1},
		{"resize=1,4096,Box", 1, 4096},
		{"resize=3,0,Box", 3, 6},
		{"resize=0,3,Box", 2, 3},
		{"grayscale=true&cropToSize=1,1,Center", 1, 1},
		{"", 2, 4},
	} {
		t.Run(tc.query, func(t *testing.T) {
			assertProcessedImage(t, processImageForTest(t, imageProcessingPNG(t, 2, 4), tc.query), tc.width, tc.height)
		})
	}
	assertProcessedImage(t, processImageForTest(t, imageProcessingPNG(t, 4096, 1), ""), 4096, 1)

	var jpegData bytes.Buffer
	if err := jpeg.Encode(&jpegData, image.NewRGBA(image.Rect(0, 0, 2, 4)), nil); err != nil {
		t.Fatal(err)
	}
	assertProcessedImage(t, processImageForTest(t, jpegData.Bytes(), "resize=3,5,Box"), 3, 5)

	// GIF canvas metadata may be larger than the decoded first frame.
	var gifData bytes.Buffer
	frame := image.NewPaletted(image.Rect(1, 1, 3, 4), color.Palette{color.Black, color.White})
	if err := gif.EncodeAll(&gifData, &gif.GIF{Image: []*image.Paletted{frame}, Delay: []int{0}, Config: image.Config{Width: 4, Height: 5}}); err != nil {
		t.Fatal(err)
	}
	assertProcessedImage(t, processImageForTest(t, gifData.Bytes(), "resize=0,6,Box"), 4, 6)
}

func assertProcessedImage(t *testing.T, response *httptest.ResponseRecorder, width, height int) {
	t.Helper()
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", response.Code)
	}
	config, _, err := image.DecodeConfig(bytes.NewReader(response.Body.Bytes()))
	if err != nil {
		t.Fatal(err)
	}
	if config.Width != width || config.Height != height {
		t.Fatalf("output = %dx%d, want %dx%d", config.Width, config.Height, width, height)
	}
}

func TestImageProcessingBoundsRejectsIntermediateExpansion(t *testing.T) {
	for _, filters := range [][]gift.Filter{
		{gift.Resize(30000, 30000, gift.BoxResampling), gift.CropToSize(1, 1, gift.CenterAnchor)},
		{gift.Resize(4096, 4096, gift.BoxResampling), gift.Rotate(45, color.Black, gift.LinearInterpolation), gift.CropToSize(1, 1, gift.CenterAnchor)},
	} {
		if final := gift.New(filters...).Bounds(image.Rect(0, 0, 1, 1)); final.Dx() != 1 || final.Dy() != 1 {
			t.Fatalf("test requires a small final image, got %v", final)
		}
		if _, ok := imageProcessingBounds(image.Rect(0, 0, 1, 1), filters, 0, 0); ok {
			t.Fatal("accepted oversized intermediate with small final output")
		}
	}
	if _, ok := imageProcessingBounds(image.Rect(0, 0, 4096, 4096), nil, 0, 0); !ok {
		t.Fatal("rejected exact dimension limit")
	}
}

func TestImageProcessingRejectsUnsafeKernelParameters(t *testing.T) {
	for _, filter := range []string{"boxblur", "gaussianblur", "edgedetection", "dilate", "erode", "median", "gaussianBlur"} {
		for _, value := range []string{"30000", "1000000000", "32.00000001", "1e100", "1e309", "NaN", "%2BInf", "-Inf", "-1", "", "oops"} {
			t.Run(filter+"="+value, func(t *testing.T) {
				// An empty source ensures invalid parameters fail before decoding.
				response := processImageForTest(t, nil, filter+"="+value)
				if response.Code != http.StatusBadRequest {
					t.Fatalf("status = %d, want 400 before decoding", response.Code)
				}
			})
		}
	}
	for _, filter := range []string{"dilate", "erode", "median"} {
		for _, value := range []string{"0.25", "0.24999999999999997", "1.25", "31.25"} {
			if response := processImageForTest(t, nil, filter+"="+value); response.Code != http.StatusBadRequest {
				t.Fatalf("%s=%s: status = %d, want 400 for insufficient kernel padding", filter, value, response.Code)
			}
		}
	}
}

func TestImageProcessingValidKernelParameters(t *testing.T) {
	data := imageProcessingPNG(t, 2, 4)
	for _, filter := range []string{"boxblur", "gaussianblur", "edgedetection", "dilate", "erode", "median", "gaussianBlur"} {
		for _, value := range []string{"0", "0.5", "1"} {
			t.Run(filter+"="+value, func(t *testing.T) {
				assertProcessedImage(t, processImageForTest(t, data, filter+"="+value), 2, 4)
			})
		}
	}
	for _, filter := range []string{"boxblur", "gaussianblur", "edgedetection", "dilate", "erode", "median", "gaussianBlur"} {
		assertProcessedImage(t, processImageForTest(t, imageProcessingPNG(t, 1, 1), filter+"=32"), 1, 1)
	}
	assertProcessedImage(t, processImageForTest(t, data, "boxblur=0.5&gaussianBlur=1&resize=3,5,Box"), 3, 5)
	// Zero radius needs no padding and must remain usable at the image-size limit.
	assertProcessedImage(t, processImageForTest(t, imageProcessingPNG(t, 4096, 1), "dilate=0"), 4096, 1)
}

func TestImageProcessingRejectsExcessiveKernelWorkAndPadding(t *testing.T) {
	for _, tc := range []struct {
		width, height uint32
		query         string
	}{
		{4096, 1, "boxblur=0.1"},
		{4096, 1, "dilate=1"},
		{1024, 1024, "boxblur=4"},
		{1024, 1024, "boxblur=4&cropToSize=1,1,Center"},
		// Each radius-2 convolution fits individually; their combined work does not.
		{1024, 1024, "boxblur=2&gaussianblur=2&edgedetection=2"},
		{1024, 1024, "median=2"},
		{1024, 1024, "dilate=2"},
		{1024, 1024, "erode=2"},
		// A GIFT blur may run after a resize, regardless of query-map order.
		{1, 2, "resize=2048,2048,Box&gaussianBlur=3"},
		{1, 2, "resize=2048,2048,Box&gaussianBlur=3&cropToSize=1,1,Center"},
	} {
		t.Run(tc.query, func(t *testing.T) {
			response := processImageForTest(t, imageProcessingPNGHeader(t, tc.width, tc.height), tc.query)
			if response.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400 before decoding", response.Code)
			}
		})
	}
	// Exercise exact budget and padding boundaries without allocating large images.
	if _, ok := imageProcessingBounds(image.Rect(0, 0, 1024, 1024), nil, 64, 0); !ok {
		t.Fatal("rejected exact work budget")
	}
	if _, ok := imageProcessingBounds(image.Rect(0, 0, 1024, 1024), nil, 65, 0); ok {
		t.Fatal("accepted excessive work")
	}
	if _, ok := imageProcessingBounds(image.Rect(0, 0, 4094, 1), nil, 1, 1); !ok {
		t.Fatal("rejected exact padding limit")
	}
	if _, ok := imageProcessingBounds(image.Rect(0, 0, 4095, 1), nil, 1, 1); ok {
		t.Fatal("accepted excessive padding")
	}
}
