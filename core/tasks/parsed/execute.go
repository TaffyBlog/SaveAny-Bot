package parsed

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"path"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/charmbracelet/log"
	"github.com/duke-git/lancet/v2/retry"
	"github.com/krau/SaveAny-Bot/common/utils/fsutil"
	"github.com/krau/SaveAny-Bot/common/utils/ioutil"
	"github.com/krau/SaveAny-Bot/config"
	"github.com/krau/SaveAny-Bot/pkg/enums/ctxkey"
	"github.com/krau/SaveAny-Bot/pkg/parser"
	"github.com/krau/SaveAny-Bot/pkg/taskevent"
	"golang.org/x/sync/errgroup"
)

func (t *Task) Execute(ctx context.Context) error {
	logger := log.FromContext(ctx)
	logger.Infof("Starting Parsed item task %s", t.item.Title)
	if t.progress != nil {
		t.progress.OnStart(ctx, t)
	}
	eg, gctx := errgroup.WithContext(ctx)
	eg.SetLimit(config.C().Workers)
	for _, resource := range t.item.Resources {
		eg.Go(func() error {
			resourceID := resource.ID()
			t.processingMu.Lock()
			if t.processing[resourceID] != nil {
				t.processingMu.Unlock()
				return fmt.Errorf("resource %s is already being processed", resourceID)
			}
			t.processing[resourceID] = &resource
			t.processingMu.Unlock()
			defer func() {
				t.processingMu.Lock()
				delete(t.processing, resourceID)
				t.processingMu.Unlock()
			}()
			err := t.processResource(gctx, resource)
			if errors.Is(err, context.Canceled) {
				logger.Debug("Resource processing canceled")
				return err
			}
			if err != nil {
				logger.Errorf("Error processing resource %s: %v", resource.URL, err)
				return fmt.Errorf("failed to process resource %s: %w", resource.URL, err)
			}
			t.downloaded.Add(1)
			return nil
		})
	}
	err := eg.Wait()
	if err != nil {
		logger.Errorf("Error during Parsed item task execution: %v", err)
	} else {
		logger.Infof("Parsed item task %s completed successfully", t.item.Title)
	}
	if t.progress != nil {
		t.progress.OnDone(ctx, t, err)
	}
	return err
}

