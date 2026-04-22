package handlers

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

type authHandler struct {
	db     *sql.DB
	config *oauth2.Config
}

func newAuthHandler(db *sql.DB) *authHandler {
	backendURL := os.Getenv("BACKEND_URL")
	if backendURL == "" {
		backendURL = "http://localhost:8080"
	}
	return &authHandler{
		db: db,
		config: &oauth2.Config{
			ClientID:     os.Getenv("GOOGLE_CLIENT_ID"),
			ClientSecret: os.Getenv("GOOGLE_CLIENT_SECRET"),
			RedirectURL:  backendURL + "/auth/google/callback",
			Scopes:       []string{"openid", "email", "profile"},
			Endpoint:     google.Endpoint,
		},
	}
}

func (h *authHandler) login(w http.ResponseWriter, r *http.Request) {
	url := h.config.AuthCodeURL("state", oauth2.AccessTypeOnline)
	http.Redirect(w, r, url, http.StatusTemporaryRedirect)
}

func (h *authHandler) callback(w http.ResponseWriter, r *http.Request) {
	code := r.URL.Query().Get("code")
	if code == "" {
		http.Error(w, "missing code", http.StatusBadRequest)
		return
	}

	oauthToken, err := h.config.Exchange(context.Background(), code)
	if err != nil {
		http.Error(w, "token exchange failed", http.StatusInternalServerError)
		return
	}

	client := h.config.Client(context.Background(), oauthToken)
	resp, err := client.Get("https://www.googleapis.com/oauth2/v2/userinfo")
	if err != nil {
		http.Error(w, "failed to get user info", http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	var info struct {
		Sub       string `json:"id"`
		Email     string `json:"email"`
		Name      string `json:"name"`
		AvatarURL string `json:"picture"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&info); err != nil {
		http.Error(w, "failed to decode user info", http.StatusInternalServerError)
		return
	}

	var userID string
	err = h.db.QueryRowContext(r.Context(), `
		INSERT INTO users (google_sub, email, name, avatar_url, updated_at)
		VALUES ($1, $2, $3, $4, NOW())
		ON CONFLICT (google_sub) DO UPDATE SET
			email      = EXCLUDED.email,
			name       = EXCLUDED.name,
			avatar_url = EXCLUDED.avatar_url,
			updated_at = NOW()
		RETURNING id
	`, info.Sub, info.Email, info.Name, info.AvatarURL).Scan(&userID)
	if err != nil {
		http.Error(w, "failed to save user", http.StatusInternalServerError)
		return
	}

	signed, err := issueJWT(userID, info.Email, info.Name)
	if err != nil {
		http.Error(w, "failed to issue token", http.StatusInternalServerError)
		return
	}

	frontendURL := os.Getenv("FRONTEND_URL")
	if frontendURL == "" {
		frontendURL = "http://localhost:5173"
	}
	http.Redirect(w, r, fmt.Sprintf("%s/#/auth/callback?token=%s", frontendURL, signed), http.StatusTemporaryRedirect)
}

func (h *authHandler) me(w http.ResponseWriter, r *http.Request) {
	userID, ok := getUserID(r)
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	var user struct {
		ID        string `json:"id"`
		Email     string `json:"email"`
		Name      string `json:"name"`
		AvatarURL string `json:"avatar_url"`
	}
	err := h.db.QueryRowContext(r.Context(), `
		SELECT id, email, name, avatar_url FROM users WHERE id = $1
	`, userID).Scan(&user.ID, &user.Email, &user.Name, &user.AvatarURL)
	if err == sql.ErrNoRows {
		http.Error(w, "user not found", http.StatusNotFound)
		return
	}
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusOK, user)
}

func issueJWT(userID, email, name string) (string, error) {
	secret := os.Getenv("JWT_SECRET")
	if secret == "" {
		return "", fmt.Errorf("JWT_SECRET not set")
	}
	claims := jwt.MapClaims{
		"sub":   userID,
		"email": email,
		"name":  name,
		"exp":   time.Now().Add(30 * 24 * time.Hour).Unix(),
		"iat":   time.Now().Unix(),
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(secret))
}
