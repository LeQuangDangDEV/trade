package main

import (
	"crypto/rand"
	"errors"
	"fmt"
	"log"
	"math/big"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	jwt "github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"gorm.io/gorm/logger"
	"gorm.io/gorm/schema"
)

/* ===== CONFIG ===== */
const (
	PORT        = "8080"
	DSN         = "root@tcp(127.0.0.1:3306)/trade?charset=utf8mb4&parseTime=True&loc=Local"
	JWT_SECRET  = "change-this-secret"
	CORS_ORIGIN = "http://localhost:5173"
	UPLOAD_DIR  = "uploads"
	KYC_DIR     = "kyc_files"
)

/* ===== DB & MODELS ===== */
var DB *gorm.DB
var uploadsAbs string
var kycAbs string

type AdminUserDetail struct {
	ID         uint   `json:"id"`
	Username   string `json:"username"`
	Name       string `json:"name"`
	Phone      string `json:"phone"`
	AvatarURL  string `json:"avatarUrl"`
	Role       string `json:"role"`
	Coins      int64  `json:"coins"`
	TotalTopup int64  `json:"totalTopup"`
	VIPLevel   int    `json:"vipLevel"`

	KYCStatus   string `json:"kycStatus"`
	KYCFullName string `json:"kycFullName"`
	KYCNumber   string `json:"kycNumber"`
	KYCDob      string `json:"kycDob"`
	HasKYCFront bool   `json:"hasKycFront"`
	HasKYCBack  bool   `json:"hasKycBack"`
}

type WithdrawRequest struct {
	UserID uint   `json:"userId" binding:"required"`
	Amount int64  `json:"amount" binding:"required,gt=0"`
	Note   string `json:"note"`
}

type User struct {
	ID           uint   `gorm:"primaryKey" json:"id"`
	Username     string `gorm:"uniqueIndex;size:191" json:"username"` // đã chuyển hệ thống sang username
	Name         string `json:"name"`
	Phone        string `json:"phone"`
	AvatarURL    string `json:"avatarUrl"`
	PasswordHash string `json:"-"`

	Role       string `gorm:"type:enum('admin','user');default:'user';index" json:"role"`
	Coins      int64  `gorm:"not null;default:0" json:"coins"`
	TotalTopup int64  `gorm:"not null;default:0" json:"totalTopup"`
	VIPLevel   int    `gorm:"column:v_ip_level;not null;default:0" json:"vipLevel"`

	SecondPasswordHash string `json:"-"`
	TxnPinHash         string `json:"-"`

	// ✅ KYC
	KYCStatus   string `gorm:"type:enum('NONE','VERIFIED');default:'NONE'" json:"kycStatus"`
	KYCFullName string `json:"kycFullName"`
	KYCNumber   string `json:"kycNumber"` // Số CCCD
	KYCDob      string `json:"kycDob"`    // YYYY-MM-DD (đơn giản, có thể chuyển sang time.Time nếu muốn)

	KYCFrontPath string `json:"-"` // chỉ lưu tên file trong KYC_DIR
	KYCBackPath  string `json:"-"`

	ReferralCode *string        `gorm:"size:16;uniqueIndex" json:"referralCode,omitempty"`
	ReferredBy   *uint          `gorm:"index" json:"referredBy,omitempty"`
	CreatedAt    time.Time      `json:"createdAt"`
	UpdatedAt    time.Time      `json:"updatedAt"`
	DeletedAt    gorm.DeletedAt `gorm:"index" json:"-"`
}

type ChangePasswordRequest struct {
	OldPassword string `json:"oldPassword" binding:"required"`
	NewPassword string `json:"newPassword" binding:"required,min=6"`
}
type ForgotPasswordReq struct {
	Username    string `json:"username"    binding:"required"`
	SecPassword string `json:"secPassword" binding:"required"` // mật khẩu cấp 2 (đã đặt trước đó)
	NewPassword string `json:"newPassword" binding:"required,min=6"`
}
type UpdateSecurityRequest struct {
	OldSecondPassword string `json:"oldSecondPassword"` // bắt buộc nếu đã từng đặt
	NewSecondPassword string `json:"newSecondPassword"` // >= 6 ký tự
	NewTxnPin         string `json:"newTxnPin"`         // 6 số
}

type KycSubmitRequest struct {
	FrontURL string `json:"frontUrl" binding:"required"` // đường dẫn tạm thời (trong /uploads)
	BackURL  string `json:"backUrl"  binding:"required"`
}

// Nhật ký mua VIP
type VipPurchaseTxn struct {
	ID        uint      `gorm:"primaryKey"`
	UserID    uint      `gorm:"not null;index"`
	Level     int       `gorm:"not null"`
	Price     int64     `gorm:"not null"`
	OldLevel  int       `gorm:"not null"`
	CreatedAt time.Time `json:"createdAt"`
}

// Nhật ký chia hoa hồng (9 tầng & admin)
type CommissionTxn struct {
	ID             uint      `gorm:"primaryKey"`
	BuyerID        uint      `gorm:"not null;index"`
	BeneficiaryID  *uint     `gorm:"index"` // upline hoặc admin
	Depth          int       `gorm:"not null"`
	Percent        int       `gorm:"not null"`
	Amount         int64     `gorm:"not null"`
	Kind           string    `gorm:"size:12;not null"` // "UPLINE" | "ADMIN"
	VipLevelBought int       `gorm:"not null"`
	CreatedAt      time.Time `json:"createdAt"`
}

// Thưởng giới thiệu 1 lần/người được mời (nếu bạn còn dùng)
type ReferralReward struct {
	ID        uint      `gorm:"primaryKey"`
	InviterID uint      `gorm:"not null;index"`
	InviteeID uint      `gorm:"not null;uniqueIndex"`
	Amount    int64     `gorm:"not null"`
	CreatedAt time.Time `json:"createdAt"`

	Inviter User `gorm:"foreignKey:InviterID"`
	Invitee User `gorm:"foreignKey:InviteeID"`
}

type BuyVipRequest struct {
	Level int `json:"level" binding:"required,gt=0"`
}

// Giao dịch chuyển coin user→user
type TransferTxn struct {
	ID        uint      `gorm:"primaryKey"`
	FromID    uint      `gorm:"not null;index"`
	ToID      uint      `gorm:"not null;index"`
	Amount    int64     `gorm:"not null"` // coin người nhận nhận (không gồm phí)
	Fee       int64     `gorm:"not null"` // phí người gửi trả
	Note      string    `gorm:"size:255"`
	CreatedAt time.Time `json:"createdAt"`

	From User `gorm:"foreignKey:FromID;constraint:OnUpdate:CASCADE,OnDelete:RESTRICT;"`
	To   User `gorm:"foreignKey:ToID;constraint:OnUpdate:CASCADE,OnDelete:RESTRICT;"`
}

type MarketRow struct {
	ID           uint   `json:"id"`
	Code         string `json:"code"`
	Qty          int64  `json:"qty"`
	PricePerUnit int64  `json:"pricePerUnit"`
	SellerID     uint   `json:"sellerId"`
	SellerUser   string `json:"sellerUsername"`
}
type UpdateKycRequest struct {
	FrontPath string `json:"frontPath" binding:"required"` // có thể là URL "/uploads/..." -> sẽ lấy filename
	BackPath  string `json:"backPath"  binding:"required"`
}

type TransferRow struct {
	ID          uint      `json:"id"`
	Direction   string    `json:"direction"` // "in" | "out"
	Amount      int64     `json:"amount"`
	Fee         int64     `json:"fee"`
	Counterpart string    `json:"counterpart"` // username đối tác
	CreatedAt   time.Time `json:"createdAt"`
}
type WithdrawTxn struct {
	ID        uint   `gorm:"primaryKey"`
	UserID    uint   `gorm:"not null;index"`
	AdminID   uint   `gorm:"not null;index"`
	Amount    int64  `gorm:"not null"` // số coin bị trừ
	Note      string `gorm:"size:255"`
	CreatedAt time.Time

	User  User `gorm:"foreignKey:UserID;constraint:OnUpdate:CASCADE,OnDelete:RESTRICT;"`
	Admin User `gorm:"foreignKey:AdminID;constraint:OnUpdate:CASCADE,OnDelete:RESTRICT;"`
}

type kycUpdateReq struct {
	FrontURL string `json:"frontUrl"` // FE gửi url /uploads/xxx.png
	BackURL  string `json:"backUrl"`
	// chấp nhận alias cũ nếu bạn từng dùng:
	FrontPath string `json:"frontPath"`
	BackPath  string `json:"backPath"`
}

type TransferRequest struct {
	ToUsername     string `json:"toUsername" binding:"required"`
	Amount         int64  `json:"amount" binding:"required,gt=0"`
	Note           string `json:"note"`
	TxnPin         string `json:"txnPin" binding:"required,len=6"`
	SecondPassword string `json:"secondPassword"`
}

