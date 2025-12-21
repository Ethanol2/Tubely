package main

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"

	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/bootdotdev/learn-file-storage-s3-golang-starter/internal/database"
)

const MAX_VIDEO_MEMORY = 1 << 30

func (cfg *apiConfig) handlerUploadVideo(w http.ResponseWriter, r *http.Request, metadata database.Video) {

	fmt.Println("uploading video", metadata.ID, "by user", metadata.UserID)

	err := r.ParseMultipartForm(MAX_VIDEO_MEMORY)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Couldn't parse multipart form", err)
		return
	}
	fileBody, header, err := r.FormFile("video")
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
	if mimeType != "video/mp4" {
		respondWithError(w, http.StatusBadRequest, "Videos can only be mp4", err)
		return
	}

	tmp, err := os.CreateTemp("", "tubely-upload.mp4")
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Error creating temporary video asset", err)
		return
	}
	defer os.Remove(tmp.Name())
	defer tmp.Close()

	n, err := io.Copy(tmp, fileBody)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Error writing video to asset", err)
		return
	}
	println("video size:", n)

	tmp.Seek(0, io.SeekStart)
	rNum := make([]byte, 32)
	rand.Read(rNum)
	key := base64.RawURLEncoding.EncodeToString(rNum)
	key += ".mp4"

	_, err = cfg.s3Client.PutObject(r.Context(), &s3.PutObjectInput{
		Bucket:      &cfg.s3Bucket,
		Key:         &key,
		Body:        tmp,
		ContentType: &mimeType,
	})
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Error pushing video to aws", err)
		return
	}

	url := fmt.Sprintf("https://%s.s3.%s.amazonaws.com/%s", cfg.s3Bucket, cfg.s3Region, key)
	metadata.VideoURL = &url

	err = cfg.db.UpdateVideo(metadata)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Error updating video metadata", err)
	}

	respondWithJSON(w, http.StatusOK, metadata)
}
