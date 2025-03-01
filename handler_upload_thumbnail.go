package main

import (
	"fmt"
    "mime"
    "strings"
    "path/filepath"
    "os"
    "io"
	"net/http"
	"github.com/bootdotdev/learn-file-storage-s3-golang-starter/internal/database"

	"github.com/bootdotdev/learn-file-storage-s3-golang-starter/internal/auth"
	"github.com/google/uuid"
    "crypto/rand"
    "encoding/base64"
)

func (cfg *apiConfig) handlerUploadThumbnail(w http.ResponseWriter, r *http.Request) {
	videoIDString := r.PathValue("videoID")
	videoID, err := uuid.Parse(videoIDString)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid ID", err)
		return
	}

    random_bytes := make([]byte, 32)
    rand.Read(random_bytes)
    thumbnail_name := base64.RawURLEncoding.EncodeToString(random_bytes)

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


	fmt.Println("uploading thumbnail for video", videoID, "by user", userID)

	// TODO: implement the upload here
    const maxMemory = 10 << 20
    r.ParseMultipartForm(maxMemory)

    file, header, err := r.FormFile("thumbnail")
    if err != nil {
        respondWithError(w, http.StatusBadRequest, "Unable to parse form file", err)
        return
    }
    defer file.Close()

    media_type, _, err := mime.ParseMediaType(header.Header.Get("Content-Type"))
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid Content-Type", err)
		return
	}
	if media_type != "image/jpeg" && media_type != "image/png" {
		respondWithError(w, http.StatusBadRequest, "Invalid file type", nil)
		return
	}
    media_type = strings.Split(media_type, "/")[1]

    /*
	image_byte, err := io.ReadAll(file)
	if err != nil {
        respondWithError(w, http.StatusBadRequest, "Unable to read file", err)
        return
	}
    */

    metadata, err := cfg.db.GetVideo(videoID)
    if (metadata.UserID != userID && metadata != database.Video{}) {
        respondWithError(w, http.StatusUnauthorized, "User not owner of video", err)
        return
    } else if err != nil {
        respondWithError(w, http.StatusInternalServerError, "Failed to fetch video from database", err)
        return
    }

    //assetPath := getAssetPath(thumbnail_name, media_type)
	//assetDiskPath := cfg.getAssetDiskPath(assetPath)
    file_dir := filepath.Join(cfg.assetsRoot, fmt.Sprintf("%s.%s", thumbnail_name, media_type))
    image_file, err := os.Create(file_dir)
    if err != nil {
        respondWithError(w, http.StatusInternalServerError, "Failed to create image in file dir", err)
        return
    }
    defer image_file.Close()

    _, err = io.Copy(image_file, file)
    if err != nil {
        respondWithError(w, http.StatusInternalServerError, "Failed to write to image file", err)
        return
    }

    thumbnailURL := fmt.Sprintf("http://localhost:%s/assets/%s.%s", cfg.port, thumbnail_name, media_type)

    metadata.ThumbnailURL = &thumbnailURL
    err = cfg.db.UpdateVideo(metadata)
    if err != nil {
        respondWithError(w, http.StatusInternalServerError, "Failed to update video database", err)
        return
    }


	respondWithJSON(w, http.StatusOK, metadata)
}
