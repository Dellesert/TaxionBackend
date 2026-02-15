package usecase

import (
	"bytes"
	"crypto/md5"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"image"
	"image/jpeg"
	"image/png"
	"io"
	"mime/multipart"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/nfnt/resize"
	"tachyon-messenger/services/file/models"
	"tachyon-messenger/services/file/repository"
)

// FileUsecase handles file business logic
type FileUsecase struct {
	repo           *repository.FileRepository
	uploadDir      string
	maxFileSize    int64
	allowedTypes   map[string]bool
	baseURL        string
}

// NewFileUsecase creates a new file usecase
func NewFileUsecase(repo *repository.FileRepository, uploadDir, baseURL string) *FileUsecase {
	// Allowed MIME types
	allowedTypes := map[string]bool{
		// Images
		"image/jpeg":               true,
		"image/png":                true,
		"image/gif":                true,
		"image/webp":               true,
		"image/svg+xml":            true,
		"image/heic":               true,
		"image/heif":               true,
		"image/bmp":                true,
		"image/tiff":               true,
		"image/avif":               true,
		"image/x-icon":             true,
		"image/vnd.microsoft.icon": true,
		// Documents
		"application/pdf":                                                               true,
		"application/msword":                                                            true,
		"application/vnd.openxmlformats-officedocument.wordprocessingml.document":       true,
		"application/vnd.ms-excel":                                                      true,
		"application/vnd.openxmlformats-officedocument.spreadsheetml.sheet":             true,
		"application/vnd.ms-powerpoint":                                                 true,
		"application/vnd.openxmlformats-officedocument.presentationml.presentation":     true,
		"text/plain":                                                                    true,
		"text/csv":                                                                      true,
		"application/rtf":                                                               true,
		"text/markdown":                                                                 true,
		"application/json":                                                              true,
		"text/xml":                                                                      true,
		"application/xml":                                                               true,
		// Video
		"video/mp4":          true,
		"video/mpeg":         true,
		"video/webm":         true,
		"video/quicktime":    true, // MOV (iPhone)
		"video/x-msvideo":    true, // AVI
		"video/3gpp":         true, // 3GP
		"video/x-matroska":   true, // MKV
		// Audio
		"audio/mpeg":         true,
		"audio/wav":          true,
		"audio/x-wav":        true,
		"audio/ogg":          true,
		"audio/webm":         true,
		"audio/mp4":          true, // M4A
		"audio/x-m4a":        true, // M4A alternative
		"audio/aac":          true,
		"audio/flac":         true,
		// Archives
		"application/zip":              true,
		"application/x-rar-compressed": true,
		"application/vnd.rar":          true,
		"application/x-7z-compressed":  true,
		"application/gzip":             true,
		"application/x-gzip":           true,
		"application/x-tar":            true,
	}

	return &FileUsecase{
		repo:         repo,
		uploadDir:    uploadDir,
		maxFileSize:  1000 * 1024 * 1024, // 1000 MB default
		allowedTypes: allowedTypes,
		baseURL:      baseURL,
	}
}

// isHEIC checks if the file is a HEIC/HEIF image
func (u *FileUsecase) isHEIC(mimeType string) bool {
	return mimeType == "image/heic" || mimeType == "image/heif"
}

// isImage checks if the file is an image that supports thumbnails
func (u *FileUsecase) isImage(mimeType string) bool {
	supportedFormats := []string{
		"image/jpeg",
		"image/png",
		"image/gif",
		"image/webp",
		"image/bmp",
	}
	for _, format := range supportedFormats {
		if mimeType == format {
			return true
		}
	}
	return false
}

