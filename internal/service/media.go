package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/Susurrium/PkuHoleStudio/internal/client"
	"github.com/Susurrium/PkuHoleStudio/internal/models"
)

var mediaExtensions = []string{".webp", ".jpg", ".jpeg", ".png", ".gif"}

type MediaRequest struct {
	ID        string
	RecordID  uint
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
	dataDir       string
	imagesDir     string
	thumbnailsDir string
	remote        MediaRemote
	repository    MediaRepository
}

func NewMediaService(dataDir string, remote MediaRemote) *MediaService {
	if strings.TrimSpace(dataDir) == "" {
		dataDir = "data"
	}
	return &MediaService{
		dataDir:       dataDir,
		imagesDir:     filepath.Join(dataDir, "images"),
		thumbnailsDir: filepath.Join(dataDir, "thumbnails"),
		remote:        remote,
	}
}

func NewMediaServiceWithRepository(dataDir string, remote MediaRemote, repository MediaRepository) *MediaService {
	service := NewMediaService(dataDir, remote)
	service.repository = repository
	return service
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
	if request.RecordID > 0 {
		return s.locateRecord(ctx, request.RecordID)
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

func (s *MediaService) locateRecord(ctx context.Context, id uint) (MediaFile, error) {
	if s == nil || s.repository == nil {
		return MediaFile{}, os.ErrNotExist
	}
	record, err := s.repository.GetMediaByID(id)
	if err != nil || record == nil || strings.TrimSpace(record.Path) == "" || record.Status != "available" {
		return MediaFile{}, os.ErrNotExist
	}
	root, err := filepath.Abs(s.dataDir)
	if err != nil {
		return MediaFile{}, err
	}
	mediaPath := filepath.FromSlash(record.Path)
	if !filepath.IsAbs(mediaPath) {
		mediaPath = filepath.Join(root, mediaPath)
	}
	mediaPath, err = filepath.Abs(mediaPath)
	if err != nil {
		return MediaFile{}, err
	}
	relative, err := filepath.Rel(root, mediaPath)
	if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return MediaFile{}, errors.New("media path is outside the data directory")
	}
	info, err := os.Stat(mediaPath)
	if errors.Is(err, os.ErrNotExist) {
		return MediaFile{}, os.ErrNotExist
	}
	if err != nil {
		return MediaFile{}, err
	}
	contentType := record.MIMEType
	if contentType == "" {
		contentType = mime.TypeByExtension(filepath.Ext(mediaPath))
	}
	return MediaFile{Path: mediaPath, ContentType: contentType, Size: info.Size()}, contextError(ctx)
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

func (s *MediaService) PendingRepairs(ctx context.Context, limit int) ([]models.MediaRepairCandidate, error) {
	if err := contextError(ctx); err != nil {
		return nil, err
	}
	repository, ok := s.repository.(MediaStateRepository)
	if !ok {
		return nil, errors.New("media state repository is not configured")
	}
	return repository.ListMissingMedia(limit)
}

func (s *MediaService) Repair(ctx context.Context, candidate models.MediaRepairCandidate) (MediaFile, error) {
	if err := contextError(ctx); err != nil {
		return MediaFile{}, err
	}
	repository, ok := s.repository.(MediaStateRepository)
	if !ok || s.remote == nil {
		return MediaFile{}, errors.New("media repair is not configured")
	}
	if candidate.OwnerType == "comment" && strings.TrimSpace(candidate.RemoteID) == "" {
		err := errors.New("comment media is missing its remote id")
		_ = repository.MarkMediaFailed(candidate.ID, err.Error())
		return MediaFile{}, err
	}
	var data []byte
	var err error
	if candidate.Variant == "thumbnail" {
		data, err = s.remote.DownloadThumbnail(ctx, candidate.RemoteID, candidate.PID)
	} else {
		data, err = s.remote.DownloadImage(ctx, candidate.RemoteID, candidate.PID)
	}
	if err != nil {
		_ = repository.MarkMediaFailed(candidate.ID, err.Error())
		return MediaFile{}, err
	}
	contentType := http.DetectContentType(data)
	if !strings.HasPrefix(contentType, "image/") {
		err = errors.New("downloaded media is not an image")
		_ = repository.MarkMediaFailed(candidate.ID, err.Error())
		return MediaFile{}, err
	}
	digest := sha256.Sum256(data)
	contentHash := hex.EncodeToString(digest[:])
	relativePath := filepath.Join("media", "objects", contentHash+extensionForContentType(contentType))
	absolutePath := filepath.Join(s.dataDir, relativePath)
	if err := writeMediaObject(absolutePath, data); err != nil {
		_ = repository.MarkMediaFailed(candidate.ID, err.Error())
		return MediaFile{}, err
	}
	if err := repository.MarkMediaAvailable(candidate.ID, contentHash, filepath.ToSlash(relativePath), contentType, int64(len(data))); err != nil {
		return MediaFile{}, err
	}
	return MediaFile{Path: absolutePath, ContentType: contentType, Size: int64(len(data))}, nil
}

func writeMediaObject(path string, data []byte) error {
	if existing, err := os.ReadFile(path); err == nil {
		if string(existing) != string(data) {
			return errors.New("existing media object content does not match")
		}
		return nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	temporary, err := os.CreateTemp(filepath.Dir(path), ".repair-media-*")
	if err != nil {
		return err
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)
	if _, err := temporary.Write(data); err != nil {
		temporary.Close()
		return err
	}
	if err := temporary.Close(); err != nil {
		return err
	}
	return os.Rename(temporaryPath, path)
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
