package handlers

import (
	"encoding/json"
	"errors"
	"net/http"

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
			respondError(w, http.StatusNotFound, err.Error())
			return
		}
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, item)
}

func (h *MediaHandler) Create(w http.ResponseWriter, r *http.Request) {
	var item models.MediaItem
	if err := json.NewDecoder(r.Body).Decode(&item); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if item.Title == "" || item.StreamURL == "" {
		respondError(w, http.StatusBadRequest, "title and stream_url are required")
		return
	}

	if err := h.mediaService.Create(r.Context(), &item); err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondJSON(w, http.StatusCreated, item)
}