type WithdrawRow struct {
	ID         uint      `json:"id"`
	Amount     int64     `json:"amount"`
	Note       string    `json:"note"`
	AdminEmail string    `json:"adminEmail"`
	CreatedAt  time.Time `json:"createdAt"`
}
type AdminUserRow struct {
	ID         uint   `json:"id"`
	Username   string `json:"username"`
	Nickname   string `json:"nickname"`
	VIPLevel   int    `json:"vipLevel"`
	TotalTopup int64  `json:"totalTopup"`
	Coins      int64  `json:"coins"`
}

type TopupRow struct {
	ID            uint      `json:"id"`
	Amount        int64     `json:"amount"`
	Note          string    `json:"note"`
	AdminUsername string    `json:"adminUsername"`
	CreatedAt     time.Time `json:"createdAt"`
}

type CommissionRow struct {
	ID            uint      `json:"id"`
	BuyerUsername string    `json:"buyerUsername"`
	Depth         int       `json:"depth"`
	Percent       int       `json:"percent"`
	Amount        int64     `json:"amount"`
	Kind          string    `json:"kind"`
	VipLevel      int       `json:"vipLevel"`
	CreatedAt     time.Time `json:"createdAt"`
}

type VipBuyRow struct {
	ID        uint      `json:"id"`
	Level     int       `json:"level"`
	Price     int64     `json:"price"`
	OldLevel  int       `json:"oldLevel"`
	CreatedAt time.Time `json:"createdAt"`
}

