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

type GroupHandler struct {
	groupService    *service.GroupService
	presenceService *service.PresenceService
}

func NewGroupHandler(groupService *service.GroupService, presenceService *service.PresenceService) *GroupHandler {
	return &GroupHandler{
		groupService:    groupService,
		presenceService: presenceService,
	}
}

type CreateGroupInput struct {
	Name string `json:"name"`
}

func (h *GroupHandler) Create(w http.ResponseWriter, r *http.Request) {
	uidStr := middleware.GetUserID(r.Context())
	ownerID, err := bson.ObjectIDFromHex(uidStr)
	if err != nil {
		respondError(w, http.StatusUnauthorized, "invalid user id")
		return
	}

	var input CreateGroupInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	group, err := h.groupService.CreateGroup(r.Context(), ownerID, input.Name)
	if err != nil {
		if errors.Is(err, service.ErrGroupNameEmpty) {
			respondError(w, http.StatusBadRequest, err.Error())
			return
		}
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondJSON(w, http.StatusCreated, group)
}

func (h *GroupHandler) ListUserGroups(w http.ResponseWriter, r *http.Request) {
	uidStr := middleware.GetUserID(r.Context())
	userID, err := bson.ObjectIDFromHex(uidStr)
	if err != nil {
		respondError(w, http.StatusUnauthorized, "invalid user id")
		return
	}

	groups, err := h.groupService.GetUserGroups(r.Context(), userID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, groups)
}

func (h *GroupHandler) GetGroup(w http.ResponseWriter, r *http.Request) {
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

	group, err := h.groupService.GetGroupDetails(r.Context(), userID, groupID)
	if err != nil {
		if errors.Is(err, repository.ErrGroupNotFound) {
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

	respondJSON(w, http.StatusOK, group)
}

type JoinGroupInput struct {
	InviteCode string `json:"invite_code"`
}

func (h *GroupHandler) Join(w http.ResponseWriter, r *http.Request) {
	uidStr := middleware.GetUserID(r.Context())
	userID, err := bson.ObjectIDFromHex(uidStr)
	if err != nil {
		respondError(w, http.StatusUnauthorized, "invalid user id")
		return
	}

	var input JoinGroupInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	group, err := h.groupService.JoinGroupByCode(r.Context(), userID, input.InviteCode)
	if err != nil {
		if errors.Is(err, repository.ErrGroupNotFound) {
			respondError(w, http.StatusNotFound, err.Error())
			return
		}
		if errors.Is(err, repository.ErrAlreadyMember) {
			respondError(w, http.StatusConflict, err.Error())
			return
		}
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, group)
}

func (h *GroupHandler) GetMembers(w http.ResponseWriter, r *http.Request) {
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

	members, err := h.groupService.GetGroupMembers(r.Context(), userID, groupID)
	if err != nil {
		if errors.Is(err, service.ErrNotGroupMember) {
			respondError(w, http.StatusForbidden, err.Error())
			return
		}
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, members)
}

func (h *GroupHandler) GetPresence(w http.ResponseWriter, r *http.Request) {
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

	members, err := h.groupService.GetGroupMembers(r.Context(), userID, groupID)
	if err != nil {
		if errors.Is(err, service.ErrNotGroupMember) {
			respondError(w, http.StatusForbidden, err.Error())
			return
		}
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	memberIDs := make([]string, len(members))
	for i, m := range members {
		memberIDs[i] = m.UserID.Hex()
	}

	presences, err := h.presenceService.GetGroupPresence(r.Context(), memberIDs, groupID.Hex())
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, presences)
}
