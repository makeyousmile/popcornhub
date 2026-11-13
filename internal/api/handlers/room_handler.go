package handlers

import (
	"encoding/json"
	"errors"
	"net/http"

	"go.mongodb.org/mongo-driver/v2/bson"

	"popcornhub/internal/api/middleware"
	"popcornhub/internal/repository"
	"popcornhub/internal/service"
)

type RoomHandler struct {
	roomService *service.RoomService
}

func NewRoomHandler(roomService *service.RoomService) *RoomHandler {
	return &RoomHandler{roomService: roomService}
}

type StartRoomInput struct {
	MediaID string `json:"media_id"`
}

func (h *RoomHandler) CreateOrGetRoom(w http.ResponseWriter, r *http.Request) {
	uidStr := middleware.GetUserID(r.Context())
	userID, err := bson.ObjectIDFromHex(uidStr)
	if err != nil {
		respondError(w, http.StatusUnauthorized, "invalid user id")
		return
	}

	groupIDStr := r.PathValue("id")
	groupID, err := bson.ObjectIDFromHex(groupIDStr)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid group id")
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1024*64)
	var input StartRoomInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	mediaID, err := bson.ObjectIDFromHex(input.MediaID)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid media id")
		return
	}

	room, err := h.roomService.GetOrCreateRoom(r.Context(), userID, groupID, mediaID)
	if err != nil {
		if errors.Is(err, service.ErrNotGroupMember) {
			respondError(w, http.StatusForbidden, "not a member of this group")
			return
		}
		if errors.Is(err, repository.ErrMediaNotFound) {
			respondError(w, http.StatusNotFound, "media not found")
			return
		}
		respondError(w, http.StatusInternalServerError, "failed to get or create room")
		return
	}

	respondJSON(w, http.StatusOK, room)
}

func (h *RoomHandler) GetRoom(w http.ResponseWriter, r *http.Request) {
	uidStr := middleware.GetUserID(r.Context())
	userID, err := bson.ObjectIDFromHex(uidStr)
	if err != nil {
		respondError(w, http.StatusUnauthorized, "invalid user id")
		return
	}

	roomIDStr := r.PathValue("id")
	roomID, err := bson.ObjectIDFromHex(roomIDStr)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid room id")
		return
	}

	details, err := h.roomService.GetRoomDetails(r.Context(), userID, roomID)
	if err != nil {
		if errors.Is(err, repository.ErrRoomNotFound) {
			respondError(w, http.StatusNotFound, err.Error())
			return
		}
		if errors.Is(err, service.ErrNotGroupMember) {
			respondError(w, http.StatusForbidden, err.Error())
			return
		}
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, details)
}