// createThumbnail creates a thumbnail for an image file
// Target size: 400x300px, maintaining aspect ratio
func (u *FileUsecase) createThumbnail(originalPath string, mimeType string) (string, int64, error) {
	// Open original file
	file, err := os.Open(originalPath)
	if err != nil {
		return "", 0, fmt.Errorf("failed to open image: %w", err)
	}
	defer file.Close()

	// Decode image based on MIME type
	var img image.Image
	switch mimeType {
	case "image/jpeg", "image/jpg":
		img, err = jpeg.Decode(file)
	case "image/png":
		img, err = png.Decode(file)
	default:
		// For other formats, try generic decode
		img, _, err = image.Decode(file)
	}
	if err != nil {
		return "", 0, fmt.Errorf("failed to decode image: %w", err)
	}

	// Calculate thumbnail dimensions (max 400x300, maintain aspect ratio)
	const maxWidth = 400
	const maxHeight = 300
	bounds := img.Bounds()
	width := bounds.Dx()
	height := bounds.Dy()

	// Calculate new dimensions maintaining aspect ratio
	var newWidth, newHeight uint
	aspectRatio := float64(width) / float64(height)

	if aspectRatio > float64(maxWidth)/float64(maxHeight) {
		// Width is the limiting factor
		newWidth = maxWidth
		newHeight = uint(float64(maxWidth) / aspectRatio)
	} else {
		// Height is the limiting factor
		newHeight = maxHeight
		newWidth = uint(float64(maxHeight) * aspectRatio)
	}

	// Resize image
	thumbnail := resize.Resize(newWidth, newHeight, img, resize.Lanczos3)

	// Generate thumbnail filename
	ext := filepath.Ext(originalPath)
	thumbnailPath := strings.TrimSuffix(originalPath, ext) + "_thumb" + ext

	// Create thumbnail file
	out, err := os.Create(thumbnailPath)
	if err != nil {
		return "", 0, fmt.Errorf("failed to create thumbnail file: %w", err)
	}
	defer out.Close()

	// Encode thumbnail with quality optimization
	switch mimeType {
	case "image/jpeg", "image/jpg":
		// JPEG with quality 85 for good balance between size and quality
		err = jpeg.Encode(out, thumbnail, &jpeg.Options{Quality: 85})
	case "image/png":
		// PNG encoding
		err = png.Encode(out, thumbnail)
	default:
		// Default to JPEG for other formats
		err = jpeg.Encode(out, thumbnail, &jpeg.Options{Quality: 85})
		if err == nil {
			// Update path if we converted to JPEG
			newThumbnailPath := strings.TrimSuffix(thumbnailPath, ext) + ".jpg"
			os.Rename(thumbnailPath, newThumbnailPath)
			thumbnailPath = newThumbnailPath
		}
	}

	if err != nil {
		os.Remove(thumbnailPath) // Clean up on error
		return "", 0, fmt.Errorf("failed to encode thumbnail: %w", err)
	}

	// Get thumbnail file size
	fileInfo, err := os.Stat(thumbnailPath)
	if err != nil {
		return "", 0, fmt.Errorf("failed to get thumbnail file info: %w", err)
	}

	return thumbnailPath, fileInfo.Size(), nil
}

// fixImageOrientation applies EXIF orientation to an image file using ImageMagick
// This physically rotates the pixels according to EXIF Orientation tag
// and then removes the tag (so the image displays correctly everywhere)
// This fixes rotation issues with photos from iPhones and other cameras
func (u *FileUsecase) fixImageOrientation(imagePath string) error {
	fmt.Printf("fixImageOrientation: starting for %s\n", imagePath)

	// Try "magick mogrify" first (ImageMagick 7), fallback to "mogrify" (older versions or Alpine)
	var cmd *exec.Cmd
	var stderr bytes.Buffer
	var stdout bytes.Buffer

	// First try: magick mogrify (ImageMagick 7 style)
	cmd = exec.Command("magick", "mogrify", "-auto-orient", imagePath)
	cmd.Stderr = &stderr
	cmd.Stdout = &stdout

	fmt.Printf("fixImageOrientation: trying 'magick mogrify -auto-orient %s'\n", imagePath)
	if err := cmd.Run(); err != nil {
		fmt.Printf("fixImageOrientation: magick failed: %v, stderr: %s\n", err, stderr.String())

		// Fallback: try mogrify directly (works on some Alpine installations)
		stderr.Reset()
		stdout.Reset()
		cmd = exec.Command("mogrify", "-auto-orient", imagePath)
		cmd.Stderr = &stderr
		cmd.Stdout = &stdout

		fmt.Printf("fixImageOrientation: trying 'mogrify -auto-orient %s'\n", imagePath)
		if err := cmd.Run(); err != nil {
			fmt.Printf("fixImageOrientation: mogrify also failed: %v, stderr: %s\n", err, stderr.String())
			return fmt.Errorf("failed to apply EXIF orientation: %v, stderr: %s", err, stderr.String())
		}
		fmt.Printf("fixImageOrientation: mogrify succeeded\n")
	} else {
		fmt.Printf("fixImageOrientation: magick succeeded\n")
	}

	return nil
}

