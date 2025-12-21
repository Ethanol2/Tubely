package main

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"
	"path/filepath"

	"github.com/bootdotdev/learn-file-storage-s3-golang-starter/internal/database"
)

const MAX_THUMBNAIL_MEMORY = 10 << 20

func (cfg *apiConfig) handlerUploadThumbnail(w http.ResponseWriter, r *http.Request, metadata database.Video) {

	fmt.Println("uploading thumbnail for video", metadata.ID, "by user", metadata.UserID)

	err := r.ParseMultipartForm(MAX_THUMBNAIL_MEMORY)
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

	rNum := make([]byte, 32)
	rand.Read(rNum)

	fileName := fmt.Sprintf("%s%s", base64.RawURLEncoding.EncodeToString(rNum), extensions[0])
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
	metadata.ThumbnailURL = &fileUrl

	err = cfg.db.UpdateVideo(metadata)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Error updating video metadata", err)
	}

	respondWithJSON(w, http.StatusOK, metadata)
}
