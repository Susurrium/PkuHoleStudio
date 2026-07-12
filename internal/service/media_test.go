package service

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/Susurrium/PkuHoleStudio/internal/models"
)

type fakeMediaRemote struct {
	data []byte
	err  error
}

type fakeMediaRepository struct {
	row        models.Media
	candidates []models.MediaRepairCandidate
}

func (f *fakeMediaRepository) GetMediaByPID(int32) ([]models.Media, error) {
	return []models.Media{f.row}, nil
}
func (f *fakeMediaRepository) GetMediaByID(uint) (*models.Media, error) {
	copy := f.row
	return &copy, nil
}
func (f *fakeMediaRepository) ListMissingMedia(int) ([]models.MediaRepairCandidate, error) {
	return append([]models.MediaRepairCandidate(nil), f.candidates...), nil
}
func (f *fakeMediaRepository) MarkMediaAvailable(_ uint, hash, path, mimeType string, size int64) error {
	f.row.ContentHash, f.row.Path, f.row.MIMEType, f.row.Size, f.row.Status = hash, path, mimeType, size, "available"
	return nil
}
func (f *fakeMediaRepository) MarkMediaFailed(_ uint, message string) error {
	f.row.Status, f.row.LastError = "failed", message
	return nil
}

func (f fakeMediaRemote) DownloadImage(context.Context, string, int32) ([]byte, error) {
	return f.data, f.err
}

func (f fakeMediaRemote) DownloadThumbnail(context.Context, string, int32) ([]byte, error) {
	return f.data, f.err
}

func TestMediaServiceDownloadLocateAndRead(t *testing.T) {
	root := t.TempDir()
	png := []byte{0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a, 0x00}
	svc := NewMediaService(root, fakeMediaRemote{data: png})
	request := MediaRequest{ID: "42"}

	file, err := svc.Download(context.Background(), request)
	if err != nil {
		t.Fatalf("Download() error = %v", err)
	}
	if filepath.Ext(file.Path) != ".png" {
		t.Fatalf("Download() path = %q", file.Path)
	}
	data, located, err := svc.Read(context.Background(), request)
	if err != nil {
		t.Fatalf("Read() error = %v", err)
	}
	if string(data) != string(png) || located.Path != file.Path {
		t.Fatalf("Read() data/path mismatch")
	}
}

func TestMediaServiceRejectsUnsafeID(t *testing.T) {
	svc := NewMediaService(t.TempDir(), fakeMediaRemote{})
	_, err := svc.Locate(context.Background(), MediaRequest{ID: "../secret"})
	if err == nil {
		t.Fatal("Locate() accepted an unsafe media id")
	}
	if !errors.Is(selectNotExist(svc, t), os.ErrNotExist) {
		t.Fatal("missing media should report os.ErrNotExist")
	}
}

func TestMediaServiceRepairsAndLocatesDatabaseMedia(t *testing.T) {
	root := t.TempDir()
	png := []byte{0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a, 0x00}
	repository := &fakeMediaRepository{candidates: []models.MediaRepairCandidate{{
		Media: models.Media{ID: 7, OwnerType: "post", OwnerID: 123456, RemoteID: "42", Variant: "original", Status: "missing"},
		PID:   123456,
	}}}
	svc := NewMediaServiceWithRepository(root, fakeMediaRemote{data: png}, repository)
	candidates, err := svc.PendingRepairs(context.Background(), 10)
	if err != nil || len(candidates) != 1 {
		t.Fatalf("PendingRepairs() = %+v, %v", candidates, err)
	}
	file, err := svc.Repair(context.Background(), candidates[0])
	if err != nil || file.Size != int64(len(png)) || repository.row.Status != "available" {
		t.Fatalf("Repair() = %+v, %v, row=%+v", file, err, repository.row)
	}
	located, err := svc.Locate(context.Background(), MediaRequest{RecordID: 7})
	if err != nil || located.Path != file.Path {
		t.Fatalf("Locate(record) = %+v, %v", located, err)
	}
}

func selectNotExist(svc *MediaService, t *testing.T) error {
	t.Helper()
	_, err := svc.Locate(context.Background(), MediaRequest{ID: "999"})
	return err
}
