package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/mikeysoft/flotilla/internal/server/auth"
	"github.com/mikeysoft/flotilla/internal/server/database"
)

const (
	invalidRequestMsg = "invalid request"
	whereIDClause     = "id = ?"
)

type UsersHandler struct{}

func NewUsersHandler() *UsersHandler { return &UsersHandler{} }

func (h *UsersHandler) List(c *gin.Context) {
	if !ensureAdmin(c) {
		return
	}
	var users []database.User
	if err := database.DB.Find(&users).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed"})
		return
	}
	c.JSON(http.StatusOK, users)
}

type createUserReq struct {
	Username string  `json:"username" binding:"required"`
	Email    *string `json:"email"`
	Password string  `json:"password" binding:"required"`
	Role     string  `json:"role" binding:"required"`
}

func (h *UsersHandler) Create(c *gin.Context) {
	if !ensureAdmin(c) {
		return
	}
	var req createUserReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": invalidRequestMsg})
		return
	}
	role := normalizeRole(req.Role)
	if !isValidRole(role) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid role"})
		return
	}
	hash, _ := auth.HashPassword(req.Password)
	u := database.User{
		Username:     req.Username,
		Email:        req.Email,
		PasswordHash: hash,
		Role:         role,
		IsActive:     true,
	}
	if err := database.DB.Create(&u).Error; err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "create failed"})
		return
	}
	c.JSON(http.StatusCreated, u)
}

type updateUserReq struct {
	Email    *string `json:"email"`
	Role     *string `json:"role"`
	IsActive *bool   `json:"is_active"`
}

func (h *UsersHandler) Update(c *gin.Context) {
	if !ensureAdmin(c) {
		return
	}
	id := c.Param("id")
	if _, err := uuid.Parse(id); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "bad id"})
		return
	}
	var req updateUserReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": invalidRequestMsg})
		return
	}
	updates := map[string]any{}
	if req.Email != nil {
		updates["email"] = req.Email
	}
	if req.Role != nil {
		role := normalizeRole(*req.Role)
		if !isValidRole(role) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid role"})
			return
		}
		updates["role"] = role
	}
	if req.IsActive != nil {
		updates["is_active"] = *req.IsActive
	}
	if len(updates) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "no changes"})
		return
	}
	if err := database.DB.Model(&database.User{}).Where("id = ?", id).Updates(updates).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "update failed"})
		return
	}
	c.Status(http.StatusNoContent)
}

type resetPasswordReq struct {
	Password string `json:"password" binding:"required"`
}

func (h *UsersHandler) ResetPassword(c *gin.Context) {
	if !ensureAdmin(c) {
		return
	}
	id := c.Param("id")
	if _, err := uuid.Parse(id); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "bad id"})
		return
	}
	var req resetPasswordReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": invalidRequestMsg})
		return
	}
	hash, _ := auth.HashPassword(req.Password)
	if err := database.DB.Model(&database.User{}).Where(whereIDClause, id).Update("password_hash", hash).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "reset failed"})
		return
	}
	c.Status(http.StatusNoContent)
}

// DeleteUserPermanently permanently deletes a user
func (h *UsersHandler) DeleteUserPermanently(c *gin.Context) {
	if !ensureAdmin(c) {
		return
	}
	userID := c.Param("id")
	if userID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "User ID is required"})
		return
	}

	// Parse UUID
	userUUID, err := uuid.Parse(userID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
		return
	}

	// Check if user exists and is inactive
	var user database.User
	if err := database.DB.Where(whereIDClause, userUUID).First(&user).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}

	if user.IsActive {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Cannot delete active user. Deactivate them first."})
		return
	}

	// Get current user ID from JWT for audit log
	currentUserIDStr, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	currentUserID, err := uuid.Parse(currentUserIDStr.(string))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Invalid user ID"})
		return
	}

	// Prevent self-deletion
	if currentUserID == userUUID {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Cannot delete your own account"})
		return
	}

	// Audit log
	auth.LogAuditEvent(&currentUserID, "user_deleted", "user", &userUUID, map[string]interface{}{
		"deleted_username": user.Username,
		"deleted_email":    user.Email,
	}, c.ClientIP(), c.GetHeader("User-Agent"))

	// Permanently delete the user (this will cascade to refresh tokens due to foreign key constraint)
	if err := database.DB.Unscoped().Delete(&user).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete user"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}
