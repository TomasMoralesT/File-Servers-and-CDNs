package main

import (
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"
	"path/filepath"

	"github.com/bootdotdev/learn-file-storage-s3-golang-starter/internal/auth"
	"github.com/bootdotdev/learn-file-storage-s3-golang-starter/internal/database"

	"github.com/google/uuid"
)

func (cfg *apiConfig) handlerUploadThumbnail(w http.ResponseWriter, r *http.Request) {
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

	fmt.Println("uploading thumbnail for video", videoID, "by user", userID)

	const maxMemory = 10 << 20
	r.ParseMultipartForm(maxMemory)

	file, header, err := r.FormFile("thumbnail")
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Unable to parse form file", err)
		return
	}
	defer file.Close()

	mediaType, _, err := mime.ParseMediaType(header.Header.Get("Content-Type"))
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid content type", err)
		return
	}

	if mediaType != "image/jpeg" && mediaType != "image/png" {
		respondWithError(w, http.StatusBadRequest, "Media type not allowed. Only jpeg and png are supported", nil)
		return
	}

	var fileExtension string
	if mediaType == "image/jpeg" {
		fileExtension = "jpg"
	} else if mediaType == "image/png" {
		fileExtension = "png"

	}
	fileName := fmt.Sprintf("%s.%s", videoID.String(), fileExtension)
	filePath := filepath.Join(cfg.assetsRoot, fileName)

	newFile, err := os.Create(filePath)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to create file", err)
		return
	}
	defer newFile.Close()

	_, err = io.Copy(newFile, file)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to write file content", err)
		return
	}

	video, err := cfg.db.GetVideo(videoID)
	if err != nil {
		respondWithError(w, http.StatusNotFound, "Couldn't find video", err)
		return
	}

	if video.UserID != userID {
		respondWithError(w, http.StatusUnauthorized, "You don't own this video", nil)
		return
	}

	thumbnailURL := fmt.Sprintf("http://localhost:%d/assets/%s", cfg.port, fileName)

	updatedVideo := cfg.db.UpdateVideo(database.Video{
		ID:           video.ID,
		CreatedAt:    video.CreatedAt,
		UpdatedAt:    video.UpdatedAt,
		ThumbnailURL: &thumbnailURL,
		VideoURL:     video.VideoURL,
		CreateVideoParams: database.CreateVideoParams{
			Title:       video.Title,
			Description: video.Description,
			UserID:      video.UserID,
		},
	})

	respondWithJSON(w, http.StatusOK, updatedVideo)
}
