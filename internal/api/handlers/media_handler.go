package handlers

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"go.mongodb.org/mongo-driver/v2/bson"

	"popcornhub/internal/models"
	"popcornhub/internal/repository"
	"popcornhub/internal/service"
)

type MediaHandler struct {
	mediaService *service.MediaService
}

func NewMediaHandler(mediaService *service.MediaService) *MediaHandler {
	return &MediaHandler{mediaService: mediaService}
}

func (h *MediaHandler) List(w http.ResponseWriter, r *http.Request) {
	list, err := h.mediaService.List(r.Context())
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, list)
}

func (h *MediaHandler) Get(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id, err := bson.ObjectIDFromHex(idStr)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid media id")
		return
	}

	item, err := h.mediaService.GetByID(r.Context(), id)
	if err != nil {
		if errors.Is(err, repository.ErrMediaNotFound) {
			respondError(w, http.StatusNotFound, "media not found")
			return
		}
		respondError(w, http.StatusInternalServerError, "failed to get media")
		return
	}

	respondJSON(w, http.StatusOK, item)
}

func (h *MediaHandler) Create(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 1024*64)
	var item models.MediaItem
	if err := json.NewDecoder(r.Body).Decode(&item); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	item.Title = strings.TrimSpace(item.Title)
	item.StreamURL = strings.TrimSpace(item.StreamURL)
	item.PosterURL = strings.TrimSpace(item.PosterURL)

	if item.Title == "" || item.StreamURL == "" {
		respondError(w, http.StatusBadRequest, "title and stream_url are required")
		return
	}

	if len([]rune(item.Title)) > 120 {
		respondError(w, http.StatusBadRequest, "title is too long (max 120 chars)")
		return
	}

	if !strings.HasPrefix(item.StreamURL, "http://") && !strings.HasPrefix(item.StreamURL, "https://") {
		respondError(w, http.StatusBadRequest, "stream_url must start with http:// or https://")
		return
	}

	if item.PosterURL != "" && !strings.HasPrefix(item.PosterURL, "http://") && !strings.HasPrefix(item.PosterURL, "https://") {
		respondError(w, http.StatusBadRequest, "poster_url must start with http:// or https://")
		return
	}

	if item.DurationSec <= 0 {
		item.DurationSec = 600 // Default 10 min fallback
	}

	if err := h.mediaService.Create(r.Context(), &item); err != nil {
		respondError(w, http.StatusInternalServerError, "failed to create media item")
		return
	}

	respondJSON(w, http.StatusCreated, item)
}