// convertHEICtoJPEG converts a HEIC file to JPEG using ImageMagick with auto-orient
// This preserves and applies EXIF orientation in a single step, preventing rotation issues
func (u *FileUsecase) convertHEICtoJPEG(heicPath string) (string, error) {
	// Generate output path with .jpg extension
	jpegPath := strings.TrimSuffix(heicPath, filepath.Ext(heicPath)) + ".jpg"

	var stderr bytes.Buffer
	var cmd *exec.Cmd

	// Use ImageMagick to convert HEIC to JPEG with auto-orient applied
	// This reads EXIF orientation, physically rotates pixels, and strips the orientation tag
	cmd = exec.Command("magick", "convert", heicPath, "-auto-orient", "-quality", "90", jpegPath)
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		fmt.Printf("convertHEICtoJPEG: magick convert failed: %v, stderr: %s\n", err, stderr.String())

		// Fallback: try with 'convert' directly (older ImageMagick or Alpine)
		stderr.Reset()
		cmd = exec.Command("convert", heicPath, "-auto-orient", "-quality", "90", jpegPath)
		cmd.Stderr = &stderr

		if err := cmd.Run(); err != nil {
			fmt.Printf("convertHEICtoJPEG: convert also failed: %v, stderr: %s\n", err, stderr.String())

			// Last fallback: use heif-convert without orientation fix
			stderr.Reset()
			cmd = exec.Command("heif-convert", "-q", "90", heicPath, jpegPath)
			cmd.Stderr = &stderr

			if err := cmd.Run(); err != nil {
				return "", fmt.Errorf("all HEIC conversion methods failed: %v, stderr: %s", err, stderr.String())
			}
		}
	}

	// Remove original HEIC file
	os.Remove(heicPath)

	return jpegPath, nil
}

// isVideo checks if the file is a video
func (u *FileUsecase) isVideo(mimeType string) bool {
	videoFormats := []string{
		"video/mp4",
		"video/mpeg",
		"video/webm",
		"video/quicktime",
		"video/x-msvideo",
		"video/3gpp",
		"video/x-matroska",
	}
	for _, format := range videoFormats {
		if mimeType == format {
			return true
		}
	}
	return false
}

// createVideoThumbnail creates a thumbnail for a video file using ffmpeg
// Extracts a frame at 1 second (or first frame for very short videos)
func (u *FileUsecase) createVideoThumbnail(videoPath string) (string, int64, error) {
	// Generate thumbnail path
	ext := filepath.Ext(videoPath)
	thumbnailPath := strings.TrimSuffix(videoPath, ext) + "_thumb.jpg"

	// Use ffmpeg to extract a frame at 1 second, scaled to 400px width
	var stderr bytes.Buffer
	cmd := exec.Command("ffmpeg",
		"-i", videoPath,
		"-ss", "1",           // seek to 1 second
		"-vframes", "1",      // extract 1 frame
		"-vf", "scale=400:-2",
		"-q:v", "5",          // JPEG quality (2-31, lower is better)
		"-y",                 // overwrite output
		thumbnailPath,
	)
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		// If seeking to 1s fails (video too short), try extracting the first frame
		stderr.Reset()
		cmd = exec.Command("ffmpeg",
			"-i", videoPath,
			"-vframes", "1",
			"-vf", "scale=400:-2",
			"-q:v", "5",
			"-y",
			thumbnailPath,
		)
		cmd.Stderr = &stderr

		if err := cmd.Run(); err != nil {
			return "", 0, fmt.Errorf("ffmpeg thumbnail extraction failed: %v, stderr: %s", err, stderr.String())
		}
	}

	// Get thumbnail file size
	fileInfo, err := os.Stat(thumbnailPath)
	if err != nil {
		return "", 0, fmt.Errorf("failed to get thumbnail file info: %w", err)
	}

	return thumbnailPath, fileInfo.Size(), nil
}

// getVideoDuration extracts video duration in seconds using ffprobe
func (u *FileUsecase) getVideoDuration(videoPath string) (float64, error) {
	var stdout, stderr bytes.Buffer
	cmd := exec.Command("ffprobe",
		"-v", "error",
		"-show_entries", "format=duration",
		"-of", "default=noprint_wrappers=1:nokey=1",
		videoPath,
	)
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return 0, fmt.Errorf("ffprobe failed: %v, stderr: %s", err, stderr.String())
	}

	durationStr := strings.TrimSpace(stdout.String())
	duration, err := strconv.ParseFloat(durationStr, 64)
	if err != nil {
		return 0, fmt.Errorf("failed to parse duration '%s': %w", durationStr, err)
	}

	return duration, nil
}

