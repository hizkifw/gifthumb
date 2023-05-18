package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path"
	"strconv"
	"strings"
	"time"
)

const (
	// Number of snapshots to take
	nSnapshots = 4
	// Resize snapshot to this height
	thumbHeight = 360
	// Framerate of the gif
	gifFramerate = 1
)

func writeError(w http.ResponseWriter, err error) bool {
	if err != nil {
		log.Println("error:", err)
		w.WriteHeader(http.StatusInternalServerError)
		w.Header().Add("Content-Type", "text/plain")
		w.Write([]byte(err.Error()))
		return true
	}
	return false
}

func getVideoDuration(url string) (float64, error) {
	cmd := exec.Command("ffprobe", "-v", "error", "-show_entries", "format=duration", "-of", "default=noprint_wrappers=1:nokey=1", url)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return 0, fmt.Errorf("failed to get video duration: %w", err)
	}

	parsed, err := strconv.ParseFloat(strings.TrimSpace(string(output)), 64)
	if err != nil {
		return 0, fmt.Errorf("failed to parse video duration: %w", err)
	}

	return parsed, nil
}

func getSnapshotAt(url string, timestamp float64, outfile string) error {
	cmd := exec.Command(
		"ffmpeg", "-y",
		"-ss", fmt.Sprintf("%f", timestamp),
		"-i", url,
		"-vf", fmt.Sprintf("scale=-2:%d", thumbHeight),
		"-vframes", "1",
		outfile,
	)

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to get snapshot: %w: %s", err, string(output))
	}

	return nil
}

func makeGif(pattern string, outfile string) error {
	// Create the gif
	cmd := exec.Command(
		"ffmpeg", "-y",
		"-framerate", fmt.Sprintf("%d", gifFramerate),
		"-i", pattern,
		outfile,
	)

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to create gif: %w: %s", err, string(output))
	}

	return nil
}

func main() {
	log.Println("Starting...")

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	// Health check
	// GET /healthz
	http.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Header().Add("Content-Type", "text/plain")
		w.Write([]byte("ok"))
	})

	// Process video
	// GET /thumb?url=<video_url>
	http.HandleFunc("/thumb", func(w http.ResponseWriter, r *http.Request) {
		videoUrl := r.URL.Query().Get("url")
		log.Println("videoUrl:", videoUrl)

		// Create temporary directory
		workdir, err := os.MkdirTemp("", "thumb-")
		if writeError(w, err) {
			return
		}
		defer os.RemoveAll(workdir)
		log.Println("workdir:", workdir)

		// Get the duration
		log.Println("getting duration...")
		duration, err := getVideoDuration(videoUrl)
		if writeError(w, err) {
			return
		}
		log.Println("duration:", duration)

		// Generate list of timestamps
		timestamps := make([]float64, nSnapshots)
		sliceDuration := duration / float64(nSnapshots)
		for i := 0; i < nSnapshots; i++ {
			timestamps[i] = (sliceDuration / 2) + (sliceDuration * float64(i))
		}

		// Generate snapshots
		for i, timestamp := range timestamps {
			outfile := path.Join(workdir, fmt.Sprintf("thumb-%d.jpg", i))
			err = getSnapshotAt(videoUrl, timestamp, outfile)
			if writeError(w, err) {
				return
			}
			log.Println("got snapshot", outfile, "at", timestamp, "seconds")
		}

		// Combine snapshots into a gif
		gifFile, err := os.CreateTemp(workdir, "thumb-*.gif")
		if writeError(w, err) {
			return
		}
		defer os.Remove(gifFile.Name())
		log.Println("creating gif ", gifFile.Name())

		err = makeGif(path.Join(workdir, "thumb-%d.jpg"), gifFile.Name())
		if writeError(w, err) {
			return
		}
		log.Println("created gif")

		// Serve the gif
		http.ServeContent(w, r, gifFile.Name(), time.Now(), gifFile)
	})

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		// Run ffmpeg
		cmd := exec.Command("ffmpeg", "-version")
		output, err := cmd.CombinedOutput()
		if writeError(w, err) {
			return
		}

		w.WriteHeader(http.StatusOK)
		w.Header().Add("Content-Type", "text/plain")
		w.Write(output)
	})

	log.Println("listening on", port)
	log.Fatal(http.ListenAndServe(":"+port, nil))
}
