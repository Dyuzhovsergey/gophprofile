package services

import (
	"bytes"
	"errors"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"testing"

	"github.com/Dyuzhovsergey/gophprofile/internal/domain"
)

func TestImageService_Process_JPEG(t *testing.T) {
	service := NewImageService()

	data := mustCreateJPEG(t, 800, 600)

	result, err := service.Process(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Width != 800 {
		t.Fatalf("unexpected width: got %d, want %d", result.Width, 800)
	}

	if result.Height != 600 {
		t.Fatalf("unexpected height: got %d, want %d", result.Height, 600)
	}

	assertThumbnails(t, result.Thumbnails)
}

func TestImageService_Process_PNG(t *testing.T) {
	service := NewImageService()

	data := mustCreatePNG(t, 1024, 768)

	result, err := service.Process(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Width != 1024 {
		t.Fatalf("unexpected width: got %d, want %d", result.Width, 1024)
	}

	if result.Height != 768 {
		t.Fatalf("unexpected height: got %d, want %d", result.Height, 768)
	}

	assertThumbnails(t, result.Thumbnails)
}

func TestImageService_Process_InvalidFile(t *testing.T) {
	service := NewImageService()

	_, err := service.Process([]byte("not image"))
	if !errors.Is(err, domain.ErrInvalidFile) {
		t.Fatalf("expected ErrInvalidFile, got %v", err)
	}
}

func TestImageService_Process_EmptyFile(t *testing.T) {
	service := NewImageService()

	_, err := service.Process(nil)
	if !errors.Is(err, domain.ErrInvalidFile) {
		t.Fatalf("expected ErrInvalidFile, got %v", err)
	}
}

func assertThumbnails(t *testing.T, thumbnails []ImageThumbnail) {
	t.Helper()

	if len(thumbnails) != 2 {
		t.Fatalf("unexpected thumbnails count: got %d, want %d", len(thumbnails), 2)
	}

	expected := map[domain.ThumbnailSize]int{
		domain.ThumbnailSize100: 100,
		domain.ThumbnailSize300: 300,
	}

	for _, thumbnail := range thumbnails {
		wantSize, ok := expected[thumbnail.Size]
		if !ok {
			t.Fatalf("unexpected thumbnail size: %q", thumbnail.Size)
		}

		if thumbnail.Width != wantSize {
			t.Fatalf("unexpected thumbnail width: got %d, want %d", thumbnail.Width, wantSize)
		}

		if thumbnail.Height != wantSize {
			t.Fatalf("unexpected thumbnail height: got %d, want %d", thumbnail.Height, wantSize)
		}

		if len(thumbnail.Data) == 0 {
			t.Fatal("expected thumbnail data")
		}

		if thumbnail.ContentType != "image/jpeg" {
			t.Fatalf("unexpected content type: got %q, want %q", thumbnail.ContentType, "image/jpeg")
		}

		if thumbnail.Extension != ".jpg" {
			t.Fatalf("unexpected extension: got %q, want %q", thumbnail.Extension, ".jpg")
		}

		decoded, _, err := image.Decode(bytes.NewReader(thumbnail.Data))
		if err != nil {
			t.Fatalf("failed to decode thumbnail: %v", err)
		}

		bounds := decoded.Bounds()
		if bounds.Dx() != wantSize {
			t.Fatalf("decoded thumbnail width: got %d, want %d", bounds.Dx(), wantSize)
		}

		if bounds.Dy() != wantSize {
			t.Fatalf("decoded thumbnail height: got %d, want %d", bounds.Dy(), wantSize)
		}
	}
}

func mustCreateJPEG(t *testing.T, width int, height int) []byte {
	t.Helper()

	img := createTestImage(width, height)

	var buffer bytes.Buffer
	if err := jpeg.Encode(&buffer, img, &jpeg.Options{Quality: 90}); err != nil {
		t.Fatalf("failed to encode jpeg: %v", err)
	}

	return buffer.Bytes()
}

func mustCreatePNG(t *testing.T, width int, height int) []byte {
	t.Helper()

	img := createTestImage(width, height)

	var buffer bytes.Buffer
	if err := png.Encode(&buffer, img); err != nil {
		t.Fatalf("failed to encode png: %v", err)
	}

	return buffer.Bytes()
}

func createTestImage(width int, height int) image.Image {
	img := image.NewRGBA(image.Rect(0, 0, width, height))

	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			img.Set(x, y, color.RGBA{
				R: uint8(x % 255),
				G: uint8(y % 255),
				B: 100,
				A: 255,
			})
		}
	}

	return img
}
