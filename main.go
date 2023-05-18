package main

import (
	"crypto/sha256"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"path"

	"github.com/hizkifw/gifthumb/config"
	"github.com/hizkifw/gifthumb/ffmpeg"
)

func HttpError(w http.ResponseWriter, status int, err error) {
	w.WriteHeader(status)
	w.Header().Add("Content-Type", "text/plain")
	w.Write([]byte(fmt.Sprintf("%d %s", status, http.StatusText(status))))

	if err != nil {
		w.Write([]byte("\n\n"))
		w.Write([]byte(err.Error()))
	}
}

func main() {
	log.Println("Starting...")
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	cfg, err := config.GetConfig()
	if err != nil {
		log.Fatalf("failed to get config: %v\n", err)
	}

	ff := ffmpeg.New(cfg)

	// Ensure cache directory exists
	if err := os.MkdirAll(cfg.CacheDir, 0755); err != nil {
		log.Fatalf("failed to create cache directory %s: %v\n", cfg.CacheDir, err)
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
		videoUrl, err := url.Parse(r.URL.Query().Get("url"))
		if err != nil {
			HttpError(w, http.StatusBadRequest, fmt.Errorf("invalid url: %w", err))
			return
		}

		// Ensure URL is allowed
		if !cfg.IsHostAllowed(videoUrl.Host) {
			HttpError(w, http.StatusForbidden, fmt.Errorf("host not allowed: %s", videoUrl.Host))
			return
		}

		// Create a hash of the video url
		urlHash := sha256.Sum256([]byte(videoUrl.String()))
		gifFileName := path.Join(cfg.CacheDir, fmt.Sprintf("%x.gif", urlHash))

		// Check if the gif already exists
		if _, err := os.Stat(gifFileName); err == nil {
			// Gif exists, serve it
			log.Println("serving cached gif", gifFileName)
			http.ServeFile(w, r, gifFileName)
			return
		}

		// Gif doesn't exist, create it
		log.Println("creating gif", gifFileName)
		if err := ff.MakeGifThumbnail(r.Context(), videoUrl.String(), gifFileName); err != nil {
			log.Println("error creating gif:", err)
			w.WriteHeader(http.StatusInternalServerError)
			w.Header().Add("Content-Type", "text/plain")
			w.Write([]byte("error creating gif"))
			return
		}

		// Serve the gif
		http.ServeFile(w, r, gifFileName)
	})

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Header().Add("Content-Type", "text/plain")
		w.Write([]byte(`
       _  __ _   _                     _     
      (_)/ _| | | |                   | |    
  __ _ _| |_| |_| |__  _   _ _ __ ___ | |__  
 / _' | |  _| __| '_ \| | | | '_ ' _ \| '_ \ 
| (_| | | | | |_| | | | |_| | | | | | | |_) |
 \__, |_|_|  \__|_| |_|\__,_|_| |_| |_|_.__/ 
  __/ |                                      
 |___/                                       
`))
	})

	log.Println("listening on", port)
	log.Fatal(http.ListenAndServe(":"+port, nil))
}
