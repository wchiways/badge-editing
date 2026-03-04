/*
 * Licensed to the Apache Software Foundation (ASF) under one
 * or more contributor license agreements.  See the NOTICE file
 * distributed with this work for additional information
 * regarding copyright ownership.  The ASF licenses this file
 * to you under the Apache License, Version 2.0 (the
 * "License"); you may not use this file except in compliance
 * with the License.  You may obtain a copy of the License at
 *
 *   http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied.  See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

package badge_editing

import (
	"embed"
	"fmt"
	"net/http"
	"reflect"
	"time"
	"unsafe"

	"github.com/apache/answer-plugins/badge-editing/i18n"
	"github.com/apache/answer-plugins/util"
	"github.com/apache/answer/plugin"
	"github.com/gin-gonic/gin"
	"github.com/segmentfault/pacman/log"
	"xorm.io/xorm"
)

//go:embed info.yaml
var Info embed.FS

func init() {
	plugin.Register(&BadgeEditing{})
}

// BadgeEditing is the plugin for badge CRUD and manual awarding.
// It implements Agent (for custom routes) and KVStorage (to obtain DB access).
type BadgeEditing struct {
	db *xorm.Engine
}

func (b *BadgeEditing) Info() plugin.Info {
	info := &util.Info{}
	info.GetInfo(Info)

	return plugin.Info{
		Name:        plugin.MakeTranslator(i18n.InfoName),
		SlugName:    info.SlugName,
		Description: plugin.MakeTranslator(i18n.InfoDescription),
		Author:      info.Author,
		Version:     info.Version,
		Link:        info.Link,
	}
}

// SetOperator implements KVStorage interface.
// We use it to extract the xorm.Engine from the KVOperator's internal Data field.
func (b *BadgeEditing) SetOperator(operator *plugin.KVOperator) {
	b.db = extractDB(operator)
	if b.db != nil {
		log.Infof("badge_editing plugin: database engine initialized successfully")
	} else {
		log.Errorf("badge_editing plugin: failed to extract database engine from KVOperator")
	}
}

// extractDB extracts the *xorm.Engine from a KVOperator by reading its unexported 'data' field.
// KVOperator layout: { data *Data, session *xorm.Session, ... }
// Data layout:       { DB *xorm.Engine, Cache cache.Cache }
func extractDB(operator *plugin.KVOperator) *xorm.Engine {
	if operator == nil {
		return nil
	}

	// KVOperator's first field is 'data *Data' (unexported).
	// We use reflect to get the field offset, then unsafe to read it.
	v := reflect.ValueOf(operator).Elem()
	dataField := v.Field(0) // data *Data

	// Get pointer to the unexported field via unsafe
	fieldPtr := unsafe.Pointer(dataField.UnsafeAddr())
	dataPtr := *(**plugin.Data)(fieldPtr)
	if dataPtr == nil {
		return nil
	}
	return dataPtr.DB
}

// --- Agent interface ---

func (b *BadgeEditing) RegisterUnAuthRouter(r *gin.RouterGroup) {}

func (b *BadgeEditing) RegisterAuthUserRouter(r *gin.RouterGroup) {}

func (b *BadgeEditing) RegisterAuthAdminRouter(r *gin.RouterGroup) {
	r.GET("/badge-editing/badges", b.ListBadges)
	r.POST("/badge-editing/badges", b.CreateBadge)
	r.PUT("/badge-editing/badges", b.UpdateBadge)
	r.DELETE("/badge-editing/badges/:id", b.DeleteBadge)
	r.GET("/badge-editing/badge-groups", b.ListBadgeGroups)
	r.POST("/badge-editing/badges/award", b.AwardBadge)
	r.DELETE("/badge-editing/badges/award", b.RevokeBadgeAward)
}

// --- Request / Response Types ---

type CreateBadgeReq struct {
	Name         string `json:"name" binding:"required"`
	Icon         string `json:"icon" binding:"required"`
	Description  string `json:"description"`
	Level        int    `json:"level" binding:"required,oneof=1 2 3"`
	BadgeGroupID int64  `json:"badge_group_id" binding:"required"`
	Single       int8   `json:"single" binding:"required,oneof=1 2"`
}

type UpdateBadgeReq struct {
	ID           string `json:"id" binding:"required"`
	Name         string `json:"name" binding:"required"`
	Icon         string `json:"icon" binding:"required"`
	Description  string `json:"description"`
	Level        int    `json:"level" binding:"required,oneof=1 2 3"`
	BadgeGroupID int64  `json:"badge_group_id" binding:"required"`
	Single       int8   `json:"single" binding:"required,oneof=1 2"`
	Status       int8   `json:"status"`
}

type AwardBadgeReq struct {
	BadgeID  string `json:"badge_id" binding:"required"`
	Username string `json:"username" binding:"required"`
}

type RevokeBadgeAwardReq struct {
	BadgeID  string `json:"badge_id" binding:"required"`
	Username string `json:"username" binding:"required"`
}

type BadgeResponse struct {
	ID           string    `json:"id"`
	Name         string    `json:"name"`
	Icon         string    `json:"icon"`
	Description  string    `json:"description"`
	AwardCount   int       `json:"award_count"`
	Status       int8      `json:"status"`
	BadgeGroupID int64     `json:"badge_group_id"`
	Level        int       `json:"level"`
	Single       int8      `json:"single"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

type BadgeGroupResponse struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// --- Database Entities (matching Answer's existing tables) ---

type Badge struct {
	ID           string    `xorm:"not null pk BIGINT(20) id"`
	CreatedAt    time.Time `xorm:"created not null default CURRENT_TIMESTAMP TIMESTAMP created_at"`
	UpdatedAt    time.Time `xorm:"updated not null default CURRENT_TIMESTAMP TIMESTAMP updated_at"`
	Name         string    `xorm:"not null default '' VARCHAR(256) name"`
	Icon         string    `xorm:"not null default '' VARCHAR(1024) icon"`
	AwardCount   int       `xorm:"not null default 0 INT(11) award_count"`
	Description  string    `xorm:"not null MEDIUMTEXT description"`
	Status       int8      `xorm:"not null default 1 INT(11) status"`
	BadgeGroupID int64     `xorm:"not null default 0 BIGINT(20) badge_group_id"`
	Level        int       `xorm:"not null default 1 TINYINT(4) level"`
	Single       int8      `xorm:"not null default 1 TINYINT(4) single"`
	Collect      string    `xorm:"not null default '' VARCHAR(128) collect"`
	Handler      string    `xorm:"not null default '' VARCHAR(128) handler"`
	Param        string    `xorm:"not null TEXT param"`
}

func (Badge) TableName() string { return "badge" }

type BadgeGroup struct {
	ID        string    `xorm:"not null pk autoincr BIGINT(20) id"`
	Name      string    `xorm:"not null default '' VARCHAR(256) name"`
	CreatedAt time.Time `xorm:"created not null default CURRENT_TIMESTAMP TIMESTAMP created_at"`
	UpdatedAt time.Time `xorm:"updated not null default CURRENT_TIMESTAMP TIMESTAMP updated_at"`
}

func (BadgeGroup) TableName() string { return "badge_group" }

type BadgeAward struct {
	ID             string    `xorm:"not null pk BIGINT(20) id"`
	CreatedAt      time.Time `xorm:"created not null default CURRENT_TIMESTAMP TIMESTAMP created_at"`
	UpdatedAt      time.Time `xorm:"updated not null default CURRENT_TIMESTAMP TIMESTAMP updated_at"`
	UserID         string    `xorm:"not null index BIGINT(20) user_id"`
	BadgeID        string    `xorm:"not null index BIGINT(20) badge_id"`
	AwardKey       string    `xorm:"not null index VARCHAR(64) award_key"`
	BadgeGroupID   int64     `xorm:"not null index BIGINT(20) badge_group_id"`
	IsBadgeDeleted int8      `xorm:"not null TINYINT(1) is_badge_deleted"`
}

func (BadgeAward) TableName() string { return "badge_award" }

type User struct {
	ID       string `xorm:"not null pk BIGINT(20) id"`
	Username string `xorm:"not null VARCHAR(50) username"`
	Status   int    `xorm:"not null default 1 INT(11) status"`
}

func (User) TableName() string { return "user" }

type UniqueID struct {
	ID         int64 `xorm:"not null pk autoincr BIGINT(20) id"`
	UniqIDType int   `xorm:"not null default 0 INT(11) uniqid_type"`
}

func (UniqueID) TableName() string { return "uniqid" }

const (
	badgeStatusActive   = 1
	badgeStatusInactive = 11
	badgeStatusDeleted  = 10

	badgeAwardNotDeleted = 0
	badgeAwardDeleted    = 1

	uniqIDTypeBadge = 6
)

// --- Helper ---

func (b *BadgeEditing) checkDB(ctx *gin.Context) bool {
	if b.db == nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "database not initialized"})
		return false
	}
	return true
}

func formatID(id int64) string {
	return fmt.Sprintf("%d", id)
}

// --- Handlers ---

func (b *BadgeEditing) ListBadges(ctx *gin.Context) {
	if !b.checkDB(ctx) {
		return
	}

	badges := make([]*Badge, 0)
	err := b.db.Where("status <> ?", badgeStatusDeleted).
		OrderBy("badge_group_id ASC, id ASC").
		Find(&badges)
	if err != nil {
		log.Errorf("list badges failed: %v", err)
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list badges"})
		return
	}

	resp := make([]*BadgeResponse, len(badges))
	for idx, badge := range badges {
		resp[idx] = badgeToResponse(badge)
	}

	ctx.JSON(http.StatusOK, gin.H{"data": resp})
}

func (b *BadgeEditing) CreateBadge(ctx *gin.Context) {
	if !b.checkDB(ctx) {
		return
	}

	req := &CreateBadgeReq{}
	if err := ctx.ShouldBindJSON(req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Verify badge group exists
	group := &BadgeGroup{}
	exists, err := b.db.ID(req.BadgeGroupID).Get(group)
	if err != nil {
		log.Errorf("check badge group failed: %v", err)
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "failed to check badge group"})
		return
	}
	if !exists {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "badge group not found"})
		return
	}

	// Generate unique ID
	uniqID := &UniqueID{UniqIDType: uniqIDTypeBadge}
	_, err = b.db.Insert(uniqID)
	if err != nil {
		log.Errorf("generate unique id failed: %v", err)
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "failed to generate unique id"})
		return
	}

	badge := &Badge{
		ID:           formatID(uniqID.ID),
		Name:         req.Name,
		Icon:         req.Icon,
		Description:  req.Description,
		Status:       badgeStatusActive,
		BadgeGroupID: req.BadgeGroupID,
		Level:        req.Level,
		Single:       req.Single,
		Collect:      "",
		Handler:      "",
		Param:        "{}",
	}

	_, err = b.db.Insert(badge)
	if err != nil {
		log.Errorf("create badge failed: %v", err)
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create badge"})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"data": badgeToResponse(badge)})
}

func (b *BadgeEditing) UpdateBadge(ctx *gin.Context) {
	if !b.checkDB(ctx) {
		return
	}

	req := &UpdateBadgeReq{}
	if err := ctx.ShouldBindJSON(req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	badge := &Badge{}
	exists, err := b.db.ID(req.ID).Get(badge)
	if err != nil {
		log.Errorf("get badge failed: %v", err)
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get badge"})
		return
	}
	if !exists || badge.Status == badgeStatusDeleted {
		ctx.JSON(http.StatusNotFound, gin.H{"error": "badge not found"})
		return
	}

	// Verify badge group exists
	group := &BadgeGroup{}
	groupExists, err := b.db.ID(req.BadgeGroupID).Get(group)
	if err != nil {
		log.Errorf("check badge group failed: %v", err)
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "failed to check badge group"})
		return
	}
	if !groupExists {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "badge group not found"})
		return
	}

	badge.Name = req.Name
	badge.Icon = req.Icon
	badge.Description = req.Description
	badge.Level = req.Level
	badge.BadgeGroupID = req.BadgeGroupID
	badge.Single = req.Single
	if req.Status > 0 {
		badge.Status = req.Status
	}

	_, err = b.db.ID(req.ID).
		Cols("name", "icon", "description", "level", "badge_group_id", "single", "status").
		Update(badge)
	if err != nil {
		log.Errorf("update badge failed: %v", err)
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update badge"})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"data": badgeToResponse(badge)})
}

func (b *BadgeEditing) DeleteBadge(ctx *gin.Context) {
	if !b.checkDB(ctx) {
		return
	}

	id := ctx.Param("id")
	if id == "" {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "badge id is required"})
		return
	}

	badge := &Badge{}
	exists, err := b.db.ID(id).Get(badge)
	if err != nil {
		log.Errorf("get badge failed: %v", err)
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get badge"})
		return
	}
	if !exists {
		ctx.JSON(http.StatusNotFound, gin.H{"error": "badge not found"})
		return
	}

	// Soft delete badge
	_, err = b.db.ID(id).Cols("status").Update(&Badge{Status: badgeStatusDeleted})
	if err != nil {
		log.Errorf("delete badge failed: %v", err)
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "failed to delete badge"})
		return
	}

	// Mark all related badge awards as deleted
	_, err = b.db.Where("badge_id = ?", id).
		Cols("is_badge_deleted").
		Update(&BadgeAward{IsBadgeDeleted: badgeAwardDeleted})
	if err != nil {
		log.Errorf("mark badge awards deleted failed: %v", err)
	}

	ctx.JSON(http.StatusOK, gin.H{"message": "badge deleted successfully"})
}

func (b *BadgeEditing) ListBadgeGroups(ctx *gin.Context) {
	if !b.checkDB(ctx) {
		return
	}

	groups := make([]*BadgeGroup, 0)
	err := b.db.Find(&groups)
	if err != nil {
		log.Errorf("list badge groups failed: %v", err)
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list badge groups"})
		return
	}

	resp := make([]*BadgeGroupResponse, len(groups))
	for idx, group := range groups {
		resp[idx] = &BadgeGroupResponse{
			ID:   group.ID,
			Name: group.Name,
		}
	}

	ctx.JSON(http.StatusOK, gin.H{"data": resp})
}

func (b *BadgeEditing) AwardBadge(ctx *gin.Context) {
	if !b.checkDB(ctx) {
		return
	}

	req := &AwardBadgeReq{}
	if err := ctx.ShouldBindJSON(req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Find user by username
	user := &User{}
	exists, err := b.db.Where("username = ? AND status = 1", req.Username).Get(user)
	if err != nil {
		log.Errorf("find user failed: %v", err)
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "failed to find user"})
		return
	}
	if !exists {
		ctx.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
		return
	}

	// Check badge exists and is active
	badge := &Badge{}
	badgeExists, err := b.db.ID(req.BadgeID).Get(badge)
	if err != nil {
		log.Errorf("find badge failed: %v", err)
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "failed to find badge"})
		return
	}
	if !badgeExists || badge.Status != badgeStatusActive {
		ctx.JSON(http.StatusNotFound, gin.H{"error": "badge not found or inactive"})
		return
	}

	// For single-award badges, check if already awarded
	if badge.Single == 1 {
		existingAward := &BadgeAward{}
		awarded, err := b.db.Where("badge_id = ? AND user_id = ?", req.BadgeID, user.ID).Get(existingAward)
		if err != nil {
			log.Errorf("check award failed: %v", err)
			ctx.JSON(http.StatusInternalServerError, gin.H{"error": "failed to check existing award"})
			return
		}
		if awarded {
			ctx.JSON(http.StatusConflict, gin.H{"error": "badge already awarded to this user"})
			return
		}
	}

	// Generate unique ID for the award
	uniqID := &UniqueID{UniqIDType: uniqIDTypeBadge}
	_, err = b.db.Insert(uniqID)
	if err != nil {
		log.Errorf("generate unique id failed: %v", err)
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "failed to generate unique id"})
		return
	}

	award := &BadgeAward{
		ID:             formatID(uniqID.ID),
		UserID:         user.ID,
		BadgeID:        req.BadgeID,
		AwardKey:       "0",
		BadgeGroupID:   badge.BadgeGroupID,
		IsBadgeDeleted: badgeAwardNotDeleted,
	}

	_, err = b.db.Insert(award)
	if err != nil {
		log.Errorf("award badge failed: %v", err)
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "failed to award badge"})
		return
	}

	// Update badge award count
	_, err = b.db.ID(req.BadgeID).Incr("award_count", 1).Update(&Badge{})
	if err != nil {
		log.Errorf("update award count failed: %v", err)
	}

	ctx.JSON(http.StatusOK, gin.H{
		"message":  "badge awarded successfully",
		"user_id":  user.ID,
		"badge_id": req.BadgeID,
		"award_id": award.ID,
	})
}

func (b *BadgeEditing) RevokeBadgeAward(ctx *gin.Context) {
	if !b.checkDB(ctx) {
		return
	}

	req := &RevokeBadgeAwardReq{}
	if err := ctx.ShouldBindJSON(req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Find user by username
	user := &User{}
	exists, err := b.db.Where("username = ? AND status = 1", req.Username).Get(user)
	if err != nil {
		log.Errorf("find user failed: %v", err)
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "failed to find user"})
		return
	}
	if !exists {
		ctx.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
		return
	}

	// Delete the award records
	count, err := b.db.Where("badge_id = ? AND user_id = ?", req.BadgeID, user.ID).Delete(&BadgeAward{})
	if err != nil {
		log.Errorf("revoke badge award failed: %v", err)
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "failed to revoke badge award"})
		return
	}

	if count == 0 {
		ctx.JSON(http.StatusNotFound, gin.H{"error": "badge award not found for this user"})
		return
	}

	// Update badge award count
	_, err = b.db.ID(req.BadgeID).Decr("award_count", int(count)).Update(&Badge{})
	if err != nil {
		log.Errorf("update award count failed: %v", err)
	}

	ctx.JSON(http.StatusOK, gin.H{
		"message":       "badge award revoked successfully",
		"revoked_count": count,
	})
}

func badgeToResponse(badge *Badge) *BadgeResponse {
	return &BadgeResponse{
		ID:           badge.ID,
		Name:         badge.Name,
		Icon:         badge.Icon,
		Description:  badge.Description,
		AwardCount:   badge.AwardCount,
		Status:       badge.Status,
		BadgeGroupID: badge.BadgeGroupID,
		Level:        badge.Level,
		Single:       badge.Single,
		CreatedAt:    badge.CreatedAt,
		UpdatedAt:    badge.UpdatedAt,
	}
}
