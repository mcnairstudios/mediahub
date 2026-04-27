package httputil

import (
	"archive/zip"
	"compress/gzip"
	"fmt"
	"io"
	"net/url"
	"os"
	"path"
	"strings"
)

type gzipReadCloser struct {
	gr   *gzip.Reader
	body io.ReadCloser
}

func (g *gzipReadCloser) Read(p []byte) (int, error) { return g.gr.Read(p) }
func (g *gzipReadCloser) Close() error {
	g.gr.Close()
	return g.body.Close()
}

type zipReadCloser struct {
	f       io.ReadCloser
	zr      *zip.ReadCloser
	tmpPath string
}

func (z *zipReadCloser) Read(p []byte) (int, error) { return z.f.Read(p) }
func (z *zipReadCloser) Close() error {
	z.f.Close()
	z.zr.Close()
	os.Remove(z.tmpPath)
	return nil
}

func DecompressReader(body io.ReadCloser, rawURL string) (io.ReadCloser, error) {
	ext := urlExtension(rawURL)

	switch ext {
	case ".gz":
		gr, err := gzip.NewReader(body)
		if err != nil {
			body.Close()
			return nil, fmt.Errorf("creating gzip reader: %w", err)
		}
		return &gzipReadCloser{gr: gr, body: body}, nil

	case ".zip":
		tmp, err := os.CreateTemp("", "mediahub-*.zip")
		if err != nil {
			body.Close()
			return nil, fmt.Errorf("creating temp file: %w", err)
		}
		tmpPath := tmp.Name()

		if _, err := io.Copy(tmp, body); err != nil {
			tmp.Close()
			body.Close()
			os.Remove(tmpPath)
			return nil, fmt.Errorf("buffering zip data: %w", err)
		}
		tmp.Close()
		body.Close()

		zr, err := zip.OpenReader(tmpPath)
		if err != nil {
			os.Remove(tmpPath)
			return nil, fmt.Errorf("opening zip archive: %w", err)
		}
		if len(zr.File) == 0 {
			zr.Close()
			os.Remove(tmpPath)
			return nil, fmt.Errorf("zip archive is empty")
		}

		f, err := zr.File[0].Open()
		if err != nil {
			zr.Close()
			os.Remove(tmpPath)
			return nil, fmt.Errorf("opening zip entry: %w", err)
		}
		return &zipReadCloser{f: f, zr: zr, tmpPath: tmpPath}, nil

	default:
		return body, nil
	}
}

func urlExtension(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		return ""
	}
	return strings.ToLower(path.Ext(u.Path))
}
