package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/bootdotdev/learn-file-storage-s3-golang-starter/internal/database"
)

func (cfg apiConfig) ensureAssetsDir() error {
	if _, err := os.Stat(cfg.assetsRoot); os.IsNotExist(err) {
		return os.Mkdir(cfg.assetsRoot, 0755)
	}
	return nil
}

func getVideoAspectRatio(filePath string) (string, error) {
	cmd := exec.Command("ffprobe", "-v", "error", "-print_format", "json", "-show_streams", filePath)

	buffer := bytes.Buffer{}
	cmd.Stdout = &buffer

	err := cmd.Run()
	if err != nil {
		log.Println(cmd.String())
		return "", err
	}

	output := struct {
		Streams []struct {
			Width  float64 `json:"width"`
			Height float64 `json:"height"`
		} `json:"streams"`
	}{}
	err = json.Unmarshal(buffer.Bytes(), &output)
	if err != nil {
		return "", err
	}

	if len(output.Streams) == 0 {
		return "", fmt.Errorf("video doesn't contain any streams")
	}

	resProduct := output.Streams[0].Width / output.Streams[0].Height

	// 16:9
	if math.Abs(resProduct-(float64(16)/float64(9))) < 0.1 {
		return "16:9", nil
	}

	// 9:16
	if math.Abs(resProduct-(float64(9)/float64(16))) < 0.1 {
		return "9:16", nil
	}

	return "other", nil
}

func processVideoForFastStart(filepath string) (string, error) {

	outPath := filepath + ".processing"
	cmd := exec.Command("ffmpeg", "-i", filepath, "-c", "copy", "-movflags", "faststart", "-f", "mp4", outPath)
	err := cmd.Run()
	if err != nil {
		return "", nil
	}
	return outPath, nil
}

func generatePresignedURL(s3Client *s3.Client, bucket, key string, expireTime time.Duration) (string, error) {

	client := s3.NewPresignClient(s3Client)
	req, err := client.PresignGetObject(context.Background(), &s3.GetObjectInput{Key: &key, Bucket: &bucket}, s3.WithPresignExpires(expireTime))
	if err != nil {
		return "", err
	}
	return req.URL, nil
}

func (cfg *apiConfig) dbVideoToSignedVideo(video database.Video) (database.Video, error) {

	if video.VideoURL == nil {
		return video, nil
	}

	vidInfo := strings.Split(*video.VideoURL, ",")
	if len(vidInfo) != 2 {
		return video, fmt.Errorf("video url format is incorrect. Expected s3-bucket,videoKey.mp4")
	}

	signedUrl, err := generatePresignedURL(cfg.s3Client, vidInfo[0], vidInfo[1], time.Duration(5)*time.Minute)
	if err != nil {
		return video, err
	}

	video.VideoURL = &signedUrl
	return video, nil
}
