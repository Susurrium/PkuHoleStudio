package service

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

type fakeMediaRemote struct {
	data []byte
	err  error
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

func selectNotExist(svc *MediaService, t *testing.T) error {
	t.Helper()
	_, err := svc.Locate(context.Background(), MediaRequest{ID: "999"})
	return err
}