func (t *Task) processResource(ctx context.Context, resource parser.Resource) error {
	logger := log.FromContext(ctx)
	// counted: bytes of this resource already reflected in the task-level
	// downloaded-bytes counter; used to keep progress correct when resuming.
	var counted int64
	newRequest := func(offset int64) (*http.Request, error) {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, resource.URL, nil)
		if err != nil {
			return nil, err
		}
		if resource.Headers != nil {
			for k, v := range resource.Headers {
				req.Header.Set(k, v)
			}
		}
		if offset > 0 {
			req.Header.Set("Range", fmt.Sprintf("bytes=%d-", offset))
		}
		return req, nil
	}
	err := retry.Retry(func() error {
		if t.stream {
			// Stream mode: no local cache file, pipe the body straight to
			// storage (no resume support).
			req, err := newRequest(0)
			if err != nil {
				return err
			}
			resp, err := t.httpClient.Do(req)
			if err != nil {
				return fmt.Errorf("failed to download resource %s: %w", resource.URL, err)
			}
			defer resp.Body.Close()
			if resp.StatusCode != http.StatusOK {
				return fmt.Errorf("failed to download resource %s: %s", resource.URL, resp.Status)
			}
			ctx = context.WithValue(ctx, ctxkey.ContentLength, func() int64 {
				if resource.Size > 0 {
					return resource.Size
				}
				return resp.ContentLength
			}())
			return t.Stor.Save(ctx, resp.Body, path.Join(t.StorPath, resource.Filename))
		}
		cachePath := filepath.Join(config.C().Temp.BasePath,
			fmt.Sprintf("resource_%s_%s", t.ID, resource.Filename))
		cacheFile, resumeOffset, openErr := fsutil.OpenResumeFile(cachePath)
		if openErr != nil {
			return fmt.Errorf("failed to open cache file for resource %s: %w", resource.URL, openErr)
		}
		// keepPartial: keep the partial file on disk so the next retry can
		// resume from it instead of starting over.
		keepPartial := false
		defer func() {
			if keepPartial {
				cacheFile.Close()
				return
			}
			if err := cacheFile.CloseAndRemove(); err != nil {
				logger.Errorf("Failed to close and remove cache file: %v", err)
			}
		}()
		if resumeOffset > 0 {
			logger.Infof("Resuming download of %s from byte %d", resource.URL, resumeOffset)
			t.downloadedBytes.Add(resumeOffset - counted)
			counted = resumeOffset
		}

		req, err := newRequest(resumeOffset)
		if err != nil {
			return err
		}
		resp, err := t.httpClient.Do(req)
		if err != nil {
			keepPartial = true
			return fmt.Errorf("failed to download resource %s: %w", resource.URL, err)
		}
		defer resp.Body.Close()

		// Determine the expected full size of the resource so we can detect
		// truncated downloads (some proxies close the connection cleanly
		// before the body is complete, which io.Copy cannot tell from a
		// plain EOF).
		var expectedTotal int64
		if cr := resp.Header.Get("Content-Range"); cr != "" {
			if i := strings.LastIndex(cr, "/"); i >= 0 {
				if total, perr := strconv.ParseInt(cr[i+1:], 10, 64); perr == nil && total > 0 {
					expectedTotal = total
				}
			}
		}
		if expectedTotal == 0 && resp.ContentLength > 0 {
			expectedTotal = resumeOffset + resp.ContentLength
		}
		if expectedTotal == 0 && resource.Size > 0 {
			expectedTotal = resource.Size
		}

		saveFile := func() error {
			if _, err := cacheFile.Seek(0, io.SeekStart); err != nil {
				return fmt.Errorf("failed to seek cache file for resource %s: %w", resource.URL, err)
			}
			if err := t.Stor.Save(ctx, cacheFile, path.Join(t.StorPath, resource.Filename)); err != nil {
				keepPartial = true
				return fmt.Errorf("failed to save resource %s to storage: %w", resource.URL, err)
			}
			return nil
		}

		switch {
		case resumeOffset > 0 && resp.StatusCode == http.StatusRequestedRangeNotSatisfiable:
			// The resource was fully downloaded in a previous attempt (e.g.
			// only the storage save failed). Skip the download and save it.
			logger.Infof("Resource %s already fully downloaded, skipping download", resource.URL)
			resp.Body.Close()
			if expectedTotal > 0 {
				if fi, statErr := cacheFile.Stat(); statErr == nil && fi.Size() != expectedTotal {
					keepPartial = true
					return fmt.Errorf("download of %s incomplete: got %d of %d bytes", resource.URL, fi.Size(), expectedTotal)
				}
			}
			return saveFile()
		case resumeOffset > 0 && resp.StatusCode == http.StatusOK:
			// Server does not support ranges: discard the partial file and
			// restart the download from scratch.
			logger.Warnf("Server ignored the Range header for %s, restarting from scratch", resource.URL)
			resp.Body.Close()
			if _, err := cacheFile.Seek(0, io.SeekStart); err != nil {
				return fmt.Errorf("failed to seek cache file for resource %s: %w", resource.URL, err)
			}
			if err := cacheFile.Truncate(0); err != nil {
				return fmt.Errorf("failed to truncate cache file for resource %s: %w", resource.URL, err)
			}
			t.downloadedBytes.Add(-counted)
			counted = 0
			resumeOffset = 0
		case resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusPartialContent:
			keepPartial = true
			return fmt.Errorf("failed to download resource %s: %s", resource.URL, resp.Status)
		}

		ctx = context.WithValue(ctx, ctxkey.ContentLength, func() int64 {
			if resource.Size > 0 {
				return resource.Size
			}
			return resumeOffset + resp.ContentLength
		}())
		wr := ioutil.NewProgressWriter(cacheFile, func(n int) {
			counted += int64(n)
			downloaded := t.downloadedBytes.Add(int64(n))
			if t.progress != nil {
				t.progress.OnProgress(ctx, t)
			}
			taskevent.Emit(ctx, taskevent.Event{
				TaskID:          t.ID,
				Phase:           taskevent.PhaseProgress,
				TotalBytes:      t.totalBytes,
				DownloadedBytes: downloaded,
			})
		})

		copyResultCh := make(chan error, 1)
		go func() {
			_, err := io.Copy(wr, resp.Body)
			copyResultCh <- err
		}()
		select {
		case err := <-copyResultCh:
			if err != nil {
				keepPartial = true
				// counted already reflects the bytes the writer added, so the
				// task-level counter stays consistent for the next retry.
				return fmt.Errorf("failed to copy resource %s to cache file: %w", resource.URL, err)
			}
			// The body read hit EOF; make sure the file really is complete
			// before handing it to storage, otherwise treat it as a
			// truncation and let the retry resume from where it stopped.
			if expectedTotal > 0 {
				if fi, statErr := cacheFile.Stat(); statErr == nil && fi.Size() != expectedTotal {
					keepPartial = true
					return fmt.Errorf("download of %s truncated: got %d of %d bytes", resource.URL, fi.Size(), expectedTotal)
				}
			}
		case <-ctx.Done():
			return ctx.Err()
		}
		return saveFile()
	}, retry.Context(ctx), retry.RetryTimes(uint(config.C().Retry)))
	if ctx.Err() != nil {
		return ctx.Err()
	}
	return err
}
