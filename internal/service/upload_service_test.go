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
	putErr error
	gotKey string
}

func (s *stubS3Uploader) PutObject(_ context.Context, _, key string, _ io.Reader, _ string) error {
	s.gotKey = key
	return s.putErr
}

func (s *stubS3Uploader) PutObjectEncrypted(_ context.Context, _, key string, _ io.Reader, _ string) error {
	s.gotKey = key
	return s.putErr
}

func TestUploadMatchImage_Success(t *testing.T) {
	stub := &stubS3Uploader{}
	svc := NewUploadService(stub, "smatch-matches")

	file := newFakeFile("fake image data")
	header := &multipart.FileHeader{Filename: "photo.jpg"}

	url, err := svc.UploadMatchImage(context.Background(), file, header)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if !strings.HasPrefix(url, "https://smatch-matches.s3.amazonaws.com/matches/") {
		t.Errorf("unexpected URL: %s", url)
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
