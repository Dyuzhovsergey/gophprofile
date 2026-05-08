package services

import (
	"bytes"
	"fmt"
	"image"
	"image/jpeg"
	"image/png"

	"github.com/Dyuzhovsergey/gophprofile/internal/domain"
	"golang.org/x/image/draw"
	_ "golang.org/x/image/webp"
)

const (
	thumbnailJPEGQuality = 90
)

// ImageService создаёт миниатюры и получает параметры изображений.
type ImageService struct{}

// NewImageService создаёт сервис обработки изображений.
func NewImageService() *ImageService {
	return &ImageService{}
}

// ImageProcessResult содержит результат обработки оригинального изображения.
type ImageProcessResult struct {
	Width      int
	Height     int
	Thumbnails []ImageThumbnail
}

// ImageThumbnail описывает созданную миниатюру.
type ImageThumbnail struct {
	Size        domain.ThumbnailSize
	Width       int
	Height      int
	Data        []byte
	ContentType string
	Extension   string
}

// Process создаёт миниатюры 100x100 и 300x300 из оригинального изображения.
func (s *ImageService) Process(data []byte) (ImageProcessResult, error) {
	sourceImage, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return ImageProcessResult{}, fmt.Errorf("decode image: %w", domain.ErrInvalidFile)
	}

	bounds := sourceImage.Bounds()
	width := bounds.Dx()
	height := bounds.Dy()

	thumbnails := make([]ImageThumbnail, 0, 2)

	thumbnail100, err := createSquareJPEGThumbnail(sourceImage, domain.ThumbnailSize100, 100)
	if err != nil {
		return ImageProcessResult{}, err
	}

	thumbnail300, err := createSquareJPEGThumbnail(sourceImage, domain.ThumbnailSize300, 300)
	if err != nil {
		return ImageProcessResult{}, err
	}

	thumbnails = append(thumbnails, thumbnail100, thumbnail300)

	return ImageProcessResult{
		Width:      width,
		Height:     height,
		Thumbnails: thumbnails,
	}, nil
}

// createSquareJPEGThumbnail создаёт квадратную JPEG-миниатюру нужного размера.
func createSquareJPEGThumbnail(
	source image.Image,
	size domain.ThumbnailSize,
	targetSize int,
) (ImageThumbnail, error) {
	cropped := cropCenterSquare(source)

	destination := image.NewRGBA(image.Rect(0, 0, targetSize, targetSize))

	draw.CatmullRom.Scale(
		destination,
		destination.Bounds(),
		cropped,
		cropped.Bounds(),
		draw.Over,
		nil,
	)

	var buffer bytes.Buffer

	if err := jpeg.Encode(&buffer, destination, &jpeg.Options{
		Quality: thumbnailJPEGQuality,
	}); err != nil {
		return ImageThumbnail{}, fmt.Errorf("encode thumbnail: %w", err)
	}

	return ImageThumbnail{
		Size:        size,
		Width:       targetSize,
		Height:      targetSize,
		Data:        buffer.Bytes(),
		ContentType: "image/jpeg",
		Extension:   ".jpg",
	}, nil
}

// cropCenterSquare возвращает центральный квадрат изображения.
func cropCenterSquare(source image.Image) image.Image {
	bounds := source.Bounds()
	width := bounds.Dx()
	height := bounds.Dy()

	if width == height {
		return source
	}

	side := width
	if height < width {
		side = height
	}

	minX := bounds.Min.X + (width-side)/2
	minY := bounds.Min.Y + (height-side)/2

	cropRect := image.Rect(minX, minY, minX+side, minY+side)

	return cropImage(source, cropRect)
}

// cropImage вырезает указанную область изображения.
func cropImage(source image.Image, rect image.Rectangle) image.Image {
	destination := image.NewRGBA(image.Rect(0, 0, rect.Dx(), rect.Dy()))

	draw.Draw(
		destination,
		destination.Bounds(),
		source,
		rect.Min,
		draw.Src,
	)

	return destination
}

// EncodePNGForTest кодирует изображение в PNG.
// Функция используется только в тестах пакета services.
func EncodePNGForTest(img image.Image) ([]byte, error) {
	var buffer bytes.Buffer

	if err := png.Encode(&buffer, img); err != nil {
		return nil, fmt.Errorf("encode png: %w", err)
	}

	return buffer.Bytes(), nil
}
