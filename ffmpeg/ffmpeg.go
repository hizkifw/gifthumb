package ffmpeg

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/hizkifw/gifthumb/config"
	"golang.org/x/sync/semaphore"
)

type Ffmpeg struct {
	cfg *config.Config

	// Create a map to keep track of which videos are being processed
	processing     map[string]struct{}
	processingLock sync.Mutex

	// Limit the number of concurrent processes
	processingSem *semaphore.Weighted
}

func New(cfg *config.Config) *Ffmpeg {
	return &Ffmpeg{
		cfg:           cfg,
		processing:    make(map[string]struct{}),
		processingSem: semaphore.NewWeighted(int64(cfg.MaxProcesses)),
	}
}

// Get the duration of a video from a URL
func (f *Ffmpeg) getVideoDuration(ctx context.Context, url string) (float64, error) {
	f.processingSem.Acquire(ctx, 1)
	defer f.processingSem.Release(1)

	cmd := exec.CommandContext(
		ctx,
		"ffprobe", "-v", "error",
		"-show_entries", "format=duration",
		"-of", "default=noprint_wrappers=1:nokey=1",
		url,
	)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return 0, fmt.Errorf("failed to run ffprobe: %w", err)
	}

	parsed, err := strconv.ParseFloat(strings.TrimSpace(string(output)), 64)
	if err != nil {
		return 0, fmt.Errorf("failed to parse ffprobe output: %w", err)
	}

	return parsed, nil
}

// Get a snapshot of a video at a given timestamp
func (f *Ffmpeg) getSnapshotAt(ctx context.Context, url string, timestamp float64, outfile string) error {
	f.processingSem.Acquire(ctx, 1)
	defer f.processingSem.Release(1)

	cmd := exec.CommandContext(
		ctx,
		"ffmpeg", "-y",
		"-ss", fmt.Sprintf("%f", timestamp),
		"-i", url,
		"-vf", fmt.Sprintf("scale=-2:%d", f.cfg.ThumbHeight),
		"-vframes", "1",
		outfile,
	)

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to get snapshot: %w: %s", err, string(output))
	}

	return nil
}

func (f *Ffmpeg) makeGif(ctx context.Context, pattern string, outfile string) error {
	f.processingSem.Acquire(ctx, 1)
	defer f.processingSem.Release(1)

	// Create the gif
	cmd := exec.CommandContext(
		ctx,
		"ffmpeg", "-y",
		"-framerate", fmt.Sprintf("%d", f.cfg.GifFramerate),
		"-i", pattern,
		outfile,
	)

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to create gif: %w: %s", err, string(output))
	}

	return nil
}

func (f *Ffmpeg) shouldProcess(videoUrl string) bool {
	f.processingLock.Lock()
	defer f.processingLock.Unlock()

	_, ok := f.processing[videoUrl]
	if ok {
		return false
	}

	f.processing[videoUrl] = struct{}{}
	return true
}

func (f *Ffmpeg) doneProcessing(videoUrl string) {
	f.processingLock.Lock()
	defer f.processingLock.Unlock()

	delete(f.processing, videoUrl)
}

func (f *Ffmpeg) MakeGifThumbnail(ctx context.Context, videoUrl string, outputFile string) error {
	if !f.shouldProcess(videoUrl) {
		// Wait for the video to finish processing
		for {
			select {
			case <-ctx.Done():
				// Context is done, exit the loop
				log.Println("Context is done")
				return nil
			default:
				if _, err := os.Stat(outputFile); err == nil {
					return nil
				}
				time.Sleep(1 * time.Second)
			}
		}
	}
	defer f.doneProcessing(videoUrl)

	// Create temporary directory
	workdir, err := os.MkdirTemp("", "thumb-")
	if err != nil {
		return fmt.Errorf("failed to create temp dir: %w", err)
	}
	defer os.RemoveAll(workdir)
	log.Println("workdir:", workdir)

	// Get the duration
	log.Println("getting duration...")
	duration, err := f.getVideoDuration(ctx, videoUrl)
	if err != nil {
		return fmt.Errorf("failed to get video duration: %w", err)
	}
	log.Println("duration:", duration)

	// Generate list of timestamps
	timestamps := make([]float64, f.cfg.NSnapshots)
	sliceDuration := duration / float64(f.cfg.NSnapshots)
	for i := 0; i < f.cfg.NSnapshots; i++ {
		timestamps[i] = (sliceDuration / 2) + (sliceDuration * float64(i))
	}

	// Generate snapshots
	log.Println("getting snapshots...")
	wg := sync.WaitGroup{}
	errs := make(chan error, f.cfg.NSnapshots)
	for i, timestamp := range timestamps {
		outfile := path.Join(workdir, fmt.Sprintf("thumb-%d.jpg", i))

		wg.Add(1)
		go func(timestamp float64) {
			defer wg.Done()

			// Create a new context with a timeout
			snapCtx, cancel := context.WithTimeout(ctx, 60*time.Second)
			defer cancel()

			err := f.getSnapshotAt(snapCtx, videoUrl, timestamp, outfile)
			if err != nil {
				log.Println("error getting snapshot:", err)
				errs <- err
				return
			}
			log.Println("got snapshot", outfile, "at", timestamp, "seconds")
		}(timestamp)
	}

	// Wait for snapshots to finish
	wg.Wait()
	log.Println("got all snapshots")

	// Check for errors
	select {
	case err := <-errs:
		if err != nil {
			return fmt.Errorf("failed to get snapshot: %w", err)
		}
	default:
	}
	log.Println("no errors getting snapshots")

	// Combine snapshots into a gif
	inputPattern := path.Join(workdir, "thumb-%d.jpg")
	if err = f.makeGif(ctx, inputPattern, outputFile); err != nil {
		return fmt.Errorf("failed to create gif: %w", err)
	}

	return nil
}
