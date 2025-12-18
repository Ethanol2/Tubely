package main

import (
	"fmt"
	"io"
	"net/http"

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

	err = r.ParseMultipartForm(MAX_MEMORY)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Couldn't parse multipart form", err)
		return
	}
	fileMultipart, header, err := r.FormFile("thumbnail")
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Unable to parse form file", err)
		return
	}
	defer fileMultipart.Close()

	fileType := header.Header.Get("Content-Type")
	file, err := io.ReadAll(fileMultipart)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Error reading file content", err)
		return
	}

	mD, err := cfg.db.GetVideo(videoID)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Error fetching data for this video IO", err)
		return
	}

	if userID != mD.UserID {
		respondWithError(w, http.StatusUnauthorized, "User is not the owner of the video", err)
		return
	}

	videoThumbnails[videoID] = thumbnail{
		data:      file,
		mediaType: fileType,
	}

	tnURL := fmt.Sprintf("http://localhost:%s/api/thumbnails/%s", cfg.port, videoID)
	mD.ThumbnailURL = &tnURL

	err = cfg.db.UpdateVideo(mD)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Error updating video metadata", err)
	}

	respondWithJSON(w, http.StatusOK, mD)
}
