package main

import (
    "context"
    "strings"
    "github.com/aws/aws-sdk-go-v2/service/s3"
    "mime"
    "io"
	"github.com/bootdotdev/learn-file-storage-s3-golang-starter/internal/database"
	"github.com/bootdotdev/learn-file-storage-s3-golang-starter/internal/auth"
	"github.com/google/uuid"
    "fmt"
    "crypto/rand"
    "encoding/base64"
    "os"
	"net/http"
)


func (cfg *apiConfig) handlerUploadVideo(w http.ResponseWriter, r *http.Request) {
    const maxMemory = 1 << 30 //1GB 
    r.Body = http.MaxBytesReader(w, r.Body, maxMemory) //Upload limit

	videoIDString := r.PathValue("videoID")

	videoID, err := uuid.Parse(videoIDString)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid ID", err)
		return
	}
    // Authenticate user

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

	fmt.Println("uploading video", videoID, "by user", userID)

    file, header, err := r.FormFile("video")
    if err != nil {
        respondWithError(w, http.StatusBadRequest, "Unable to parse form file", err)
        return
    }
    defer file.Close()

    media_type_full, _, err := mime.ParseMediaType(header.Header.Get("Content-Type"))
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid Content-Type", err)
		return
	}
	if media_type_full != "video/mp4" {
        respondWithError(w, http.StatusBadRequest, "Invalid file type", nil)
		return
	}
    media_type := strings.Split(media_type_full, "/")[1]

    metadata, err := cfg.db.GetVideo(videoID)
    if (metadata.UserID != userID && metadata != database.Video{}) {
        respondWithError(w, http.StatusUnauthorized, "User not owner of video", err)
        return
    } else if err != nil {
        respondWithError(w, http.StatusInternalServerError, "Failed to fetch video from database", err)
        return
    }

    video_fileName := "tubley-upload.mp4"

    temp_file, err := os.CreateTemp("", video_fileName)
    if err != nil {
        respondWithError(w, http.StatusInternalServerError, "Failed to create temp video file", err)
        return
    }
    defer os.Remove(temp_file.Name())
    defer temp_file.Close()

    _, err = io.Copy(temp_file, file)
    if err != nil {
        respondWithError(w, http.StatusInternalServerError, "Failed to write to video file", err)
        return
    }

    _, err = temp_file.Seek(0, io.SeekStart) // Reset temp_file pointer, read file again from beginning
    if err != nil {
        respondWithError(w, http.StatusInternalServerError, "Failed to reset temp_file pointer", err)
        return
    }

    random_bytes := make([]byte, 32)
    rand.Read(random_bytes)

    /*
    path, err := os.Getwd()
    if err != nil {
        respondWithError(w, http.StatusInternalServerError, "Failed to get aspect ratio", err)
        return
    }
    */

    //video_type_prefix, err := getVideoAspectRatio(fmt.Sprintf("%s/%s", path, video_fileName))
    video_type_prefix, err := getVideoAspectRatio(temp_file.Name())
    if err != nil {
        respondWithError(w, http.StatusInternalServerError, "Failed to get aspect ratio", err)
        return
    }

    switch video_type_prefix {
    case "16:9":
        video_type_prefix = "landscape"
        break
    case "9:16":
        video_type_prefix = "portrait"
        break
    default:
        video_type_prefix = "other"
        break
    }

    processed_video, err := processVideoForFastStart(temp_file.Name())
    if err != nil {
        respondWithError(w, http.StatusInternalServerError, "Failed to process video", err)
        return
    }
    defer os.Remove(processed_video)

    processed_video_file, err := os.Open(processed_video)
    if err != nil {
        respondWithError(w, http.StatusInternalServerError, "Failed to open processed video", err)
        return
    }
    defer processed_video_file.Close()

    video_name := fmt.Sprintf("%s/%s.%s", video_type_prefix, base64.RawURLEncoding.EncodeToString(random_bytes), media_type)

    s3_input := s3.PutObjectInput{
        Bucket:         &cfg.s3Bucket,
        Body:           processed_video_file,
        Key:            &video_name,
        ContentType:    &media_type_full,
    }

    _, err = cfg.s3Client.PutObject(context.Background(), &s3_input)
    if err != nil {
        respondWithError(w, http.StatusInternalServerError, "Failed to put object to s3 bucket", err)
        return
    }

    thumbnailURL := fmt.Sprintf("%s/%s", cfg.s3CfDistribution, video_name)
    //thumbnailURL := fmt.Sprintf("%s,%s", cfg.s3Bucket, video_name)
    metadata.VideoURL = &thumbnailURL

    // Upload metadata or else tmp URL to get video will be lost
    err = cfg.db.UpdateVideo(metadata)
    if err != nil {
        respondWithError(w, http.StatusInternalServerError, "Failed to update video database", err)
        return
    }


	respondWithJSON(w, http.StatusOK, metadata)
}

