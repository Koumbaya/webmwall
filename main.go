package main

import (
	"embed"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

const autoShutdownDelay = 60 * time.Second

//go:embed static/*
var staticFiles embed.FS

var videoDir string
var autoShutdown bool
var videoFiles []string // Global slice to hold randomized video list

func openBrowser(url string) {
	var cmd string
	var args []string

	switch runtime.GOOS {
	case "linux":
		cmd = "xdg-open"
	case "windows":
		cmd = "rundll32"
		args = []string{"url.dll,FileProtocolHandler"}
	case "darwin":
		cmd = "open"
	}
	args = append(args, url)
	exec.Command(cmd, args...).Start()
}

func main() {
	flag.StringVar(&videoDir, "dir", ".", "Directory containing video files")
	flag.StringVar(&videoDir, "d", ".", "Directory containing video files (shorthand)")
	flag.BoolVar(&autoShutdown, "auto-shutdown", true, "Automatically shutdown the server after 2 minutes without any request")
	flag.Parse()

	initVideoList()

	// Open the browser to localhost
	openBrowser("http://localhost:8080")

	// Serve the index.html directly at root
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			data, err := staticFiles.ReadFile("static/index.html")
			if err != nil {
				http.Error(w, "Failed to read index.html", http.StatusInternalServerError)
				return
			}
			w.Header().Set("Content-Type", "text/html")
			w.Write(data)
		} else {
			// Check if it's a static file (PNG icons)
			if strings.HasSuffix(r.URL.Path, ".png") || strings.HasSuffix(r.URL.Path, ".svg") {
				filePath := "static" + r.URL.Path
				data, err := staticFiles.ReadFile(filePath)
				if err != nil {
					http.NotFound(w, r)
					return
				}
				if strings.HasSuffix(r.URL.Path, ".svg") {
					w.Header().Set("Content-Type", "image/svg+xml")
				} else {
					w.Header().Set("Content-Type", "image/png")
				}
				w.Write(data)
			} else {
				http.NotFound(w, r)
			}
		}
	})

	// Serve video files from the specified directory
	http.Handle("/videos/", http.StripPrefix("/videos/", http.FileServer(http.Dir(videoDir))))

	// make chan for auto off
	timeC := make(chan time.Time)
	defer close(timeC)
	if autoShutdown {
		go shutdowner(timeC)
	}

	// API endpoint to list videos
	http.HandleFunc("/api/videos", makeHandleVideosList(autoShutdown, timeC))

	// API endpoint to delete files
	http.HandleFunc("/api/delete", handleDeleteFile)

	// API endpoint to get file counts by type
	http.HandleFunc("/api/stats", handleStats)

	// API endpoint to exit the application
	http.HandleFunc("/api/exit", handleExit)

	log.Printf("Serving videos from: %s", videoDir)
	log.Println("Serving on http://localhost:8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}

func shutdowner(timeC <-chan time.Time) {
	ticker := time.NewTicker(autoShutdownDelay)
	lastTime := time.Now().UTC()
	for {
		select {
		case <-ticker.C:
			if time.Since(lastTime) > autoShutdownDelay {
				log.Println("No request for some time. Shutting down...")
				os.Exit(0)
			}
		case t := <-timeC:
			lastTime = t
		}
	}
}

func initVideoList() {
	entries, err := os.ReadDir(videoDir)
	if err != nil {
		log.Fatalf("Failed to read videos directory: %v", err)
	}

	for _, entry := range entries {
		ext := strings.ToLower(filepath.Ext(entry.Name()))
		if !entry.IsDir() && (ext == ".webm" || ext == ".mp4" || ext == ".gif" || ext == ".jpg" || ext == ".webp" || ext == ".jpeg" || ext == ".png" || ext == ".bmp") {
			videoFiles = append(videoFiles, "/videos/"+entry.Name())
		}
	}

	rand.Shuffle(len(videoFiles), func(i, j int) {
		videoFiles[i], videoFiles[j] = videoFiles[j], videoFiles[i]
	})

	log.Printf("Loaded %d video files in randomized order", len(videoFiles))
}

func makeHandleVideosList(autoOff bool, timeC chan<- time.Time) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		if autoOff {
			timeC <- time.Now().UTC()
		}
		if len(videoFiles) == 0 {
			json.NewEncoder(w).Encode([]string{})
			return
		}

		typesParam := r.URL.Query().Get("types")
		var allowedTypes map[string]bool
		if typesParam != "" {
			allowedTypes = make(map[string]bool)
			for _, t := range strings.Split(typesParam, ",") {
				allowedTypes["."+strings.TrimSpace(t)] = true
			}
		}

		var filteredVideos []string
		if allowedTypes != nil {
			for _, video := range videoFiles {
				ext := strings.ToLower(filepath.Ext(video))
				if allowedTypes[ext] {
					filteredVideos = append(filteredVideos, video)
				}
			}
		} else {
			filteredVideos = videoFiles
		}

		total := len(filteredVideos)
		if total == 0 {
			json.NewEncoder(w).Encode([]string{})
			return
		}

		randomIndex := rand.Intn(total)
		result := []string{filteredVideos[randomIndex]}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(result)
	}
}

