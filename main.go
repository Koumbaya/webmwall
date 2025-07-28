package main

import (
	"embed"
	"encoding/json"
	"flag"
	"log"
	"math/rand"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

//go:embed static/*
var staticFiles embed.FS

var videoDir string
var videoFiles []string // Global slice to hold randomized video list

func openBrowser(url string) {
	cmd := exec.Command("open", url)
	cmd.Start()
}

func main() {
	flag.StringVar(&videoDir, "dir", ".", "Directory containing video files")
	flag.StringVar(&videoDir, "d", ".", "Directory containing video files (shorthand)")
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
			http.NotFound(w, r)
		}
	})

	// Serve video files from the specified directory
	http.Handle("/videos/", http.StripPrefix("/videos/", http.FileServer(http.Dir(videoDir))))

	// API endpoint to list videos
	http.HandleFunc("/api/videos", handleVideoList)

	log.Printf("Serving videos from: %s", videoDir)
	log.Println("Serving on http://localhost:8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}

func initVideoList() {
	entries, err := os.ReadDir(videoDir)
	if err != nil {
		log.Fatalf("Failed to read videos directory: %v", err)
	}

	for _, entry := range entries {
		ext := strings.ToLower(filepath.Ext(entry.Name()))
		if !entry.IsDir() && (ext == ".webm" || ext == ".mp4" || ext == ".gif" || ext == ".jpg" || ext == ".jpeg" || ext == ".png" || ext == ".bmp") {
			videoFiles = append(videoFiles, "/videos/"+entry.Name())
		}
	}

	rand.Shuffle(len(videoFiles), func(i, j int) {
		videoFiles[i], videoFiles[j] = videoFiles[j], videoFiles[i]
	})

	log.Printf("Loaded %d video files in randomized order", len(videoFiles))
}

func handleVideoList(w http.ResponseWriter, r *http.Request) {
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
