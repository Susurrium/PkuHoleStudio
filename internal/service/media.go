package service

import (
	"context"
	"errors"
	"fmt"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/Susurrium/PkuHoleStudio/internal/client"
)

var mediaExtensions = []string{".webp", ".jpg", ".jpeg", ".png", ".gif"}

type MediaRequest struct {
	ID        string
	PID       int32
	Thumbnail bool
}

type MediaFile struct {
	Path        string
	ContentType string
	Size        int64
}

type MediaRemote interface {
	DownloadImage(ctx context.Context, id string, pid int32) ([]byte, error)
	DownloadThumbnail(ctx context.Context, id string, pid int32) ([]byte, error)
}

type MediaService struct {
	imagesDir     string
	thumbnailsDir string
	remote        MediaRemote
}

func NewMediaService(dataDir string, remote MediaRemote) *MediaService {
	if strings.TrimSpace(dataDir) == "" {
		dataDir = "data"
	}
	return &MediaService{
		imagesDir:     filepath.Join(dataDir, "images"),
		thumbnailsDir: filepath.Join(dataDir, "thumbnails"),
		remote:        remote,
	}
}

func NewTreeholeMediaRemote(c *client.Client) MediaRemote {
	if c == nil {
		return nil
	}
	return &treeholeMediaRemote{client: c}
}

func (s *MediaService) Locate(ctx context.Context, request MediaRequest) (MediaFile, error) {
	if err := contextError(ctx); err != nil {
		return MediaFile{}, err
	}
	base, err := mediaBaseName(request)
	if err != nil {
		return MediaFile{}, err
	}
	dir := s.imagesDir
	if request.Thumbnail {
		dir = s.thumbnailsDir
	}
	for _, ext := range mediaExtensions {
		path := filepath.Join(dir, base+ext)
		info, statErr := os.Stat(path)
		if statErr == nil && !info.IsDir() {
			return MediaFile{Path: path, ContentType: mime.TypeByExtension(ext), Size: info.Size()}, nil
		}
		if statErr != nil && !errors.Is(statErr, os.ErrNotExist) {
			return MediaFile{}, statErr
		}
	}
	return MediaFile{}, os.ErrNotExist
}

func (s *MediaService) Read(ctx context.Context, request MediaRequest) ([]byte, MediaFile, error) {
	file, err := s.Locate(ctx, request)
	if err != nil {
		return nil, MediaFile{}, err
	}
	data, err := os.ReadFile(file.Path)
	if err != nil {
		return nil, MediaFile{}, err
	}
	return data, file, contextError(ctx)
}

func (s *MediaService) Download(ctx context.Context, request MediaRequest) (MediaFile, error) {
	if err := contextError(ctx); err != nil {
		return MediaFile{}, err
	}
	if s == nil || s.remote == nil {
		return MediaFile{}, errors.New("media remote is not configured")
	}
	base, err := mediaBaseName(request)
	if err != nil {
		return MediaFile{}, err
	}
	var data []byte
	if request.Thumbnail {
		data, err = s.remote.DownloadThumbnail(ctx, request.ID, request.PID)
	} else {
		data, err = s.remote.DownloadImage(ctx, request.ID, request.PID)
	}
	if err != nil {
		return MediaFile{}, err
	}
	if err := contextError(ctx); err != nil {
		return MediaFile{}, err
	}
	contentType := http.DetectContentType(data)
	ext := extensionForContentType(contentType)
	dir := s.imagesDir
	if request.Thumbnail {
		dir = s.thumbnailsDir
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return MediaFile{}, err
	}
	path := filepath.Join(dir, base+ext)
	temporary, err := os.CreateTemp(dir, ".media-*")
	if err != nil {
		return MediaFile{}, err
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)
	if _, err := temporary.Write(data); err != nil {
		temporary.Close()
		return MediaFile{}, err
	}
	if err := temporary.Close(); err != nil {
		return MediaFile{}, err
	}
	if err := os.Rename(temporaryPath, path); err != nil {
		return MediaFile{}, err
	}
	return MediaFile{Path: path, ContentType: contentType, Size: int64(len(data))}, nil
}

func mediaBaseName(request MediaRequest) (string, error) {
	id := strings.TrimSpace(request.ID)
	if id != "" {
		if _, err := strconv.ParseInt(id, 10, 64); err != nil {
			return "", fmt.Errorf("invalid media id %q", request.ID)
		}
		return id, nil
	}
	if request.PID <= 0 {
		return "", errors.New("media id or pid is required")
	}
	return strconv.FormatInt(int64(request.PID), 10), nil
}

func extensionForContentType(contentType string) string {
	switch strings.ToLower(strings.TrimSpace(strings.Split(contentType, ";")[0])) {
	case "image/webp":
		return ".webp"
	case "image/png":
		return ".png"
	case "image/gif":
		return ".gif"
	default:
		return ".jpg"
	}
}

type treeholeMediaRemote struct{ client *client.Client }

func (r *treeholeMediaRemote) DownloadImage(ctx context.Context, id string, pid int32) ([]byte, error) {
	if err := contextError(ctx); err != nil {
		return nil, err
	}
	return r.client.DownloadImageBinary(id, pid)
}

func (r *treeholeMediaRemote) DownloadThumbnail(ctx context.Context, id string, pid int32) ([]byte, error) {
	if err := contextError(ctx); err != nil {
		return nil, err
	}
	return r.client.DownloadThumbnail(id, pid)
}
