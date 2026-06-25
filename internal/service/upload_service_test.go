package service

import (
	"bytes"
	"context"
	"errors"
	"io"
	"mime/multipart"
	"strings"
	"testing"
)

type fakeFile struct{ *bytes.Reader }

func (f *fakeFile) Close() error                            { return nil }
func (f *fakeFile) ReadAt(p []byte, off int64) (int, error) { return f.Reader.ReadAt(p, off) }

func newFakeFile(data string) multipart.File {
	return &fakeFile{bytes.NewReader([]byte(data))}
}

type stubS3Uploader struct {
	putErr     error
	deleteErr  error
	gotKey     string
	deletedKey string
}

func (s *stubS3Uploader) PutObject(_ context.Context, _, key string, _ io.Reader, _ string) error {
	s.gotKey = key
	return s.putErr
}

func (s *stubS3Uploader) PutObjectEncrypted(_ context.Context, _, key string, _ io.Reader, _ string) error {
	s.gotKey = key
	return s.putErr
}

func (s *stubS3Uploader) DeleteObject(_ context.Context, _, key string) error {
	s.deletedKey = key
	return s.deleteErr
}

func TestUploadMatchImage_Success(t *testing.T) {
	stub := &stubS3Uploader{}
	svc := NewUploadService(stub, "smatch-matches")

	file := newFakeFile("fake image data")
	header := &multipart.FileHeader{Filename: "photo.jpg"}

	key, err := svc.UploadMatchImage(context.Background(), file, header)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if !strings.HasPrefix(key, "matches/") {
		t.Errorf("expected key to start with 'matches/', got: %s", key)
	}

	if !strings.HasPrefix(stub.gotKey, "matches/") {
		t.Errorf("unexpected key: %s", stub.gotKey)
	}
}

func TestUploadMatchImage_InvalidExtension(t *testing.T) {
	stub := &stubS3Uploader{}
	svc := NewUploadService(stub, "smatch-matches")

	file := newFakeFile("fake pdf data")
	header := &multipart.FileHeader{Filename: "document.pdf"}

	_, err := svc.UploadMatchImage(context.Background(), file, header)
	if err == nil {
		t.Fatal("expected an error for invalid extension, got nil")
	}
}

func TestUploadMatchImage_S3Error(t *testing.T) {
	stub := &stubS3Uploader{putErr: errors.New("s3 down")}
	svc := NewUploadService(stub, "smatch-matches")

	file := newFakeFile("fake image data")
	header := &multipart.FileHeader{Filename: "photo.png"}

	_, err := svc.UploadMatchImage(context.Background(), file, header)
	if err == nil {
		t.Fatal("expected an error when S3 fails, got nil")
	}
}

func TestUploadProfilePhoto_Success(t *testing.T) {
	stub := &stubS3Uploader{}
	svc := NewUploadService(stub, "smatch-profiles")

	file := newFakeFile("fake image data")
	header := &multipart.FileHeader{Filename: "avatar.jpg"}

	key, err := svc.UploadProfilePhoto(context.Background(), "user-123", "", file, header)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if !strings.HasPrefix(key, "profile/user-123/") {
		t.Errorf("expected key to start with 'profile/user-123/', got: %s", key)
	}
}

func TestUploadProfilePhoto_InvalidExtension(t *testing.T) {
	stub := &stubS3Uploader{}
	svc := NewUploadService(stub, "smatch-profiles")

	file := newFakeFile("fake pdf data")
	header := &multipart.FileHeader{Filename: "doc.pdf"}

	_, err := svc.UploadProfilePhoto(context.Background(), "user-123", "", file, header)
	if err == nil {
		t.Fatal("expected an error for invalid extension, got nil")
	}
}

func TestUploadProfilePhoto_DeletesOldPhoto(t *testing.T) {
	stub := &stubS3Uploader{}
	svc := NewUploadService(stub, "smatch-profiles")

	file := newFakeFile("fake image data")
	header := &multipart.FileHeader{Filename: "avatar.jpg"}

	_, err := svc.UploadProfilePhoto(context.Background(), "user-123", "profile/user-123/old-photo.jpg", file, header)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if stub.deletedKey != "profile/user-123/old-photo.jpg" {
		t.Errorf("expected old key to be deleted, got deletedKey=%q", stub.deletedKey)
	}
}

func TestUploadProfilePhoto_S3Error(t *testing.T) {
	stub := &stubS3Uploader{putErr: errors.New("s3 down")}
	svc := NewUploadService(stub, "smatch-profiles")

	file := newFakeFile("fake image data")
	header := &multipart.FileHeader{Filename: "avatar.png"}

	_, err := svc.UploadProfilePhoto(context.Background(), "user-123", "", file, header)
	if err == nil {
		t.Fatal("expected an error when S3 fails, got nil")
	}
}
