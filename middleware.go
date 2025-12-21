package main

import (
	"net/http"

	"github.com/bootdotdev/learn-file-storage-s3-golang-starter/internal/auth"
	"github.com/bootdotdev/learn-file-storage-s3-golang-starter/internal/database"
	"github.com/google/uuid"
)

func (cfg *apiConfig) middlewareAuthenticateUpload(function func(w http.ResponseWriter, r *http.Request, metadata database.Video)) func(w http.ResponseWriter, r *http.Request) {

	return func(w http.ResponseWriter, r *http.Request) {

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

		mD, err := cfg.db.GetVideo(videoID)
		if err != nil {
			respondWithError(w, http.StatusBadRequest, "Error fetching data for this video IO", err)
			return
		}

		if userID != mD.UserID {
			respondWithError(w, http.StatusUnauthorized, "User is not the owner of the video", err)
			return
		}

		function(w, r, mD)
	}
}
