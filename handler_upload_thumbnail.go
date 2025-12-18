package main

import (
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"
	"path/filepath"

	"github.com/bootdotdev/learn-file-storage-s3-golang-starter/internal/auth"
	"github.com/google/uuid"
)

const MAX_MEMORY = 10 << 20

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

	// Upload logic

	mD, err := cfg.db.GetVideo(videoID)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Error fetching data for this video IO", err)
		return
	}

	if userID != mD.UserID {
		respondWithError(w, http.StatusUnauthorized, "User is not the owner of the video", err)
		return
	}

	err = r.ParseMultipartForm(MAX_MEMORY)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Couldn't parse multipart form", err)
		return
	}
	fileBody, header, err := r.FormFile("thumbnail")
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Unable to parse form file", err)
		return
	}
	defer fileBody.Close()

	fileType := header.Header.Get("Content-Type")

	mimeType, _, err := mime.ParseMediaType(fileType)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Couldn't parse content type", err)
		return
	}
	if mimeType != "image/jpeg" && mimeType != "image/png" {
		respondWithError(w, http.StatusBadRequest, "Thumbnails can only be jpg or png", err)
		return
	}

	extensions, _ := mime.ExtensionsByType(fileType)

	fileName := fmt.Sprintf("%s%s", videoID, extensions[0])
	path := filepath.Join("assets/", fileName)
	asset, err := os.Create(path)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Error creating thumbnail asset on disk", err)
		return
	}

	_, err = io.Copy(asset, fileBody)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Error writing thumbnail to asset", err)
		return
	}

	fileUrl := fmt.Sprintf("http://localhost:%s/assets/%s", cfg.port, fileName)
	mD.ThumbnailURL = &fileUrl

	err = cfg.db.UpdateVideo(mD)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Error updating video metadata", err)
	}

	respondWithJSON(w, http.StatusOK, mD)
}