func handleDeleteFile(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		http.Error(w, "Invalid request method", http.StatusMethodNotAllowed)
		return
	}

	filePath := r.URL.Query().Get("path")
	if filePath == "" {
		http.Error(w, "File path is required", http.StatusBadRequest)
		return
	}

	// Convert URL path to filesystem path
	// filePath will be like "/videos/filename.ext"
	if !strings.HasPrefix(filePath, "/videos/") {
		http.Error(w, "Invalid file path", http.StatusBadRequest)
		return
	}

	// Extract filename and build actual filesystem path
	filename := strings.TrimPrefix(filePath, "/videos/")
	actualPath := filepath.Join(videoDir, filename)

	// Sanitize the path to prevent directory traversal
	cleanPath := filepath.Clean(actualPath)

	// Check if file exists
	if _, err := os.Stat(cleanPath); os.IsNotExist(err) {
		http.Error(w, "File not found", http.StatusNotFound)
		return
	}

	// Delete the file
	err := os.Remove(cleanPath)
	if err != nil {
		fmt.Printf("Failed to delete file %s: %v", cleanPath, err)
		http.Error(w, fmt.Sprintf("Failed to delete file %v", err), http.StatusInternalServerError)
		return
	}

	log.Printf("Deleted file: %s", cleanPath)

	// Remove from video files list
	for i, videoFile := range videoFiles {
		if videoFile == filePath {
			videoFiles = append(videoFiles[:i], videoFiles[i+1:]...)
			break
		}
	}

	w.WriteHeader(http.StatusNoContent)
}

func handleStats(w http.ResponseWriter, r *http.Request) {
	type statsResponse struct {
		Gif    int `json:"gif"`
		Webm   int `json:"webm"`
		Mp4    int `json:"mp4"`
		Images int `json:"images"`
		Total  int `json:"total"`
	}

	imageExts := map[string]bool{".jpg": true, ".jpeg": true, ".png": true, ".bmp": true, ".webp": true}
	stats := statsResponse{}
	for _, f := range videoFiles {
		ext := strings.ToLower(filepath.Ext(f))
		switch ext {
		case ".gif":
			stats.Gif++
		case ".webm":
			stats.Webm++
		case ".mp4":
			stats.Mp4++
		default:
			if imageExts[ext] {
				stats.Images++
			}
		}
	}
	stats.Total = stats.Gif + stats.Webm + stats.Mp4 + stats.Images

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(stats)
}

func handleExit(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Invalid request method", http.StatusMethodNotAllowed)
		return
	}

	log.Println("Exit requested from frontend. Shutting down...")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("Goodbye"))

	if f, ok := w.(http.Flusher); ok {
		f.Flush()
	}

	os.Exit(0)
}