// getVideoDimensions extracts video width and height using ffprobe
func (u *FileUsecase) getVideoDimensions(videoPath string) (int, int, error) {
	var stdout, stderr bytes.Buffer
	cmd := exec.Command("ffprobe",
		"-v", "error",
		"-select_streams", "v:0",
		"-show_entries", "stream=width,height",
		"-of", "csv=p=0:s=x",
		videoPath,
	)
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return 0, 0, fmt.Errorf("ffprobe failed: %v, stderr: %s", err, stderr.String())
	}

	parts := strings.Split(strings.TrimSpace(stdout.String()), "x")
	if len(parts) != 2 {
		return 0, 0, fmt.Errorf("unexpected ffprobe output: %s", stdout.String())
	}

	width, err := strconv.Atoi(parts[0])
	if err != nil {
		return 0, 0, fmt.Errorf("failed to parse width: %w", err)
	}
	height, err := strconv.Atoi(parts[1])
	if err != nil {
		return 0, 0, fmt.Errorf("failed to parse height: %w", err)
	}

	return width, height, nil
}

// getImageDimensions extracts image width and height by reading the image header
func (u *FileUsecase) getImageDimensions(imagePath string) (int, int, error) {
	file, err := os.Open(imagePath)
	if err != nil {
		return 0, 0, fmt.Errorf("failed to open image: %w", err)
	}
	defer file.Close()

	config, _, err := image.DecodeConfig(file)
	if err != nil {
		return 0, 0, fmt.Errorf("failed to decode image config: %w", err)
	}

	return config.Width, config.Height, nil
}

// compressVideo converts a video to H.264 MP4 with max 720p resolution using FFmpeg
// Returns the path to the compressed file
func (u *FileUsecase) compressVideo(videoPath string) (string, error) {
	// Generate output path (always .mp4)
	ext := filepath.Ext(videoPath)
	outputPath := strings.TrimSuffix(videoPath, ext) + "_compressed.mp4"

	var stderr bytes.Buffer
	cmd := exec.Command("ffmpeg",
		"-i", videoPath,
		"-vf", "scale='min(1280,iw)':'min(720,ih)':force_original_aspect_ratio=decrease,pad=ceil(iw/2)*2:ceil(ih/2)*2",
		"-c:v", "libx264",
		"-pix_fmt", "yuv420p",
		"-crf", "23",
		"-preset", "medium",
		"-c:a", "aac",
		"-b:a", "128k",
		"-movflags", "+faststart",
		"-y",
		outputPath,
	)
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("ffmpeg compression failed: %v, stderr: %s", err, stderr.String())
	}

	return outputPath, nil
}

// processVideoAsync handles video compression in the background
// It compresses the video, updates the thumbnail, and updates the database record
func (u *FileUsecase) processVideoAsync(fileID uint, originalPath string) {
	fmt.Printf("processVideoAsync: starting compression for file ID %d, path: %s\n", fileID, originalPath)

	// Compress video
	compressedPath, err := u.compressVideo(originalPath)
	if err != nil {
		fmt.Printf("processVideoAsync: compression failed for file ID %d: %v\n", fileID, err)
		// Update status to failed
		if file, getErr := u.repo.GetByID(fileID); getErr == nil {
			file.ConversionStatus = "failed"
			u.repo.Update(file)
		}
		return
	}

	// Get compressed file info
	compressedInfo, err := os.Stat(compressedPath)
	if err != nil {
		fmt.Printf("processVideoAsync: failed to stat compressed file for ID %d: %v\n", fileID, err)
		os.Remove(compressedPath)
		if file, getErr := u.repo.GetByID(fileID); getErr == nil {
			file.ConversionStatus = "failed"
			u.repo.Update(file)
		}
		return
	}

	// Get the file record
	file, err := u.repo.GetByID(fileID)
	if err != nil {
		fmt.Printf("processVideoAsync: failed to get file record for ID %d: %v\n", fileID, err)
		os.Remove(compressedPath)
		return
	}

	// Replace with the final path (rename compressed to final name)
	finalPath := strings.TrimSuffix(originalPath, filepath.Ext(originalPath)) + ".mp4"
	if finalPath == originalPath {
		// Same extension — use a different final name to avoid conflict
		finalPath = strings.TrimSuffix(originalPath, filepath.Ext(originalPath)) + "_conv.mp4"
	}
	if err := os.Rename(compressedPath, finalPath); err != nil {
		fmt.Printf("processVideoAsync: failed to rename compressed file for ID %d: %v\n", fileID, err)
		os.Remove(compressedPath)
		file.ConversionStatus = "failed"
		u.repo.Update(file)
		return
	}

	// Delete original file if it's different from the final path
	if originalPath != finalPath {
		os.Remove(originalPath)
	}

	// Delete old thumbnail and regenerate from compressed video
	if file.ThumbnailPath != "" {
		os.Remove(file.ThumbnailPath)
	}
	thumbPath, thumbSize, err := u.createVideoThumbnail(finalPath)
	if err != nil {
		fmt.Printf("processVideoAsync: failed to regenerate thumbnail for ID %d: %v\n", fileID, err)
	} else {
		file.ThumbnailPath = thumbPath
		file.ThumbnailSize = thumbSize
	}

	// Re-extract duration and dimensions from compressed video
	if dur, err := u.getVideoDuration(finalPath); err == nil {
		file.Duration = dur
	}
	if w, h, err := u.getVideoDimensions(finalPath); err == nil {
		file.Width = w
		file.Height = h
	}

	// Update database record
	originalSize := file.FileSize
	file.FilePath = finalPath
	file.FileName = filepath.Base(finalPath)
	file.FileSize = compressedInfo.Size()
	file.MimeType = "video/mp4"
	file.OriginalName = strings.TrimSuffix(file.OriginalName, filepath.Ext(file.OriginalName)) + ".mp4"
	file.ConversionStatus = "completed"

	if err := u.repo.Update(file); err != nil {
		fmt.Printf("processVideoAsync: failed to update file record for ID %d: %v\n", fileID, err)
		return
	}

	fmt.Printf("processVideoAsync: completed for file ID %d, original size: %d, compressed size: %d\n",
		fileID, originalSize, compressedInfo.Size())
}

