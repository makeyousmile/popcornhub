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

type AuthHandler struct {
	authService *service.AuthService
}

func NewAuthHandler(authService *service.AuthService) *AuthHandler {
	return &AuthHandler{authService: authService}
}

func (h *AuthHandler) Register(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 1024*64)
	var input service.RegisterInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	res, err := h.authService.Register(r.Context(), input)
	if err != nil {
		if errors.Is(err, repository.ErrUserAlreadyExists) {
			respondError(w, http.StatusConflict, "user with this email already exists")
			return
		}
		if errors.Is(err, service.ErrEmptyFields) || errors.Is(err, service.ErrInvalidEmail) || errors.Is(err, service.ErrPasswordTooShort) {
			respondError(w, http.StatusBadRequest, err.Error())
			return
		}
		respondError(w, http.StatusInternalServerError, "registration failed")
		return
	}

	respondJSON(w, http.StatusCreated, res)
}

type LoginInput struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 1024*64)
	var input LoginInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	res, err := h.authService.Login(r.Context(), input.Email, input.Password)
	if err != nil {
		if errors.Is(err, service.ErrInvalidCredentials) {
			respondError(w, http.StatusUnauthorized, "invalid email or password")
			return
		}
		respondError(w, http.StatusInternalServerError, "login failed")
		return
	}

	respondJSON(w, http.StatusOK, res)
}

type RefreshInput struct {
	RefreshToken string `json:"refresh_token"`
}

func (h *AuthHandler) Refresh(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 1024*64)
	var input RefreshInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	tokens, err := h.authService.Refresh(r.Context(), input.RefreshToken)
	if err != nil {
		respondError(w, http.StatusUnauthorized, "invalid or expired refresh token")
		return
	}

	respondJSON(w, http.StatusOK, tokens)
}

func (h *AuthHandler) Me(w http.ResponseWriter, r *http.Request) {
	uidStr := middleware.GetUserID(r.Context())
	uid, err := bson.ObjectIDFromHex(uidStr)
	if err != nil {
		respondError(w, http.StatusUnauthorized, "invalid user id")
		return
	}

	user, err := h.authService.GetUser(r.Context(), uid)
	if err != nil {
		respondError(w, http.StatusNotFound, "user not found")
		return
	}

	respondJSON(w, http.StatusOK, user)
}