type VipTier struct {
	ID        uint      `gorm:"primaryKey"           json:"id"`
	Level     int       `gorm:"uniqueIndex;not null" json:"level"`
	Name      string    `gorm:"size:50;not null"     json:"name"`
	MinTopup  int64     `gorm:"not null;default:0"   json:"minTopup"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

type CoinTxn struct {
	ID        uint      `gorm:"primaryKey"`
	UserID    uint      `gorm:"not null;index"`
	AdminID   uint      `gorm:"not null;index"`
	Amount    int64     `gorm:"not null"`
	Note      string    `gorm:"size:255"`
	CreatedAt time.Time `json:"createdAt"`

	User  User `gorm:"foreignKey:UserID;constraint:OnUpdate:CASCADE,OnDelete:RESTRICT;"`
	Admin User `gorm:"foreignKey:AdminID;constraint:OnUpdate:CASCADE,OnDelete:RESTRICT;"`
}

/* ===== DTO ===== */
// Đăng ký: username, password, nickname(name), phone, ref (mã mời) — KHÔNG email
type RegisterRequest struct {
	Username string `json:"username" binding:"required,min=3,max=50"`
	Password string `json:"password" binding:"required,min=6,max=100"`
	Nickname string `json:"nickname"` // FE mới
	Name     string `json:"name"`     // FE cũ (fallback)
	Phone    string `json:"phone" binding:"required,min=8,max=20"`
	Ref      string `json:"ref"` // mã mời (tuỳ chọn)
}

// Đăng nhập: username + password
type LoginRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

type AuthResponse struct {
	Token string `json:"token"`
	User  User   `json:"user"`
}
type ProfileUpdateRequest struct {
	Name      string `json:"name" binding:"required,min=2,max=100"`
	Phone     string `json:"phone" binding:"required,min=8,max=20"`
	AvatarURL string `json:"avatarUrl" binding:"max=500"`
}
type TopupRequest struct {
	UserID uint   `json:"userId" binding:"required"`
	Amount int64  `json:"amount" binding:"required,gt=0"`
	Note   string `json:"note"`
}

// ===== Treasure (rương), Túi đồ & Chợ =====

const (
	CHEST_COST   int64 = 50   // phí mở 1 lần
	MERGE_REWARD int64 = 5000 // thưởng khi hợp nhất đủ 7 viên
)

// item trong túi: chỉ cần Dragon Ball 1..7 (DB1..DB7)
// item trong túi
type InventoryItem struct {
	ID        uint      `gorm:"primaryKey" json:"-"`
	UserID    uint      `gorm:"index;not null" json:"-"`
	Code      string    `gorm:"size:10;not null;index" json:"code"`
	Qty       int64     `gorm:"not null;default:0"   json:"qty"`
	CreatedAt time.Time `json:"-"`
	UpdatedAt time.Time `json:"-"`
}

// log mở rương
type ChestTxn struct {
	ID           uint   `gorm:"primaryKey"`
	UserID       uint   `gorm:"index;not null"`
	Cost         int64  `gorm:"not null"`
	RewardKind   string `gorm:"size:16;not null"` // "COIN" | "DRAGON_BALL"
	RewardCode   string `gorm:"size:10"`          // DB1..DB7 (nếu là DRAGON_BALL)
	RewardAmount int64  `gorm:"not null"`         // coin nếu COIN, còn DB là 1
	CreatedAt    time.Time
}

// bài đăng trên chợ
type MarketListing struct {
	ID           uint   `gorm:"primaryKey"`
	SellerID     uint   `gorm:"index;not null"`
	Code         string `gorm:"size:10;not null"` // DB1..DB7
	Qty          int64  `gorm:"not null"`
	PricePerUnit int64  `gorm:"not null"` // coin / 1 viên
	IsActive     bool   `gorm:"not null;default:true"`
	CreatedAt    time.Time
	UpdatedAt    time.Time

	Seller User `gorm:"foreignKey:SellerID;constraint:OnUpdate:CASCADE,OnDelete:RESTRICT;"`
}

/* ===== DB CONNECT ===== */
func connectDB() {
	dialect := mysql.New(mysql.Config{
		DSN:                       DSN,
		DefaultStringSize:         191,
		DisableDatetimePrecision:  true,
		DontSupportRenameIndex:    true,
		DontSupportRenameColumn:   true,
		SkipInitializeWithVersion: false,
	})

	sqlLogger := logger.New(
		log.New(os.Stdout, "SQL ", log.LstdFlags),
		logger.Config{
			SlowThreshold: 200 * time.Millisecond,
			LogLevel:      logger.Info,
			Colorful:      true,
		},
	)

	db, err := gorm.Open(dialect, &gorm.Config{
		NamingStrategy: schema.NamingStrategy{SingularTable: false},
		Logger:         sqlLogger,
	})
	if err != nil {
		log.Fatal("❌ Cannot connect DB:", err)
	}
	DB = db

	sqlDB, err := DB.DB()
	if err != nil {
		log.Fatal("❌ DB():", err)
	}
	sqlDB.SetMaxIdleConns(10)
	sqlDB.SetMaxOpenConns(50)
	sqlDB.SetConnMaxLifetime(60 * time.Minute)

	if err := sqlDB.Ping(); err != nil {
		log.Fatal("❌ DB Ping error:", err)
	}

	if err := DB.AutoMigrate(
		&User{},
		&VipTier{},
		&CoinTxn{},
		&TransferTxn{},
		&ReferralReward{},
		&VipPurchaseTxn{},
		&CommissionTxn{},
		&WithdrawTxn{},
		&InventoryItem{}, &ChestTxn{}, &MarketListing{},
	); err != nil {
		log.Fatal("❌ AutoMigrate error:", err)
	}

	seedVipTiers()
	fmt.Println("✅ DB migrated")
}

/* ===== HELPERS ===== */
func getAnyAdmin(tx *gorm.DB) (User, error) {
	var admin User
	err := tx.Where("role = ?", "admin").Order("id ASC").First(&admin).Error
	return admin, err
}

// Lấy tối đa 9 cấp upline theo trường referred_by
// Lấy tối đa maxDepth cấp upline theo trường referred_by
func getUplines(tx *gorm.DB, start *uint, maxDepth int) ([]User, error) {
	out := []User{}
	if start == nil {
		return out, nil
	}
	seen := map[uint]bool{}
	cur := start
	for depth := 1; depth <= maxDepth && cur != nil && *cur != 0; depth++ {
		if seen[*cur] {
			break
		}
		seen[*cur] = true
		var u User
		if err := tx.Select("id, email, coins, v_ip_level, referred_by").First(&u, *cur).Error; err != nil {
			break
		}
		out = append(out, u)
		cur = u.ReferredBy
	}
	return out, nil
}

// tạo mã 8 ký tự A..Z 2..9 (tránh 0,O,1,I)
func genRefCode(n int) string {
	const letters = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789"
	b := make([]byte, n)
	for i := range b {
		k, _ := rand.Int(rand.Reader, big.NewInt(int64(len(letters))))
		b[i] = letters[k.Int64()]
	}
	return string(b)
}

// ==== RNG helpers ====
// Trả số nguyên trong [min, max] (bao gồm 2 đầu)
func randInt(min, max int64) int64 {
	if max < min {
		min, max = max, min
	}
	if max == min {
		return min
	}
	n, _ := rand.Int(rand.Reader, big.NewInt(max-min+1))
	return min + n.Int64()
}

// Pick ngẫu nhiên 1 phần tử trong slice (nếu cần)
func randPick[T any](arr []T) T {
	i := randInt(0, int64(len(arr)-1))
	return arr[i]
}

func generateUniqueRefCode(tx *gorm.DB, n int) (string, error) {
	for i := 0; i < 10; i++ {
		code := genRefCode(n)
		var cnt int64
		if err := tx.Model(&User{}).Where("referral_code = ?", code).Count(&cnt).Error; err != nil {
			return "", err
		}
		if cnt == 0 {
			return code, nil
		}
	}
	return "", fmt.Errorf("cannot generate unique referral_code")
}

/* ===== SEED ===== */
func seedVipTiers() {
	var cnt int64
	DB.Model(&VipTier{}).Count(&cnt)
	if cnt > 0 {
		return
	}
	tiers := []VipTier{
		{Level: 1, Name: "VIP 1", MinTopup: 1_000},
		{Level: 2, Name: "VIP 2", MinTopup: 5_000},
		{Level: 3, Name: "VIP 3", MinTopup: 20_000},
		{Level: 4, Name: "VIP 4", MinTopup: 50_000},
		{Level: 5, Name: "VIP 5", MinTopup: 100_000},
	}
	DB.Create(&tiers)
	fmt.Println("🌱 Seeded vip_tiers (MinTopup)")
}

/* ===== MIDDLEWARE ===== */
func authRequired() gin.HandlerFunc {
	return func(c *gin.Context) {
		h := c.GetHeader("Authorization")
		if !strings.HasPrefix(h, "Bearer ") {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Missing token"})
			return
		}
		tokenStr := h[7:]
		t, err := jwt.Parse(tokenStr, func(t *jwt.Token) (interface{}, error) { return []byte(JWT_SECRET), nil })
		if err != nil || !t.Valid {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Token không hợp lệ"})
			return
		}
		if claims, ok := t.Claims.(jwt.MapClaims); ok {
			c.Set("claims", claims)
		}
		c.Next()
	}
}

func adminRequired() gin.HandlerFunc {
	return func(c *gin.Context) {
		val, ok := c.Get("claims")
		if !ok {
			c.AbortWithStatusJSON(401, gin.H{"error": "Unauthorized"})
			return
		}
		role, _ := val.(jwt.MapClaims)["role"].(string)
		if role != "admin" {
			c.AbortWithStatusJSON(403, gin.H{"error": "Forbidden: admin only"})
			return
		}
		c.Next()
	}
}

/* ===== HISTORIES ===== */
func topupHistoryHandler(c *gin.Context) {
	uid := uint(c.MustGet("claims").(jwt.MapClaims)["sub"].(float64))
	var rows []TopupRow
	DB.Table("coin_txns t").
		Select("t.id, t.amount, t.note, t.created_at, a.username AS admin_email").
		Joins("LEFT JOIN users a ON a.id = t.admin_id").
		Where("t.user_id = ?", uid).
		Order("t.id DESC").Scan(&rows)
	c.JSON(200, gin.H{"rows": rows})
}

func vipHistoryHandler(c *gin.Context) {
	uid := uint(c.MustGet("claims").(jwt.MapClaims)["sub"].(float64))
	var rows []VipBuyRow
	DB.Model(&VipPurchaseTxn{}).
		Where("user_id = ?", uid).
		Order("id DESC").Scan(&rows)
	c.JSON(200, gin.H{"rows": rows})
}

func transferHistoryHandler(c *gin.Context) {
	uid := uint(c.MustGet("claims").(jwt.MapClaims)["sub"].(float64))
	var rows []TransferRow
	DB.Table("transfer_txns t").
		Select(`
			t.id,
			IF(t.from_id = ?, 'out', 'in') AS direction,
			t.amount, t.fee, t.created_at,
			CASE WHEN t.from_id = ? THEN to_u.username ELSE from_u.username END AS counterpart`,
			uid, uid).
		Joins("LEFT JOIN users from_u ON from_u.id = t.from_id").
		Joins("LEFT JOIN users to_u   ON to_u.id   = t.to_id").
		Where("t.from_id = ? OR t.to_id = ?", uid, uid).
		Order("t.id DESC").Scan(&rows)
	c.JSON(200, gin.H{"rows": rows})
}

func myCommissionHistoryHandler(c *gin.Context) {
	uid := uint(c.MustGet("claims").(jwt.MapClaims)["sub"].(float64))
	rows := []CommissionRow{}
	DB.Table("commission_txns ct").
		Select(`ct.id, b.username AS buyer_username, ct.depth, ct.percent, ct.amount, ct.kind, ct.vip_level_bought AS vip_level, ct.created_at`).
		Joins("LEFT JOIN users b ON b.id = ct.buyer_id").
		Where("ct.beneficiary_id = ?", uid).
		Order("ct.id DESC").
		Scan(&rows)
	c.JSON(200, gin.H{"rows": rows})
}
func changePasswordHandler(c *gin.Context) {
	uid := uint(c.MustGet("claims").(jwt.MapClaims)["sub"].(float64))

	var req ChangePasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	var user User
	if err := DB.First(&user, uid).Error; err != nil {
		c.JSON(404, gin.H{"error": "User không tồn tại"})
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.OldPassword)); err != nil {
		c.JSON(400, gin.H{"error": "Mật khẩu hiện tại không đúng"})
		return
	}

	newHash, _ := bcrypt.GenerateFromPassword([]byte(req.NewPassword), bcrypt.DefaultCost)
	if err := DB.Model(&user).Update("password_hash", string(newHash)).Error; err != nil {
		c.JSON(500, gin.H{"error": "Không thể cập nhật mật khẩu"})
		return
	}

	c.JSON(200, gin.H{"message": "Đổi mật khẩu thành công"})
}
func updateSecurityHandler(c *gin.Context) {
	uid := uint(c.MustGet("claims").(jwt.MapClaims)["sub"].(float64))

	var req UpdateSecurityRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	var u User
	if err := DB.First(&u, uid).Error; err != nil {
		c.JSON(404, gin.H{"error": "User không tồn tại"})
		return
	}

	// Đổi/đặt mật khẩu cấp 2
	if strings.TrimSpace(req.NewSecondPassword) != "" {
		if len(req.NewSecondPassword) < 6 {
			c.JSON(400, gin.H{"error": "Mật khẩu cấp 2 tối thiểu 6 ký tự"})
			return
		}
		// Nếu đã có mật khẩu cấp 2 trước đó thì yêu cầu nhập cũ để xác nhận
		if u.SecondPasswordHash != "" {
			if err := bcrypt.CompareHashAndPassword([]byte(u.SecondPasswordHash), []byte(req.OldSecondPassword)); err != nil {
				c.JSON(400, gin.H{"error": "Mật khẩu cấp 2 cũ không đúng"})
				return
			}
		}
		hash, _ := bcrypt.GenerateFromPassword([]byte(req.NewSecondPassword), bcrypt.DefaultCost)
		u.SecondPasswordHash = string(hash)
	}

	// Đặt/đổi PIN 6 số (hash)
	if strings.TrimSpace(req.NewTxnPin) != "" {
		pin := req.NewTxnPin
		if len(pin) != 6 || strings.Trim(pin, "0123456789") != "" {
			c.JSON(400, gin.H{"error": "Mã bảo mật (PIN) phải gồm đúng 6 chữ số"})
			return
		}
		hash, _ := bcrypt.GenerateFromPassword([]byte(pin), bcrypt.DefaultCost)
		u.TxnPinHash = string(hash)
	}

	if err := DB.Save(&u).Error; err != nil {
		c.JSON(500, gin.H{"error": "Cập nhật bảo mật thất bại"})
		return
	}
	c.JSON(200, gin.H{"message": "Cập nhật bảo mật thành công"})
}

func forgotPasswordHandler(c *gin.Context) {
	var req ForgotPasswordReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	uname := strings.ToLower(strings.TrimSpace(req.Username))

	var u User
	if err := DB.Where("LOWER(username) = ?", uname).First(&u).Error; err != nil {
		c.JSON(404, gin.H{"error": "Tài khoản không tồn tại"})
		return
	}

	if u.SecondPasswordHash == "" {
		c.JSON(400, gin.H{"error": "Bạn chưa thiết lập mật khẩu cấp 2. Vui lòng liên hệ hỗ trợ"})
		return
	}

	// xác minh mật khẩu cấp 2
	if err := bcrypt.CompareHashAndPassword([]byte(u.SecondPasswordHash), []byte(req.SecPassword)); err != nil {
		c.JSON(400, gin.H{"error": "Mật khẩu cấp 2 không đúng"})
		return
	}

	// cập nhật mật khẩu đăng nhập
	hash, _ := bcrypt.GenerateFromPassword([]byte(req.NewPassword), bcrypt.DefaultCost)
	if err := DB.Model(&u).Update("password_hash", string(hash)).Error; err != nil {
		c.JSON(500, gin.H{"error": "Không thể cập nhật mật khẩu"})
		return
	}

	c.JSON(200, gin.H{"message": "Đổi mật khẩu thành công. Hãy đăng nhập lại bằng mật khẩu mới."})
}
func adminUserDetailHandler(c *gin.Context) {
	uidStr := c.Param("id")
	uid64, err := strconv.ParseUint(uidStr, 10, 64)
	if err != nil || uid64 == 0 {
		c.JSON(400, gin.H{"error": "ID không hợp lệ"})
		return
	}
	var u User
	if err := DB.First(&u, uint(uid64)).Error; err != nil {
		c.JSON(404, gin.H{"error": "User không tồn tại"})
		return
	}
	out := AdminUserDetail{
		ID: u.ID, Username: u.Username, Name: u.Name, Phone: u.Phone, AvatarURL: u.AvatarURL,
		Role: u.Role, Coins: u.Coins, TotalTopup: u.TotalTopup, VIPLevel: u.VIPLevel,
		KYCStatus: u.KYCStatus, KYCFullName: u.KYCFullName, KYCNumber: u.KYCNumber, KYCDob: u.KYCDob,
		HasKYCFront: u.KYCFrontPath != "", HasKYCBack: u.KYCBackPath != "",
	}
	c.JSON(200, gin.H{"user": out})
}
func moveToKycIfFromUploads(srcURL string, prefix string) (string, error) {
	// chỉ nhận file đã ở /uploads (public) => chuyển sang KYC private
	if !strings.HasPrefix(srcURL, "/uploads/") {
		// vẫn chấp nhận nhưng không di chuyển (KHÔNG khuyến khích)
		return "", fmt.Errorf("invalid source (not in /uploads)")
	}
	base := filepath.Base(srcURL)              // tên file
	srcPath := filepath.Join(uploadsAbs, base) // đường dẫn thực tế trong uploads
	if _, err := os.Stat(srcPath); err != nil {
		return "", fmt.Errorf("source file not found")
	}

	// tạo tên mới an toàn trong KYC_DIR
	ext := filepath.Ext(base)
	newName := fmt.Sprintf("%s_%d%s", prefix, time.Now().UnixNano(), ext)
	dstPath := filepath.Join(kycAbs, newName)

	// di chuyển
	if err := os.Rename(srcPath, dstPath); err != nil {
		return "", fmt.Errorf("move failed: %w", err)
	}
	return newName, nil // chỉ lưu filename (không phải đường dẫn)
}

func kycSubmitHandler(c *gin.Context) {
	uid := uint(c.MustGet("claims").(jwt.MapClaims)["sub"].(float64))

	// Parse form
	if err := c.Request.ParseMultipartForm(16 << 20); err != nil {
		c.JSON(400, gin.H{"error": "Dữ liệu không hợp lệ"})
		return
	}
	fullName := strings.TrimSpace(c.PostForm("fullName"))
	dob := strings.TrimSpace(c.PostForm("dob"))       // YYYY-MM-DD
	number := strings.TrimSpace(c.PostForm("number")) // Số CCCD

	if fullName == "" || dob == "" || number == "" {
		c.JSON(400, gin.H{"error": "Vui lòng nhập Họ và tên, Ngày sinh và Số CCCD"})
		return
	}

	// Files
	front, errF := c.FormFile("front")
	back, errB := c.FormFile("back")
	if errF != nil || errB != nil {
		c.JSON(400, gin.H{"error": "Thiếu ảnh mặt trước / mặt sau CCCD"})
		return
	}

	// Unique filenames
	ts := time.Now().UnixNano()
	fFront := fmt.Sprintf("u%d_front_%d%s", uid, ts, filepath.Ext(front.Filename))
	fBack := fmt.Sprintf("u%d_back_%d%s", uid, ts, filepath.Ext(back.Filename))

	pFront := filepath.Join(kycAbs, fFront)
	pBack := filepath.Join(kycAbs, fBack)

	if err := c.SaveUploadedFile(front, pFront); err != nil {
		c.JSON(500, gin.H{"error": "Lưu ảnh CCCD (mặt trước) thất bại"})
		return
	}
	if err := c.SaveUploadedFile(back, pBack); err != nil {
		_ = os.Remove(pFront)
		c.JSON(500, gin.H{"error": "Lưu ảnh CCCD (mặt sau) thất bại"})
		return
	}

	// Cập nhật user -> auto VERIFIED
	if err := DB.Transaction(func(tx *gorm.DB) error {
		var u User
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&u, uid).Error; err != nil {
			return err
		}
		updates := map[string]any{
			"k_y_c_status":     "VERIFIED",
			"k_y_c_full_name":  fullName,
			"k_y_c_dob":        dob,
			"k_y_c_number":     number,
			"k_y_c_front_path": fFront,
			"k_y_c_back_path":  fBack,
		}
		return tx.Model(&u).Updates(updates).Error
	}); err != nil {
		c.JSON(500, gin.H{"error": "Cập nhật KYC thất bại"})
		return
	}

	c.JSON(200, gin.H{"message": "Xác minh thành công", "status": "VERIFIED"})
}
func adminServeKyc(c *gin.Context, side string) {
	uidStr := c.Param("uid")
	uid64, err := strconv.ParseUint(uidStr, 10, 64)
	if err != nil || uid64 == 0 {
		c.JSON(400, gin.H{"error": "ID không hợp lệ"})
		return
	}
	var u User
	if err := DB.First(&u, uint(uid64)).Error; err != nil {
		c.JSON(404, gin.H{"error": "User không tồn tại"})
		return
	}
	var file string
	if side == "front" {
		file = u.KYCFrontPath
	} else {
		file = u.KYCBackPath
	}
	if file == "" {
		c.JSON(404, gin.H{"error": "Chưa có ảnh " + side})
		return
	}
	p := filepath.Join(kycAbs, file)
	c.File(p)
}
func adminServeKycFront(c *gin.Context) {
	uid64, _ := strconv.ParseUint(c.Param("userId"), 10, 64)
	uid := uint(uid64)
	var u User
	if err := DB.Select("kyc_front_path").First(&u, uid).Error; err != nil || u.KYCFrontPath == "" {
		c.JSON(404, gin.H{"error": "Không có ảnh KYC front"})
		return
	}
	path := filepath.Join(kycAbs, u.KYCFrontPath)
	c.File(path)
}

func adminServeKycBack(c *gin.Context) {
	uid64, _ := strconv.ParseUint(c.Param("userId"), 10, 64)
	uid := uint(uid64)
	var u User
	if err := DB.Select("kyc_back_path").First(&u, uid).Error; err != nil || u.KYCBackPath == "" {
		c.JSON(404, gin.H{"error": "Không có ảnh KYC back"})
		return
	}
	path := filepath.Join(kycAbs, u.KYCBackPath)
	c.File(path)
}

/* ===== AUTH & PROFILE ===== */
func registerHandler(c *gin.Context) {
	var req RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	username := strings.ToLower(strings.TrimSpace(req.Username))
	if username == "" {
		c.JSON(400, gin.H{"error": "Thiếu username"})
		return
	}
	// chặn trùng username
	var exists int64
	_ = DB.Model(&User{}).Where("username = ?", username).Count(&exists)
	if exists > 0 {
		c.JSON(409, gin.H{"error": "Username đã tồn tại"})
		return
	}

	if err := DB.Transaction(func(tx *gorm.DB) error {
		hash, _ := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
		code, err := generateUniqueRefCode(tx, 8)
		if err != nil {
			return fmt.Errorf("gen refcode: %w", err)
		}

		// tìm upline theo referral_code hoặc username
		var referredBy *uint
		ref := strings.TrimSpace(req.Ref)
		if ref != "" {
			var inviter User
			if err := tx.Where("LOWER(referral_code)=? OR LOWER(username)=?",
				strings.ToLower(ref), strings.ToLower(ref)).
				Select("id,username").First(&inviter).Error; err == nil && inviter.ID != 0 {
				referredBy = &inviter.ID
			}
		}

		displayName := strings.TrimSpace(req.Nickname)
		if displayName == "" {
			displayName = strings.TrimSpace(req.Name)
		}
		u := User{
			Username:     username,
			Name:         displayName,
			Phone:        strings.TrimSpace(req.Phone),
			PasswordHash: string(hash),
			Role:         "user",
			Coins:        0,
			TotalTopup:   0,
			VIPLevel:     0,
			ReferralCode: &code,
			ReferredBy:   referredBy,
		}
		if err := tx.Create(&u).Error; err != nil {
			return fmt.Errorf("create user: %w", err)
		}
		return nil
	}); err != nil {
		c.JSON(500, gin.H{"error": "Không thể tạo người dùng"})
		return
	}
	c.JSON(201, gin.H{"message": "Đăng ký thành công"})
}

func invAdd(tx *gorm.DB, userID uint, code string, qty int64) error {
	var it InventoryItem
	err := tx.Where("user_id=? AND code=?", userID, code).First(&it).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			it = InventoryItem{UserID: userID, Code: code, Qty: 0}
			if err := tx.Create(&it).Error; err != nil {
				return err
			}
		} else {
			return err
		}
	}
	return tx.Model(&it).Update("qty", gorm.Expr("qty + ?", qty)).Error
}

func invSub(tx *gorm.DB, userID uint, code string, qty int64) error {
	var it InventoryItem
	if err := tx.Where("user_id=? AND code=?", userID, code).First(&it).Error; err != nil {
		return fmt.Errorf("không có vật phẩm %s", code)
	}
	if it.Qty < qty {
		return fmt.Errorf("vật phẩm %s không đủ", code)
	}
	return tx.Model(&it).Update("qty", gorm.Expr("qty - ?", qty)).Error
}

// trả về: result:"COIN"|"DRAGON_BALL", code, amount, coins, inv(map)
func chestOpenHandler(c *gin.Context) {
	uid := uint(c.MustGet("claims").(jwt.MapClaims)["sub"].(float64))

	var user User
	if err := DB.First(&user, uid).Error; err != nil {
		c.JSON(404, gin.H{"error": "User không tồn tại"})
		return
	}
	if user.Coins < CHEST_COST {
		c.JSON(400, gin.H{"error": "Số dư không đủ (50 coin/ lần)"})
		return
	}

	type result struct {
		kind   string // "COIN" | "DRAGON_BALL"
		code   string // "DB1".."DB7" nếu là DRAGON_BALL
		amount int64  // 100/10 coin hoặc 1 viên
	}

	// Xác suất: 2% -> 100 coin, 3% -> Dragon Ball ngẫu nhiên, còn lại -> 10 coin
	pick := func() result {
		n, _ := rand.Int(rand.Reader, big.NewInt(10000)) // 0..9999
		x := n.Int64()
		switch {
		case x < 200: // 2%
			return result{"COIN", "", 100}
		case x < 500: // +3% = 5% đầu
			star := randInt(1, 7) // dùng helper global randInt(min,max)
			return result{"DRAGON_BALL", fmt.Sprintf("DB%d", star), 1}
		default:
			return result{"COIN", "", 10}
		}
	}

	var out result
	if err := DB.Transaction(func(tx *gorm.DB) error {
		// trừ phí mở
		if err := tx.Model(&User{}).
			Where("id = ?", user.ID).
			Update("coins", gorm.Expr("coins - ?", CHEST_COST)).Error; err != nil {
			return err
		}

		// bốc quà
		out = pick()
		if out.kind == "COIN" {
			if err := tx.Model(&User{}).
				Where("id = ?", user.ID).
				Update("coins", gorm.Expr("coins + ?", out.amount)).Error; err != nil {
				return err
			}
		} else {
			if err := invAdd(tx, user.ID, out.code, out.amount); err != nil {
				return err
			}
		}

		// log
		return tx.Create(&ChestTxn{
			UserID:       user.ID,
			Cost:         CHEST_COST,
			RewardKind:   out.kind,
			RewardCode:   out.code,
			RewardAmount: out.amount,
		}).Error
	}); err != nil {
		c.JSON(500, gin.H{"error": "Mở rương thất bại"})
		return
	}

	// coins & túi đồ hiện tại
	DB.Select("coins").First(&user, user.ID)
	inv := map[string]int64{}
	var items []InventoryItem
	DB.Where("user_id = ?", user.ID).Find(&items)
	for _, it := range items {
		inv[it.Code] = it.Qty
	}

	c.JSON(200, gin.H{
		"result": out.kind,
		"code":   out.code,
		"amount": out.amount,
		"coins":  user.Coins,
		"inv":    inv,
	})
}

func inventoryHandler(c *gin.Context) {
	uid := uint(c.MustGet("claims").(jwt.MapClaims)["sub"].(float64))
	var items []InventoryItem
	DB.Where("user_id=?", uid).Order("code asc").Find(&items)
	c.JSON(200, gin.H{"items": items})
}

func mergeDragonBallsHandler(c *gin.Context) {
	uid := uint(c.MustGet("claims").(jwt.MapClaims)["sub"].(float64))
	var user User
	if err := DB.First(&user, uid).Error; err != nil {
		c.JSON(404, gin.H{"error": "User không tồn tại"})
		return
	}

	if err := DB.Transaction(func(tx *gorm.DB) error {
		// đủ DB1..DB7 mỗi loại >=1?
		need := []string{"DB1", "DB2", "DB3", "DB4", "DB5", "DB6", "DB7"}
		for _, code := range need {
			var it InventoryItem
			if err := tx.Where("user_id=? AND code=?", uid, code).First(&it).Error; err != nil {
				return fmt.Errorf("Thiếu %s", code)
			}
			if it.Qty < 1 {
				return fmt.Errorf("Thiếu %s", code)
			}
		}
		// trừ mỗi loại 1
		for _, code := range need {
			if err := invSub(tx, uid, code, 1); err != nil {
				return err
			}
		}
		// cộng thưởng
		return tx.Model(&user).Update("coins", gorm.Expr("coins + ?", MERGE_REWARD)).Error
	}); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	DB.First(&user, uid)
	c.JSON(200, gin.H{"message": "Hợp nhất thành công", "coins": user.Coins})
}

// POST /private/market/list  { code:"DB1", qty:1, pricePerUnit:500 }
func marketListHandler(c *gin.Context) {
	uid := uint(c.MustGet("claims").(jwt.MapClaims)["sub"].(float64))
	var req struct {
		Code         string `json:"code"`
		Qty          int64  `json:"qty"`
		PricePerUnit int64  `json:"pricePerUnit"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.Qty <= 0 || req.PricePerUnit <= 0 {
		c.JSON(400, gin.H{"error": "Dữ liệu không hợp lệ"})
		return
	}
	req.Code = strings.ToUpper(strings.TrimSpace(req.Code))

	if err := DB.Transaction(func(tx *gorm.DB) error {
		if err := invSub(tx, uid, req.Code, req.Qty); err != nil {
			return err
		}
		return tx.Create(&MarketListing{
			SellerID: uid, Code: req.Code, Qty: req.Qty, PricePerUnit: req.PricePerUnit, IsActive: true,
		}).Error
	}); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	c.JSON(200, gin.H{"message": "Đã đăng bán"})
}