// needsVideoConversion checks if a video file needs conversion
// Small MP4 files (< 5MB) are skipped
func (u *FileUsecase) needsVideoConversion(mimeType string, fileSize int64) bool {
	if mimeType == "video/mp4" && fileSize < 5*1024*1024 {
		return false
	}
	return true
}

// UploadFile uploads a file and creates a record in the database
func (u *FileUsecase) UploadFile(
	file *multipart.FileHeader,
	fileType models.FileType,
	uploadedBy uint,
	entityType *string,
	entityID *uint,
	isPublic bool,
) (*models.File, error) {
	// Validate file size
	if file.Size > u.maxFileSize {
		return nil, fmt.Errorf("file size exceeds maximum allowed size of %d bytes", u.maxFileSize)
	}

	// Get MIME type
	mimeType := file.Header.Get("Content-Type")

	// Validate MIME type
	if !u.allowedTypes[mimeType] {
		return nil, errors.New("file type not allowed")
	}

	// Open uploaded file
	src, err := file.Open()
	if err != nil {
		return nil, fmt.Errorf("failed to open uploaded file: %w", err)
	}
	defer src.Close()

	// Calculate SHA-256 hash of file content for deduplication
	hasher := sha256.New()
	if _, err := io.Copy(hasher, src); err != nil {
		return nil, fmt.Errorf("failed to calculate file hash: %w", err)
	}
	contentHash := hex.EncodeToString(hasher.Sum(nil))

	// Check if a file with the same content already exists
	if existingFile, err := u.repo.GetByContentHash(contentHash); err == nil && existingFile != nil {
		return existingFile, nil
	}

	// Reset file reader to beginning for saving
	if _, err := src.Seek(0, io.SeekStart); err != nil {
		return nil, fmt.Errorf("failed to reset file reader: %w", err)
	}

	// Generate unique filename
	fileName, err := u.generateUniqueFileName(file.Filename)
	if err != nil {
		return nil, fmt.Errorf("failed to generate filename: %w", err)
	}

	// Create upload directory if it doesn't exist
	uploadPath := filepath.Join(u.uploadDir, string(fileType))
	if err := os.MkdirAll(uploadPath, 0755); err != nil {
		return nil, fmt.Errorf("failed to create upload directory: %w", err)
	}

	// Full file path
	filePath := filepath.Join(uploadPath, fileName)

	// Create destination file
	dst, err := os.Create(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to create destination file: %w", err)
	}
	defer dst.Close()

	// Copy file content
	written, err := io.Copy(dst, src)
	if err != nil {
		os.Remove(filePath) // Clean up on error
		return nil, fmt.Errorf("failed to save file: %w", err)
	}

	// Close destination file before conversion
	dst.Close()

	// Convert HEIC to JPEG if necessary
	originalName := file.Filename
	finalMimeType := mimeType
	if u.isHEIC(mimeType) {
		convertedPath, err := u.convertHEICtoJPEG(filePath)
		if err != nil {
			os.Remove(filePath) // Clean up on error
			return nil, fmt.Errorf("failed to convert HEIC to JPEG: %w", err)
		}
		filePath = convertedPath
		fileName = filepath.Base(convertedPath)
		finalMimeType = "image/jpeg"
		// Update original name extension
		originalName = strings.TrimSuffix(originalName, filepath.Ext(originalName)) + ".jpg"

		// Get new file size after conversion
		fileInfo, err := os.Stat(filePath)
		if err != nil {
			os.Remove(filePath) // Clean up on error
			return nil, fmt.Errorf("failed to get converted file info: %w", err)
		}
		written = fileInfo.Size()
	}

	// Fix EXIF orientation for all JPEG/PNG images (fixes rotation issues from iPhone/cameras)
	// This applies the EXIF Orientation tag to the actual pixels and removes the tag
	if u.isImage(finalMimeType) {
		if err := u.fixImageOrientation(filePath); err != nil {
			// Log warning but don't fail - image is still usable, just might be rotated
			fmt.Printf("Warning: failed to fix image orientation: %v\n", err)
		} else {
			// Update file size after orientation fix (image might have been rotated)
			fileInfo, err := os.Stat(filePath)
			if err == nil {
				written = fileInfo.Size()
			}
		}
	}

	// Create thumbnail for images
	var thumbnailPath string
	var thumbnailSize int64
	var mediaWidth, mediaHeight int
	if u.isImage(finalMimeType) {
		thumbPath, thumbSize, err := u.createThumbnail(filePath, finalMimeType)
		if err != nil {
			// Log error but don't fail the upload if thumbnail creation fails
			// Just continue without thumbnail
			fmt.Printf("Warning: failed to create thumbnail: %v\n", err)
		} else {
			thumbnailPath = thumbPath
			thumbnailSize = thumbSize
		}

		// Extract image dimensions
		if w, h, err := u.getImageDimensions(filePath); err != nil {
			fmt.Printf("Warning: failed to get image dimensions: %v\n", err)
		} else {
			mediaWidth = w
			mediaHeight = h
		}
	}

	// Create thumbnail and extract duration for videos
	var duration float64
	if u.isVideo(finalMimeType) {
		// Generate video thumbnail
		thumbPath, thumbSize, err := u.createVideoThumbnail(filePath)
		if err != nil {
			fmt.Printf("Warning: failed to create video thumbnail: %v\n", err)
		} else {
			thumbnailPath = thumbPath
			thumbnailSize = thumbSize
		}

		// Extract video duration
		dur, err := u.getVideoDuration(filePath)
		if err != nil {
			fmt.Printf("Warning: failed to get video duration: %v\n", err)
		} else {
			duration = dur
		}

		// Extract video dimensions
		if w, h, err := u.getVideoDimensions(filePath); err != nil {
			fmt.Printf("Warning: failed to get video dimensions: %v\n", err)
		} else {
			mediaWidth = w
			mediaHeight = h
		}
	}

	// Convert video synchronously before creating DB record
	conversionStatus := ""
	if u.isVideo(finalMimeType) && u.needsVideoConversion(finalMimeType, written) {
		fmt.Printf("Upload: starting synchronous video conversion for %s\n", filePath)

		compressedPath, err := u.compressVideo(filePath)
		if err != nil {
			fmt.Printf("Upload: video conversion failed: %v\n", err)
			os.Remove(filePath)
			return nil, fmt.Errorf("video conversion failed: %w", err)
		}

		// Rename compressed file to final path
		finalPath := strings.TrimSuffix(filePath, filepath.Ext(filePath)) + ".mp4"
		if finalPath == filePath {
			finalPath = strings.TrimSuffix(filePath, filepath.Ext(filePath)) + "_conv.mp4"
		}
		if err := os.Rename(compressedPath, finalPath); err != nil {
			os.Remove(compressedPath)
			os.Remove(filePath)
			return nil, fmt.Errorf("failed to rename converted video: %w", err)
		}

		// Delete original if different
		if filePath != finalPath {
			os.Remove(filePath)
		}

		// Update file info with converted data
		compressedInfo, err := os.Stat(finalPath)
		if err != nil {
			os.Remove(finalPath)
			return nil, fmt.Errorf("failed to stat converted video: %w", err)
		}

		filePath = finalPath
		fileName = filepath.Base(finalPath)
		originalName = strings.TrimSuffix(originalName, filepath.Ext(originalName)) + ".mp4"
		written = compressedInfo.Size()
		finalMimeType = "video/mp4"
		conversionStatus = "completed"

		// Regenerate thumbnail from converted video
		if thumbnailPath != "" {
			os.Remove(thumbnailPath)
		}
		thumbPath, thumbSize, err := u.createVideoThumbnail(finalPath)
		if err != nil {
			fmt.Printf("Warning: failed to create thumbnail for converted video: %v\n", err)
		} else {
			thumbnailPath = thumbPath
			thumbnailSize = thumbSize
		}

		// Re-extract duration and dimensions from converted video
		if dur, err := u.getVideoDuration(finalPath); err == nil {
			duration = dur
		}
		if w, h, err := u.getVideoDimensions(finalPath); err == nil {
			mediaWidth = w
			mediaHeight = h
		}

		fmt.Printf("Upload: video conversion completed, size: %d -> %d\n", written, compressedInfo.Size())
	}

	// Create file record
	fileRecord := &models.File{
		FileName:         fileName,
		OriginalName:     originalName,
		FilePath:         filePath,
		FileSize:         written,
		ThumbnailPath:    thumbnailPath,
		ThumbnailSize:    thumbnailSize,
		MimeType:         finalMimeType,
		FileType:         fileType,
		UploadedBy:       uploadedBy,
		EntityType:       entityType,
		EntityID:         entityID,
		IsPublic:         isPublic,
		Duration:         duration,
		Width:            mediaWidth,
		Height:           mediaHeight,
		ConversionStatus: conversionStatus,
		ContentHash:      contentHash,
	}

	// Save to database
	if err := u.repo.Create(fileRecord); err != nil {
		os.Remove(filePath) // Clean up on error
		return nil, fmt.Errorf("failed to save file record: %w", err)
	}

	return fileRecord, nil
}

