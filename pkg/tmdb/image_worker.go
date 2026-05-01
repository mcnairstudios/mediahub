package tmdb

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

type ImageWorker struct {
	store    *Store
	imageDir string
	client   *http.Client
}

func NewImageWorker(store *Store, imageDir string) *ImageWorker {
	os.MkdirAll(imageDir, 0755)
	return &ImageWorker{
		store:    store,
		imageDir: imageDir,
		client:   &http.Client{Timeout: 30 * time.Second},
	}
}

func (w *ImageWorker) Run(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		localPath, entry, err := w.store.PickImage()
		if err != nil {
			log.Printf("[tmdb-images] dequeue error: %v", err)
			select {
			case <-ctx.Done():
				return
			case <-time.After(10 * time.Second):
			}
			continue
		}

		if entry == nil {
			select {
			case <-ctx.Done():
				return
			case <-time.After(10 * time.Second):
			}
			continue
		}

		if entry.TMDBPath == "" {
			w.store.DeleteImageEntry(localPath)
			continue
		}

		fullPath := filepath.Join(w.imageDir, localPath)
		if _, err := os.Stat(fullPath); err == nil {
			w.store.DeleteImageEntry(localPath)
			continue
		}

		err = w.downloadImage(entry, fullPath)
		if err != nil {
			if isNetworkError(err) {
				log.Printf("[tmdb-images] network error downloading %s: %v (will retry)", localPath, err)
				select {
				case <-ctx.Done():
					return
				case <-time.After(5 * time.Second):
				}
				continue
			}
			log.Printf("[tmdb-images] failed to download %s: %v (removing from queue)", localPath, err)
			w.store.DeleteImageEntry(localPath)
			continue
		}

		w.store.DeleteImageEntry(localPath)

		select {
		case <-ctx.Done():
			return
		case <-time.After(100 * time.Millisecond):
		}
	}
}

func (w *ImageWorker) downloadImage(entry *ImageQueueEntry, fullPath string) error {
	url := fmt.Sprintf("https://image.tmdb.org/t/p/%s%s", entry.Size, entry.TMDBPath)

	resp, err := w.client.Get(url)
	if err != nil {
		return &networkError{err: err}
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("TMDB CDN returned %d", resp.StatusCode)
	}

	dir := filepath.Dir(fullPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("create directory: %w", err)
	}

	tmpPath := fullPath + ".tmp"
	f, err := os.Create(tmpPath)
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}

	if _, err := io.Copy(f, resp.Body); err != nil {
		f.Close()
		os.Remove(tmpPath)
		return &networkError{err: fmt.Errorf("write: %w", err)}
	}
	f.Close()

	if err := os.Rename(tmpPath, fullPath); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("rename: %w", err)
	}

	return nil
}

type networkError struct {
	err error
}

func (e *networkError) Error() string {
	return e.err.Error()
}

func isNetworkError(err error) bool {
	_, ok := err.(*networkError)
	return ok
}
