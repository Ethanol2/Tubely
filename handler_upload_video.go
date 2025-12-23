package main

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"io"
	"log"
	"mime"
	"net/http"
	"os"
	"strings"

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

	asset, err := os.CreateTemp("", "tubely-upload-*.mp4")
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Error creating temporary video asset", err)
		return
	}

	n, err := io.Copy(asset, fileBody)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Error writing video to asset", err)
		return
	}
	println("video size:", n)

	asset.Seek(0, io.SeekStart)

	ratio, err := getVideoAspectRatio(asset.Name())
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Something went wrong while processing the video", err)
		return
	}

	processedPath, err := processVideoForFastStart(asset.Name())
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Something went wrong while processing the video", err)
		return
	}
	asset.Close()
	os.Remove(asset.Name())

	asset, err = os.Open(processedPath)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Something went wrong while handling the video", err)
	}
	defer os.Remove(asset.Name())
	defer asset.Close()

	// Delete old video
	if metadata.VideoURL != nil {
		oldVidInfo := strings.Split(*metadata.VideoURL, ",")

		if len(oldVidInfo) == 2 {

			_, err = cfg.s3Client.DeleteObject(r.Context(), &s3.DeleteObjectInput{
				Bucket: &oldVidInfo[0],
				Key:    &oldVidInfo[1],
			})

			if err != nil {
				log.Println("Error deleting old video:\n", err)
			} else {
				log.Println("Old video deleted")
			}

		} else {
			log.Println("Couldn't delete old video:", metadata.VideoURL)
		}
	}

	var key string
	switch ratio {
	case "16:9":
		key = "landscape/"
	case "9:16":
		key = "portrait/"
	default:
		key = "other/"
	}

	rNum := make([]byte, 32)
	rand.Read(rNum)
	key += base64.RawURLEncoding.EncodeToString(rNum)
	key += ".mp4"

	_, err = cfg.s3Client.PutObject(r.Context(), &s3.PutObjectInput{
		Bucket:      &cfg.s3Bucket,
		Key:         &key,
		Body:        asset,
		ContentType: &mimeType,
	})
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Error pushing video to aws", err)
		return
	}

	//url := fmt.Sprintf("https://%s.s3.%s.amazonaws.com/%s", cfg.s3Bucket, cfg.s3Region, key)
	url := fmt.Sprintf("%s,%s", cfg.s3Bucket, key)
	metadata.VideoURL = &url

	err = cfg.db.UpdateVideo(metadata)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Error updating video metadata", err)
	}

	signedMd, err := cfg.dbVideoToSignedVideo(metadata)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Error getting video url", err)
		return
	}

	respondWithJSON(w, http.StatusOK, signedMd)
}