// GetFile retrieves a file by ID
func (u *FileUsecase) GetFile(id uint, userID uint) (*models.File, error) {
	file, err := u.repo.GetByID(id)
	if err != nil {
		return nil, err
	}

	// Check access permissions
	if !file.IsPublic && file.UploadedBy != userID {
		return nil, errors.New("access denied")
	}

	return file, nil
}

// GetFileByID retrieves a file by ID without access control (for internal service use)
func (u *FileUsecase) GetFileByID(id uint) (*models.File, error) {
	return u.repo.GetByID(id)
}

// GetFileByName retrieves a file by filename
// If the filename ends with _thumb, it will try to find the original file and return it with thumbnail path
func (u *FileUsecase) GetFileByName(fileName string, userID uint) (*models.File, error) {
	// Check if this is a thumbnail request
	isThumbnail := strings.Contains(fileName, "_thumb")

	if isThumbnail {
		// For thumbnails, search by thumbnail_path directly
		// This handles cases where thumbnail extension differs from original (e.g., video.mp4 -> video_thumb.jpg)
		file, err := u.repo.GetByThumbnailFileName(fileName)
		if err != nil {
			// Fallback: try the old approach for backwards compatibility
			ext := filepath.Ext(fileName)
			originalFileName := strings.Replace(fileName, "_thumb"+ext, ext, 1)
			file, err = u.repo.GetByFileName(originalFileName)
			if err != nil {
				return nil, err
			}
		}

		// Check access permissions
		if !file.IsPublic && file.UploadedBy != userID {
			return nil, errors.New("access denied")
		}

		return file, nil
	}

	file, err := u.repo.GetByFileName(fileName)
	if err != nil {
		return nil, err
	}

	// Check access permissions
	if !file.IsPublic && file.UploadedBy != userID {
		return nil, errors.New("access denied")
	}

	return file, nil
}

