package api

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/mikeysoft/flotilla/internal/server/auth"
	"github.com/mikeysoft/flotilla/internal/server/database"
)

const (
	csrfTokenHeader = "X-CSRF-Token"
)

type AuthHandler struct{}

func NewAuthHandler() *AuthHandler { return &AuthHandler{} }

type loginRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

func (h *AuthHandler) Setup(c *gin.Context) {
	// Only allow if no users exist
	var cnt int64
	database.DB.Model(&database.User{}).Count(&cnt)
	if cnt > 0 {
		c.JSON(http.StatusForbidden, gin.H{"error": "setup disabled"})
		return
	}
	var req loginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}
	hash, _ := auth.HashPassword(req.Password)
	u := database.User{Username: req.Username, PasswordHash: hash, Role: "admin", IsActive: true}
	if err := database.DB.Create(&u).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create admin"})
		return
	}
	c.JSON(http.StatusCreated, gin.H{"status": "ok"})
}

func (h *AuthHandler) Login(c *gin.Context) {
	var req loginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}
	var u database.User
	if err := database.DB.Where("username = ? AND is_active = ?", req.Username, true).First(&u).Error; err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid credentials"})
		return
	}
	ok, _ := auth.VerifyPassword(req.Password, u.PasswordHash)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid credentials"})
		return
	}
	now := time.Now()
	database.DB.Model(&u).Update("last_login_at", &now)

	// Audit log successful login
	auth.LogAuditEvent(&u.ID, "user_login", "user", &u.ID, map[string]interface{}{
		"username":   u.Username,
		"ip_address": c.ClientIP(),
	}, c.ClientIP(), c.GetHeader("User-Agent"))

	// Issue access and refresh
	jti := uuid.New().String()
	access, err := auth.SignAccessToken(u.ID.String(), u.Username, u.Role, jti, 10*time.Minute)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "token error"})
		return
	}
	familyID := uuid.New()
	tokenID := uuid.New()
	// Persist refresh token metadata
	rt := database.RefreshToken{UserID: u.ID, FamilyID: familyID, TokenID: tokenID, CreatedAt: now, ExpiresAt: now.Add(14 * 24 * time.Hour)}
	_ = database.DB.Create(&rt).Error
	// Set HttpOnly cookie with opaque token id (for MVP we use tokenID)
	c.SetCookie("flotilla_refresh", tokenID.String(), int((14 * 24 * time.Hour).Seconds()), "/", "", true, true)
	// Also set a non-HttpOnly CSRF cookie so the client can recover after reload
	// SameSite defaults to Lax in modern browsers; keeping domain empty for localhost
	http.SetCookie(c.Writer, &http.Cookie{
		Name:     "flotilla_csrf",
		Value:    jti,
		Path:     "/",
		Secure:   true,
		HttpOnly: false,
		Expires:  now.Add(14 * 24 * time.Hour),
	})
	// Return access token and CSRF token (reuse jti as simple CSRF token for MVP)
	c.Header(csrfTokenHeader, jti)
	c.JSON(http.StatusOK, gin.H{"access_token": access, "user": gin.H{"id": u.ID, "username": u.Username, "role": u.Role}})
}

func (h *AuthHandler) Refresh(c *gin.Context) {
	csrf := c.GetHeader(csrfTokenHeader)
	if csrf == "" {
		// Fallback to CSRF cookie to support reloads
		if v, err := c.Cookie("flotilla_csrf"); err == nil {
			csrf = v
		}
	}
	if csrf == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "missing csrf"})
		return
	}
	cookie, err := c.Cookie("flotilla_refresh")
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "missing refresh"})
		return
	}
	// Lookup token and user
	var rt database.RefreshToken
	if err := database.DB.Where("token_id = ?", cookie).First(&rt).Error; err != nil || rt.RevokedAt != nil || time.Now().After(rt.ExpiresAt) {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid refresh"})
		return
	}
	var u database.User
	if err := database.DB.First(&u, "id = ? AND is_active = ?", rt.UserID, true).Error; err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid user"})
		return
	}
	// Rotate refresh
	now := time.Now()
	_ = database.DB.Model(&rt).Update("revoked_at", now).Error
	newTokenID := uuid.New()
	nrt := database.RefreshToken{UserID: u.ID, FamilyID: rt.FamilyID, TokenID: newTokenID, CreatedAt: now, ExpiresAt: now.Add(14 * 24 * time.Hour)}
	_ = database.DB.Create(&nrt).Error
	c.SetCookie("flotilla_refresh", newTokenID.String(), int((14 * 24 * time.Hour).Seconds()), "/", "", true, true)
	// Issue new access
	jti := uuid.New().String()
	access, err := auth.SignAccessToken(u.ID.String(), u.Username, u.Role, jti, 10*time.Minute)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "token error"})
		return
	}
	// Update CSRF cookie and header
	http.SetCookie(c.Writer, &http.Cookie{
		Name:     "flotilla_csrf",
		Value:    jti,
		Path:     "/",
		Secure:   true,
		HttpOnly: false,
		Expires:  now.Add(14 * 24 * time.Hour),
	})
	c.Header(csrfTokenHeader, jti)
	c.JSON(http.StatusOK, gin.H{"access_token": access})
}

func (h *AuthHandler) Logout(c *gin.Context) {
	cookie, err := c.Cookie("flotilla_refresh")
	if err == nil && cookie != "" {
		// Revoke entire family for simplicity
		var rt database.RefreshToken
		if err := database.DB.Where("token_id = ?", cookie).First(&rt).Error; err == nil {
			database.DB.Model(&database.RefreshToken{}).Where("family_id = ? AND revoked_at IS NULL", rt.FamilyID).Update("revoked_at", time.Now())
		}
	}
	c.SetCookie("flotilla_refresh", "", -1, "/", "", true, true)
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

// GET /api/v1/auth/setup: Returns setup mode status (true if no users exist)
func (h *AuthHandler) GetSetupStatus(c *gin.Context) {
	var cnt int64
	database.DB.Model(&database.User{}).Count(&cnt)
	c.JSON(http.StatusOK, gin.H{"setup": cnt == 0})
}
