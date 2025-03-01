package main

import (
    "os"
    "errors"
    "fmt"
    "bytes"
    "os/exec"
	"encoding/json"
	"net/http"

	"github.com/bootdotdev/learn-file-storage-s3-golang-starter/internal/auth"
	"github.com/bootdotdev/learn-file-storage-s3-golang-starter/internal/database"
	"github.com/google/uuid"
)

func (cfg *apiConfig) handlerVideoMetaCreate(w http.ResponseWriter, r *http.Request) {
	type parameters struct {
		database.CreateVideoParams
	}

	token, err := auth.GetBearerToken(r.Header)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "Couldn't find JWT", err)
		return
	}
	userID, err := auth.ValidateJWT(token, cfg.jwtSecret)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "Couldn't validate JWT", err)
		return
	}

	decoder := json.NewDecoder(r.Body)
	params := parameters{}
	err = decoder.Decode(&params)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Couldn't decode parameters", err)
		return
	}
	params.UserID = userID

	video, err := cfg.db.CreateVideo(params.CreateVideoParams)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Couldn't create video", err)
		return
	}

	respondWithJSON(w, http.StatusCreated, video)
}

func (cfg *apiConfig) handlerVideoMetaDelete(w http.ResponseWriter, r *http.Request) {
	videoIDString := r.PathValue("videoID")
	videoID, err := uuid.Parse(videoIDString)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid ID", err)
		return
	}

	token, err := auth.GetBearerToken(r.Header)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "Couldn't find JWT", err)
		return
	}
	userID, err := auth.ValidateJWT(token, cfg.jwtSecret)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "Couldn't validate JWT", err)
		return
	}

	video, err := cfg.db.GetVideo(videoID)
	if err != nil {
		respondWithError(w, http.StatusNotFound, "Couldn't get video", err)
		return
	}
	if video.UserID != userID {
		respondWithError(w, http.StatusForbidden, "You can't delete this video", err)
		return
	}

	err = cfg.db.DeleteVideo(videoID)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Couldn't delete video", err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (cfg *apiConfig) handlerVideoGet(w http.ResponseWriter, r *http.Request) {
	videoIDString := r.PathValue("videoID")
	videoID, err := uuid.Parse(videoIDString)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid video ID", err)
		return
	}

	video, err := cfg.db.GetVideo(videoID)
	if err != nil {
		respondWithError(w, http.StatusNotFound, "Couldn't get video", err)
		return
	}

	respondWithJSON(w, http.StatusOK, video)
}

func (cfg *apiConfig) handlerVideosRetrieve(w http.ResponseWriter, r *http.Request) {
	token, err := auth.GetBearerToken(r.Header)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "Couldn't find JWT", err)
		return
	}
	userID, err := auth.ValidateJWT(token, cfg.jwtSecret)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "Couldn't validate JWT", err)
		return
	}

	videos, err := cfg.db.GetVideos(userID)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Couldn't retrieve videos", err)
		return
	}

	respondWithJSON(w, http.StatusOK, videos)
}

func getVideoAspectRatio(filePath string) (string, error) {

    fmt.Println(filePath)

    if _, err := os.Stat(filePath); errors.Is(err, os.ErrNotExist) {
        return "", fmt.Errorf("File dosen't exist: %w", err)
    }
    // Secureity risk without cleaning?
    //cmd := exec.Command("ffprobe", "-v", "error", "-print_format", "json", "-show_streams", filePath)
    // Set up the ffprobe command
	cmd := exec.Command(
		"ffprobe",
		"-v", "error",           // Suppress verbose output, show only errors
		"-print_format", "json", // Output in JSON format
		"-show_streams",         // Show stream information
		filePath,               // Path to the video file
	)

    var buffer bytes.Buffer
    cmd.Stdout = &buffer

    fmt.Println("Got this far")
    err := cmd.Run()
    if err != nil {
        return "", fmt.Errorf("Failed to run command: %s", err)
    }

    // Stream represents a single stream in the ffprobe output
    type Stream struct {
        Width          int    `json:"width"`
        Height         int    `json:"height"`
    }

    // FFProbeOutput represents the top-level ffprobe JSON structure
    type FFProbeOutput struct {
        Streams []Stream `json:"streams"`
    }


	params := FFProbeOutput{}
    fmt.Println("Got this far")
    err = json.Unmarshal(buffer.Bytes(), &params)
    if err != nil {
        return "", fmt.Errorf("Failed to Unmarshal output: %w", err)
    }

    fmt.Println(params)

    aspect_ratio := float64(params.Streams[0].Width / params.Streams[0].Height)
    // margin := 0.1
    switch aspect_ratio {
    case 16/9://float64(16/9) - margin < aspect_ratio < float64(16/9) + margin:
        return "16:9", nil
    case 9/16:
        return "9:16", nil
    default:
        return "other", nil
    }
}

func processVideoForFastStart(filePath string) (string, error) {
    process_file := fmt.Sprintf("%s.processing", filePath)

    cmd := exec.Command("ffmpeg", "-i", filePath, "-c",  "copy", "-movflags", "faststart", "-f", "mp4", process_file)

    err := cmd.Run()
    if err != nil {
        return "", err
    }

    return process_file, nil
}