// GetFileByNameInternal retrieves a file by filename without access control check
// Used for internal service-to-service communication
func (u *FileUsecase) GetFileByNameInternal(fileName string) (*models.File, error) {
	return u.repo.GetByFileName(fileName)
}

// GetPublicFileByName retrieves a public file by filename (no auth required)
// If the filename ends with _thumb, it will try to find the original file and return it with thumbnail path
func (u *FileUsecase) GetPublicFileByName(fileName string) (*models.File, error) {
	// Check if this is a thumbnail request
	isThumbnail := strings.Contains(fileName, "_thumb")

	if isThumbnail {
		// For thumbnails, search by thumbnail_path directly
		// This handles cases where thumbnail extension differs from original (e.g., video.mp4 -> video_thumb.jpg)
		file, err := u.repo.GetByThumbnailFileName(fileName)
		if err != nil {
			// Fallback: try the old approach for backwards compatibility
			ext := filepath.Ext(fileName)
			originalFileName := strings.Replace(fileName, "_thumb"+ext, ext, 1)
			file, err = u.repo.GetByFileName(originalFileName)
			if err != nil {
				return nil, err
			}
		}

		if !file.IsPublic {
			return nil, errors.New("file is not public")
		}

		return file, nil
	}

	file, err := u.repo.GetByFileName(fileName)
	if err != nil {
		return nil, err
	}

	if !file.IsPublic {
		return nil, errors.New("file is not public")
	}

	return file, nil
}