// GET /market?code=DB1
// GET /market?code=DB1
func marketQueryHandler(c *gin.Context) {
	code := strings.ToUpper(strings.TrimSpace(c.Query("code")))
	q := DB.Model(&MarketListing{}).Where("is_active = 1 AND qty > 0")
	if code != "" {
		q = q.Where("code = ?", code)
	}

	type Row struct {
		ID           uint   `json:"id"`
		Code         string `json:"code"`
		Qty          int64  `json:"qty"`
		PricePerUnit int64  `json:"pricePerUnit"`
		SellerID     uint   `json:"sellerId"`
		SellerEmail  string `json:"sellerEmail"` // alias username cho FE
	}
	var rows []Row
	q.Order("price_per_unit asc, id asc").
		Joins("LEFT JOIN users u ON u.id = market_listings.seller_id").
		Select("market_listings.id, market_listings.code, market_listings.qty, market_listings.price_per_unit, u.id as seller_id, u.username as seller_email").
		Scan(&rows)
	c.JSON(200, gin.H{"rows": rows})
}

// POST /private/market/buy { listingId, qty }
// POST /private/market/buy { listingId, qty }
func marketBuyHandler(c *gin.Context) {
	buyerID := uint(c.MustGet("claims").(jwt.MapClaims)["sub"].(float64))
	var req struct {
		ListingID uint  `json:"listingId"`
		Qty       int64 `json:"qty"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.Qty <= 0 {
		c.JSON(400, gin.H{"error": "Dữ liệu không hợp lệ"})
		return
	}

	if err := DB.Transaction(func(tx *gorm.DB) error {
		var l MarketListing
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&l, req.ListingID).Error; err != nil {
			return fmt.Errorf("Listing không tồn tại")
		}
		if !l.IsActive || l.Qty < req.Qty {
			return fmt.Errorf("Số lượng không đủ")
		}

		// ❌ Không cho người bán tự mua
		if l.SellerID == buyerID {
			return fmt.Errorf("Bạn không thể mua sản phẩm của chính mình. Hãy rút lại nếu muốn.")
		}

		var buyer, seller User
		if err := tx.First(&buyer, buyerID).Error; err != nil {
			return err
		}
		if err := tx.First(&seller, l.SellerID).Error; err != nil {
			return err
		}

		total := req.Qty * l.PricePerUnit
		if buyer.Coins < total {
			return fmt.Errorf("Số dư không đủ")
		}

		// trừ người mua, cộng người bán
		if err := tx.Model(&buyer).Update("coins", gorm.Expr("coins - ?", total)).Error; err != nil {
			return err
		}
		if err := tx.Model(&seller).Update("coins", gorm.Expr("coins + ?", total)).Error; err != nil {
			return err
		}

		// cộng vật phẩm cho buyer
		if err := invAdd(tx, buyer.ID, l.Code, req.Qty); err != nil {
			return err
		}

		// trừ số lượng listing
		left := l.Qty - req.Qty
		up := map[string]any{"qty": left}
		if left == 0 {
			up["is_active"] = false
		}
		if err := tx.Model(&l).Updates(up).Error; err != nil {
			return err
		}

		return nil
	}); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	c.JSON(200, gin.H{"message": "Mua thành công"})
}

// POST /private/market/withdraw { listingId, qty? }
// - Nếu không truyền qty: rút toàn bộ phần còn lại.
// - Nếu truyền qty: rút đúng số lượng đó (không vượt quá phần còn lại).
func marketWithdrawHandler(c *gin.Context) {
	sellerID := uint(c.MustGet("claims").(jwt.MapClaims)["sub"].(float64))
	var req struct {
		ListingID uint   `json:"listingId"`
		Qty       *int64 `json:"qty"` // optional
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": "Dữ liệu không hợp lệ"})
		return
	}

	if err := DB.Transaction(func(tx *gorm.DB) error {
		var l MarketListing
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&l, req.ListingID).Error; err != nil {
			return fmt.Errorf("Listing không tồn tại")
		}
		if l.SellerID != sellerID {
			return fmt.Errorf("Bạn không phải chủ bài đăng này")
		}
		if !l.IsActive || l.Qty <= 0 {
			return fmt.Errorf("Listing đã hết hoặc không hoạt động")
		}

		// xác định lượng rút
		var back int64 = l.Qty
		if req.Qty != nil && *req.Qty > 0 && *req.Qty < l.Qty {
			back = *req.Qty
		}

		// trả vật phẩm về túi đồ
		if err := invAdd(tx, sellerID, l.Code, back); err != nil {
			return err
		}

		// cập nhật listing
		left := l.Qty - back
		up := map[string]any{"qty": left}
		if left == 0 {
			up["is_active"] = false
		}
		if err := tx.Model(&l).Updates(up).Error; err != nil {
			return err
		}

		return nil
	}); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	c.JSON(200, gin.H{"message": "Đã rút lại sản phẩm về túi"})
}

func loginHandler(c *gin.Context) {
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	var user User
	if err := DB.Where("username = ?", strings.ToLower(req.Username)).First(&user).Error; err != nil {
		c.JSON(401, gin.H{"error": "Sai username hoặc mật khẩu"})
		return
	}
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)); err != nil {
		c.JSON(401, gin.H{"error": "Sai username hoặc mật khẩu"})
		return
	}
	claims := jwt.MapClaims{
		"sub": user.ID, "username": user.Username, "role": user.Role,
		"exp": time.Now().Add(24 * time.Hour).Unix(),
	}
	t := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, _ := t.SignedString([]byte(JWT_SECRET))
	user.PasswordHash = ""
	c.JSON(200, AuthResponse{Token: signed, User: user})
}

func meHandler(c *gin.Context) {
	uid := uint(c.MustGet("claims").(jwt.MapClaims)["sub"].(float64))
	var user User
	if err := DB.First(&user, uid).Error; err != nil {
		c.JSON(404, gin.H{"error": "User không tồn tại"})
		return
	}
	user.PasswordHash = ""
	c.JSON(200, gin.H{"user": user})
}
func withdrawHistoryHandler(c *gin.Context) {
	uid := uint(c.MustGet("claims").(jwt.MapClaims)["sub"].(float64))
	var rows []WithdrawRow
	DB.Table("withdraw_txns w").
		Select("w.id, w.amount, w.note, w.created_at, a.username AS admin_email").
		Joins("LEFT JOIN users a ON a.id = w.admin_id").
		Where("w.user_id = ?", uid).
		Order("w.id DESC").Scan(&rows)
	c.JSON(200, gin.H{"rows": rows})
}
func updateProfileHandler(c *gin.Context) {
	uid := uint(c.MustGet("claims").(jwt.MapClaims)["sub"].(float64))

	var req ProfileUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	var user User
	if err := DB.First(&user, uid).Error; err != nil {
		c.JSON(404, gin.H{"error": "User không tồn tại"})
		return
	}

	updates := map[string]any{
		"name":       strings.TrimSpace(req.Name),
		"phone":      strings.TrimSpace(req.Phone),
		"avatar_url": strings.TrimSpace(req.AvatarURL),
	}
	if err := DB.Model(&user).Updates(updates).Error; err != nil {
		c.JSON(500, gin.H{"error": "Không thể cập nhật hồ sơ"})
		return
	}

	DB.First(&user, uid)
	user.PasswordHash = ""
	c.JSON(200, gin.H{"user": user, "message": "Cập nhật thành công"})
}

/* ===== UPLOAD AVATAR ===== */
func ensureUploadsDirAbs() (string, error) {
	abs, err := filepath.Abs(UPLOAD_DIR)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(abs, 0755); err != nil {
		return "", err
	}
	return abs, nil
}
func ensureDirAbs(dir string) (string, error) {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(abs, 0755); err != nil {
		return "", err
	}
	return abs, nil
}

func ensureKycDirAbs() (string, error) {
	abs, err := filepath.Abs(KYC_DIR)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(abs, 0755); err != nil {
		return "", err
	}
	return abs, nil
}

// POST /private/upload  (multipart/form-data, field: "file")
func uploadAvatarHandler(c *gin.Context) {
	file, err := c.FormFile("file")
	if err != nil {
		c.JSON(400, gin.H{"error": "Không có file"})
		return
	}
	filename := fmt.Sprintf("%d_%s", time.Now().UnixNano(), filepath.Base(file.Filename))
	savePath := filepath.Join(uploadsAbs, filename)

	if err := c.SaveUploadedFile(file, savePath); err != nil {
		c.JSON(500, gin.H{"error": "Lưu file thất bại"})
		return
	}
	url := "/uploads/" + filename
	c.JSON(200, gin.H{"url": url})
}

/* ===== VIP / COIN API ===== */
func getVipTiersHandler(c *gin.Context) {
	var tiers []VipTier
	DB.Order("level asc").Find(&tiers)
	c.JSON(200, gin.H{"tiers": tiers})
}

func getWalletHandler(c *gin.Context) {
	uid := uint(c.MustGet("claims").(jwt.MapClaims)["sub"].(float64))
	var user User
	if err := DB.First(&user, uid).Error; err != nil {
		c.JSON(404, gin.H{"error": "User không tồn tại"})
		return
	}
	c.JSON(200, gin.H{
		"coins":      user.Coins,
		"totalTopup": user.TotalTopup,
		"vipLevel":   user.VIPLevel,
	})
}

func adminWithdrawHandler(c *gin.Context) {
	claims := c.MustGet("claims").(jwt.MapClaims)
	adminID := uint(claims["sub"].(float64))

	var req WithdrawRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	var user User
	if err := DB.First(&user, req.UserID).Error; err != nil {
		c.JSON(404, gin.H{"error": "User không tồn tại"})
		return
	}

	if err := DB.Transaction(func(tx *gorm.DB) error {
		// khoá hàng để tránh race
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&user, user.ID).Error; err != nil {
			return err
		}
		if user.Coins < req.Amount {
			return fmt.Errorf("Số dư không đủ (cần %d, hiện %d)", req.Amount, user.Coins)
		}
		// trừ coin
		if err := tx.Model(&user).Update("coins", gorm.Expr("coins - ?", req.Amount)).Error; err != nil {
			return err
		}
		// log rút tiền
		return tx.Create(&WithdrawTxn{
			UserID:  user.ID,
			AdminID: adminID,
			Amount:  req.Amount,
			Note:    strings.TrimSpace(req.Note),
		}).Error
	}); err != nil {
		log.Println("withdraw error:", err)
		c.JSON(500, gin.H{"error": "Rút coin thất bại: " + err.Error()})
		return
	}

	c.JSON(200, gin.H{"message": "Rút coin thành công", "userId": user.ID})
}

// GET /admin/users?vipLevel=1&nickname=abc&email=x@y.com
func adminSearchUsersHandler(c *gin.Context) {
	vipLevel := strings.TrimSpace(c.Query("vipLevel"))
	nickname := strings.TrimSpace(c.Query("nickname"))
	username := strings.TrimSpace(c.Query("username"))
	// (tương thích tham số cũ 'email' -> dùng làm username filter)
	if username == "" {
		username = strings.TrimSpace(c.Query("email"))
	}

	q := DB.Model(&User{})
	if vipLevel != "" {
		if lvl, err := strconv.Atoi(vipLevel); err == nil {
			q = q.Where("v_ip_level = ?", lvl)
		}
	}
	if nickname != "" {
		like := "%" + strings.ToLower(nickname) + "%"
		q = q.Where("LOWER(name) LIKE ?", like)
	}
	if username != "" {
		like := "%" + strings.ToLower(username) + "%"
		q = q.Where("LOWER(username) LIKE ?", like)
	}

	var users []User
	if err := q.Order("v_ip_level DESC, id ASC").Find(&users).Error; err != nil {
		c.JSON(500, gin.H{"error": "Query failed"})
		return
	}

	rows := make([]AdminUserRow, 0, len(users))
	for _, u := range users {
		rows = append(rows, AdminUserRow{
			ID: u.ID, Nickname: u.Name, Username: u.Username,
			VIPLevel: u.VIPLevel, TotalTopup: u.TotalTopup, Coins: u.Coins,
		})
	}
	c.JSON(200, gin.H{"rows": rows})
}

func adminTopupHandler(c *gin.Context) {
	claims := c.MustGet("claims").(jwt.MapClaims)
	adminID := uint(claims["sub"].(float64))

	var req TopupRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	var user User
	if err := DB.First(&user, req.UserID).Error; err != nil {
		c.JSON(404, gin.H{"error": "User không tồn tại"})
		return
	}

	err := DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&user).Updates(map[string]interface{}{
			"coins":       gorm.Expr("coins + ?", req.Amount),
			"total_topup": gorm.Expr("total_topup + ?", req.Amount),
		}).Error; err != nil {
			return err
		}
		txn := CoinTxn{UserID: user.ID, AdminID: adminID, Amount: req.Amount, Note: strings.TrimSpace(req.Note)}
		return tx.Create(&txn).Error
	})
	if err != nil {
		c.JSON(500, gin.H{"error": "Nạp coin thất bại"})
		return
	}
	c.JSON(200, gin.H{"message": "Nạp coin thành công", "userId": user.ID})
}

// DELETE /admin/users/:id  (hard delete)
func adminHardDeleteUserHandler(c *gin.Context) {
	idStr := c.Param("id")
	uid64, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil || uid64 == 0 {
		c.JSON(400, gin.H{"error": "ID không hợp lệ"})
		return
	}
	uid := uint(uid64)

	var u User
	if err := DB.First(&u, uid).Error; err != nil {
		c.JSON(404, gin.H{"error": "User không tồn tại"})
		return
	}

	if err := DB.Transaction(func(tx *gorm.DB) error {
		// dọn tham chiếu/upline để tránh lỗi FK
		if err := tx.Model(&User{}).Where("referred_by = ?", uid).Update("referred_by", nil).Error; err != nil {
			return fmt.Errorf("clear referred_by: %w", err)
		}
		// xóa các lịch sử liên quan
		if err := tx.Where("from_id = ? OR to_id = ?", uid, uid).Delete(&TransferTxn{}).Error; err != nil {
			return fmt.Errorf("del transfer_txns: %w", err)
		}
		if err := tx.Where("user_id = ? OR admin_id = ?", uid, uid).Delete(&CoinTxn{}).Error; err != nil {
			return fmt.Errorf("del coin_txns: %w", err)
		}
		if err := tx.Where("inviter_id = ? OR invitee_id = ?", uid, uid).Delete(&ReferralReward{}).Error; err != nil {
			return fmt.Errorf("del referral_rewards: %w", err)
		}
		if err := tx.Where("user_id = ?", uid).Delete(&VipPurchaseTxn{}).Error; err != nil {
			return fmt.Errorf("del vip_purchase_txns: %w", err)
		}
		if err := tx.Where("buyer_id = ? OR beneficiary_id = ?", uid, uid).Delete(&CommissionTxn{}).Error; err != nil {
			return fmt.Errorf("del commission_txns: %w", err)
		}
		// ❗ xóa cứng user
		if err := tx.Unscoped().Delete(&User{}, uid).Error; err != nil {
			return fmt.Errorf("del user: %w", err)
		}
		return nil
	}); err != nil {
		log.Println("admin hard delete error:", err)
		c.JSON(500, gin.H{"error": "Xoá tài khoản thất bại"})
		return
	}

	c.Status(204)
}

/* ===== BUSINESS: BUY VIP & TRANSFER ===== */
// POST /private/buy-vip  { level }
func buyVipHandler(c *gin.Context) {
	uid := uint(c.MustGet("claims").(jwt.MapClaims)["sub"].(float64))

	var user User
	if err := DB.First(&user, uid).Error; err != nil {
		c.JSON(404, gin.H{"error": "User không tồn tại"})
		return
	}

	// Giá VIP: lấy từ vip_tiers level=1, fallback 10000
	var price int64 = 10000
	if t := (VipTier{}); DB.Where("level = ?", 1).First(&t).Error == nil && t.MinTopup > 0 {
		price = t.MinTopup
	}

	if user.VIPLevel >= 1 {
		c.JSON(400, gin.H{"error": "Bạn đã là VIP"})
		return
	}
	if user.Coins < price {
		c.JSON(400, gin.H{"error": "Số dư không đủ"})
		return
	}

	if err := DB.Transaction(func(tx *gorm.DB) error {
		// trừ coin & set VIP = 1
		if err := tx.Model(&User{}).Where("id = ?", user.ID).Update("coins", gorm.Expr("coins - ?", price)).Error; err != nil {
			return err
		}
		old := user.VIPLevel
		if err := tx.Model(&User{}).Where("id = ?", user.ID).Update("VIPLevel", 1).Error; err != nil {
			return err
		}
		if err := tx.Create(&VipPurchaseTxn{UserID: user.ID, Level: 1, Price: price, OldLevel: old}).Error; err != nil {
			return err
		}

		// chia hoa hồng 9 tầng, mỗi tầng 10%
		uplines, _ := getUplines(tx, user.ReferredBy, 9)
		allocated := 0
		for i, up := range uplines {
			if allocated >= 100 {
				break
			}
			pct := 10
			if left := 100 - allocated; pct > left {
				pct = left
			}
			amt := (price * int64(pct)) / 100
			if amt > 0 {
				if err := tx.Model(&User{}).Where("id = ?", up.ID).Update("coins", gorm.Expr("coins + ?", amt)).Error; err != nil {
					return fmt.Errorf("upline depth %d: %w", i+1, err)
				}
				if err := tx.Create(&CommissionTxn{
					BuyerID: user.ID, BeneficiaryID: &up.ID, Depth: i + 1,
					Percent: pct, Amount: amt, Kind: "UPLINE", VipLevelBought: 1,
				}).Error; err != nil {
					return err
				}
			}
			allocated += pct
		}

		// phần còn lại (nếu còn) -> log cho admin (không cộng coin admin)
		if allocated < 100 {
			if admin, err := getAnyAdmin(tx); err == nil {
				rem := 100 - allocated
				if rem > 0 {
					amt := (price * int64(rem)) / 100
					if err := tx.Create(&CommissionTxn{
						BuyerID: user.ID, BeneficiaryID: &admin.ID, Depth: 0,
						Percent: rem, Amount: amt, Kind: "ADMIN", VipLevelBought: 1,
					}).Error; err != nil {
						return err
					}
				}
			} else {
				return fmt.Errorf("no admin account found")
			}
		}
		return nil
	}); err != nil {
		c.JSON(500, gin.H{"error": "Mua VIP thất bại"})
		return
	}

	DB.First(&user, user.ID)
	c.JSON(200, gin.H{"message": "Mua VIP thành công", "level": user.VIPLevel, "coins": user.Coins})
}

// POST /private/transfer  { toEmail, amount, note }
func transferHandler(c *gin.Context) {
	var req TransferRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": "Dữ liệu không hợp lệ: " + err.Error()})
		return
	}
	toUsername := strings.ToLower(strings.TrimSpace(req.ToUsername))

	// Lấy người gửi từ token
	uid := uint(c.MustGet("claims").(jwt.MapClaims)["sub"].(float64))
	var from User
	if err := DB.First(&from, uid).Error; err != nil {
		c.JSON(404, gin.H{"error": "Người gửi không tồn tại"})
		return
	}

	// Không tự chuyển cho chính mình
	if toUsername == strings.ToLower(from.Username) {
		c.JSON(400, gin.H{"error": "Không thể tự chuyển cho chính mình"})
		return
	}

	// Kiểm tra đã đặt PIN chưa
	if strings.TrimSpace(from.TxnPinHash) == "" {
		c.JSON(400, gin.H{"error": "Bạn chưa thiết lập mã bảo mật (PIN). Hãy vào Hồ sơ > Bảo mật để đặt PIN 6 số."})
		return
	}
	// So khớp PIN
	if err := bcrypt.CompareHashAndPassword([]byte(from.TxnPinHash), []byte(req.TxnPin)); err != nil {
		c.JSON(400, gin.H{"error": "Mã PIN không đúng"})
		return
	}

	// Tìm người nhận theo username
	var to User
	if err := DB.Where("LOWER(username) = ?", toUsername).First(&to).Error; err != nil {
		c.JSON(404, gin.H{"error": "Người nhận không tồn tại"})
		return
	}

	// Phí 0.5% (làm tròn lên)
	fee := (req.Amount*5 + 999) / 1000 // ceil(amount*0.005)
	totalDebit := req.Amount + fee

	// Giao dịch
	if err := DB.Transaction(func(tx *gorm.DB) error {
		// khoá người gửi
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&from, from.ID).Error; err != nil {
			return err
		}
		if from.Coins < totalDebit {
			return fmt.Errorf("Số dư không đủ (cần %d, hiện %d)", totalDebit, from.Coins)
		}
		// trừ người gửi
		if err := tx.Model(&from).Update("coins", gorm.Expr("coins - ?", totalDebit)).Error; err != nil {
			return err
		}
		// cộng người nhận
		if err := tx.Model(&to).Update("coins", gorm.Expr("coins + ?", req.Amount)).Error; err != nil {
			return err
		}
		// log
		return tx.Create(&TransferTxn{
			FromID: from.ID, ToID: to.ID,
			Amount: req.Amount, Fee: fee, Note: strings.TrimSpace(req.Note),
		}).Error
	}); err != nil {
		c.JSON(500, gin.H{"error": "Chuyển coin thất bại: " + err.Error()})
		return
	}

	c.JSON(200, gin.H{"message": "Chuyển coin thành công", "fee": fee, "debit": totalDebit})
}

func updateKycHandler(c *gin.Context) {
	uid := uint(c.MustGet("claims").(jwt.MapClaims)["sub"].(float64))

	var req kycUpdateReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	// Lấy filename an toàn (chỉ lưu tên file, không lưu URL đầy đủ)
	front := strings.TrimSpace(req.FrontURL)
	back := strings.TrimSpace(req.BackURL)
	if front == "" && req.FrontPath != "" {
		front = req.FrontPath
	}
	if back == "" && req.BackPath != "" {
		back = req.BackPath
	}
	if front == "" || back == "" {
		c.JSON(400, gin.H{"error": "Thiếu ảnh mặt trước hoặc mặt sau"})
		return
	}
	frontFile := filepath.Base(front)
	backFile := filepath.Base(back)

	var u User
	if err := DB.First(&u, uid).Error; err != nil {
		c.JSON(404, gin.H{"error": "User không tồn tại"})
		return
	}

	// ✅ Auto-approve: lưu file + set APPROVED ngay
	if err := DB.Model(&u).Updates(map[string]any{
		"kyc_front_path": frontFile,
		"kyc_back_path":  backFile,
		"kyc_status":     "APPROVED",
	}).Error; err != nil {
		c.JSON(500, gin.H{"error": "Cập nhật KYC thất bại"})
		return
	}

	c.JSON(200, gin.H{"message": "Đã xác minh KYC (tự động phê duyệt)"})
}
func adminGetKycImage(c *gin.Context) {
	// GET /admin/kyc-file/:userId/:side  (side=front|back)
	uidStr := c.Param("userId")
	side := c.Param("side")
	uid64, _ := strconv.ParseUint(uidStr, 10, 64)
	uid := uint(uid64)

	var u User
	if err := DB.Select("kyc_front_path, kyc_back_path").First(&u, uid).Error; err != nil {
		c.JSON(404, gin.H{"error": "User không tồn tại"})
		return
	}

	var fname string
	if side == "front" {
		fname = u.KYCFrontPath
	} else {
		fname = u.KYCBackPath
	}
	if fname == "" {
		c.JSON(404, gin.H{"error": "Chưa có ảnh"})
		return
	}

	// nếu bạn để chung trong uploads/
	// KHUYẾN NGHỊ: tách riêng KYC_DIR = "uploads/kyc"
	full := filepath.Join(uploadsAbs, fname)
	c.File(full)
}

/* ===== REFERRAL INFO ===== */
// GET /private/referral-info
func referralInfoHandler(c *gin.Context) {
	uid := uint(c.MustGet("claims").(jwt.MapClaims)["sub"].(float64))
	var u User
	if err := DB.First(&u, uid).Error; err != nil {
		c.JSON(404, gin.H{"error": "User không tồn tại"})
		return
	}

	var cnt, total int64
	DB.Model(&ReferralReward{}).Where("inviter_id = ?", uid).Count(&cnt)
	DB.Model(&ReferralReward{}).Select("COALESCE(SUM(amount),0)").Where("inviter_id = ?", uid).Scan(&total)

	codeStr := ""
	if u.ReferralCode != nil {
		codeStr = *u.ReferralCode
	}
	link := fmt.Sprintf("%s?ref=%s", CORS_ORIGIN, codeStr)
	c.JSON(200, gin.H{"code": codeStr, "link": link, "count": cnt, "total": total})
}

/* ===== MAIN & CORS ===== */
func main() {
	connectDB()

	r := gin.Default()
	r.MaxMultipartMemory = 16 << 20 // 16 MiB

	// CORS
	r.Use(func(c *gin.Context) {
		origin := c.Request.Header.Get("Origin")
		if origin == "" {
			origin = CORS_ORIGIN
		}
		c.Writer.Header().Set("Access-Control-Allow-Origin", origin)
		c.Writer.Header().Set("Vary", "Origin")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(204)
			return
		}
		c.Next()
	})

	// serve uploads
	if abs, err := ensureUploadsDirAbs(); err == nil {
		uploadsAbs = abs
		log.Println("📁 Serving /uploads from:", uploadsAbs)
		r.Static("/uploads", uploadsAbs)
	} else {
		log.Println("⚠️  Không thể tạo thư mục uploads:", err)
	}
	// KYC dir (KHÔNG public)
	if abs, err := ensureDirAbs(KYC_DIR); err == nil {
		kycAbs = abs
		log.Println("🔒 KYC dir:", kycAbs)
	} else {
		log.Println("⚠️  Không thể tạo thư mục KYC:", err)
	}

	// Public
	r.POST("/register", registerHandler)
	r.POST("/login", loginHandler)
	r.GET("/vip-tiers", getVipTiersHandler)
	r.GET("/market", marketQueryHandler)
	r.POST("/forgot-password", forgotPasswordHandler)

	// Private
	priv := r.Group("/private")
	priv.Use(authRequired())
	priv.GET("/me", meHandler)
	priv.PUT("/profile", updateProfileHandler)
	priv.GET("/wallet", getWalletHandler)
	priv.POST("/upload", uploadAvatarHandler)
	priv.POST("/transfer", transferHandler)
	priv.GET("/referral-info", referralInfoHandler)
	priv.POST("/buy-vip", buyVipHandler)
	priv.GET("/history/withdraws", withdrawHistoryHandler)

	priv.GET("/history/topups", topupHistoryHandler)
	priv.GET("/history/transfers", transferHistoryHandler)
	priv.GET("/history/vip", vipHistoryHandler)
	priv.GET("/history/commissions", myCommissionHistoryHandler)
	priv.POST("/chest-open", chestOpenHandler)
	priv.GET("/inventory", inventoryHandler)
	priv.POST("/merge-dragon", mergeDragonBallsHandler)

	priv.POST("/market/list", marketListHandler)
	priv.POST("/market/buy", marketBuyHandler)
	priv.POST("/market/withdraw", marketWithdrawHandler)
	priv.POST("/change-password", changePasswordHandler)
	priv.PUT("/change-password", changePasswordHandler)

	priv.POST("/update-security", updateSecurityHandler)
	priv.POST("/kyc-submit", kycSubmitHandler)
	priv.PUT("/security", updateSecurityHandler)
	priv.PUT("/kyc", updateKycHandler)
	priv.POST("/kyc", kycSubmitHandler)

	// Admin
	admin := r.Group("/admin")
	admin.Use(authRequired(), adminRequired())
	admin.POST("/topup", adminTopupHandler)
	admin.GET("/users", adminSearchUsersHandler)
	admin.DELETE("/users/:id", adminHardDeleteUserHandler)
	admin.POST("/withdraw", adminWithdrawHandler)
	admin.GET("/kyc/:userId/front", adminServeKycFront)
	admin.GET("/kyc/:userId/back", adminServeKycBack)
	admin.GET("/kyc-file/:userId/:side", adminGetKycImage)
	admin.GET("/users/:id", adminUserDetailHandler)

	fmt.Println("🚀 Server running at :" + PORT)
	_ = r.Run(":" + PORT)
}