// GetFilesByEntity retrieves files for a specific entity
func (u *FileUsecase) GetFilesByEntity(entityType string, entityID uint) ([]models.File, error) {
	return u.repo.GetByEntity(entityType, entityID)
}

// GetUserFiles retrieves files uploaded by a user
func (u *FileUsecase) GetUserFiles(userID uint, limit, offset int) ([]models.File, int64, error) {
	if limit <= 0 {
		limit = 20
	}
	if offset < 0 {
		offset = 0
	}

	return u.repo.GetByUploader(userID, limit, offset)
}

// ListFiles retrieves files with filters
func (u *FileUsecase) ListFiles(filter *models.FileFilterRequest, userID uint) ([]models.File, int64, error) {
	// Users can only list their own files unless they're admin
	// For now, we'll restrict to user's own files
	filter.UploadedBy = &userID

	return u.repo.List(filter)
}

// DeleteFile deletes a file (both record and physical file)
func (u *FileUsecase) DeleteFile(id uint, userID uint) error {
	file, err := u.repo.GetByID(id)
	if err != nil {
		return err
	}

	// Only the uploader can delete the file
	if file.UploadedBy != userID {
		return errors.New("access denied: only the uploader can delete this file")
	}

	// Delete physical file
	if err := os.Remove(file.FilePath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to delete physical file: %w", err)
	}

	// Delete thumbnail if exists
	if file.ThumbnailPath != "" {
		if err := os.Remove(file.ThumbnailPath); err != nil && !os.IsNotExist(err) {
			// Log error but continue - don't fail if thumbnail deletion fails
			fmt.Printf("Warning: failed to delete thumbnail: %v\n", err)
		}
	}

	// Delete database record
	if err := u.repo.Delete(id); err != nil {
		return fmt.Errorf("failed to delete file record: %w", err)
	}

	return nil
}

// DeleteFilesByEntity deletes all files for an entity
func (u *FileUsecase) DeleteFilesByEntity(entityType string, entityID uint, userID uint) error {
	files, err := u.repo.GetByEntity(entityType, entityID)
	if err != nil {
		return err
	}

	for _, file := range files {
		// Check permission for each file
		if file.UploadedBy != userID {
			continue // Skip files not owned by the user
		}

		// Delete physical file
		if err := os.Remove(file.FilePath); err != nil && !os.IsNotExist(err) {
			// Log error but continue
			continue
		}
	}

	// Delete all records
	return u.repo.DeleteByEntity(entityType, entityID)
}

// GetUserAvatar retrieves user's avatar
func (u *FileUsecase) GetUserAvatar(userID uint) (*models.File, error) {
	return u.repo.GetUserAvatar(userID)
}

// UpdateFileEntity updates the entity association of a file
func (u *FileUsecase) UpdateFileEntity(id uint, entityType *string, entityID *uint, userID uint) error {
	file, err := u.repo.GetByID(id)
	if err != nil {
		return err
	}

	// Only the uploader can update the file
	if file.UploadedBy != userID {
		return errors.New("access denied")
	}

	file.EntityType = entityType
	file.EntityID = entityID

	return u.repo.Update(file)
}

// generateUniqueFileName generates a unique filename based on timestamp and hash
func (u *FileUsecase) generateUniqueFileName(originalName string) (string, error) {
	// Get file extension
	ext := filepath.Ext(originalName)

	// Generate hash from filename and timestamp
	hash := md5.New()
	hash.Write([]byte(originalName + time.Now().String()))
	hashStr := hex.EncodeToString(hash.Sum(nil))

	// Create unique filename: timestamp_hash.ext
	fileName := fmt.Sprintf("%d_%s%s", time.Now().Unix(), hashStr[:16], ext)

	// Sanitize filename (remove any potentially dangerous characters)
	fileName = strings.ReplaceAll(fileName, "..", "")
	fileName = strings.ReplaceAll(fileName, "/", "")
	fileName = strings.ReplaceAll(fileName, "\\", "")

	return fileName, nil
}

// GetBaseURL returns the base URL for file access
func (u *FileUsecase) GetBaseURL() string {
	return u.baseURL
}

// GetFileStatsInternal retrieves file statistics for analytics (no auth required - internal use)
func (u *FileUsecase) GetFileStatsInternal() (*repository.FileStatsInternal, error) {
	return u.repo.GetFileStatsInternal()
}
