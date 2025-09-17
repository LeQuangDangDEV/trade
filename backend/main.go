package main

import (
	"crypto/rand"
	"errors"
	"fmt"
	"log"
	"math/big"
	mathrand "math/rand"
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

type DashboardOverview struct {
	TotalAssets           int64 `json:"totalAssets"`
	F1Count               int64 `json:"f1Count"`
	F1CommissionTotal     int64 `json:"f1CommissionTotal"`
	SystemCount           int64 `json:"systemCount"`
	SystemCommissionTotal int64 `json:"systemCommissionTotal"`
	VipLevel              int   `json:"vipLevel"`
}
type DailyCommission struct {
	Day    int   `json:"day"`
	Amount int64 `json:"amount"`
}
type MonthlyCommissionResp struct {
	Year       int               `json:"year"`
	Month      int               `json:"month"`
	Days       []DailyCommission `json:"days"`
	MonthTotal int64             `json:"monthTotal"`
}
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
type DownlineRow struct {
	ID        uint   `json:"id"`
	Username  string `json:"username"`
	Depth     int    `json:"depth"`
	VIPLevel  int    ` json:"vipLevel"`
	CreatedAt string `json:"createdAt"`
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

	Role                 string `gorm:"type:enum('admin','user');default:'user';index" json:"role"`
	Coins                int64  `gorm:"not null;default:0" json:"coins"`
	TotalTopup           int64  `gorm:"not null;default:0" json:"totalTopup"`
	VIPLevel             int    `gorm:"column:v_ip_level;not null;default:0" json:"vipLevel"`
	BonusCoins           int64  `gorm:"not null;default:0" json:"bonusCoins"`
	Invite10VipBonusPaid bool   `gorm:"not null;default:false" json:"-"`

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
	// 🔹 Free spins (lượt quay miễn phí)
	FreeSpins      int `gorm:"not null;default:0" json:"freeSpins"`
	ChestOpenCount int `gorm:"not null;default:0"`
}
type LeaderboardRow struct {
	Rank     int    `json:"rank"`
	Username string `json:"username"`
	Score    int64  `json:"score"`
}
type MyRankResp struct {
	Rank     int    `json:"rank"`
	Username string `json:"username"`
	Score    int64  `json:"score"`
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
type RedeemReq struct {
	Code string `json:"code" binding:"required"`
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
	Note        string    `json:"note"`
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

type Notification struct {
	ID        uint      `gorm:"primaryKey"             json:"id"`
	UserID    uint      `gorm:"index;not null"         json:"userId"`
	Title     string    `gorm:"size:120;not null"      json:"title"`
	Body      string    `gorm:"size:500;not null"      json:"body"`
	IsRead    bool      `gorm:"not null;default:false" json:"isRead"`
	CreatedAt time.Time `json:"createdAt"`
}

type PromoCode struct {
	ID             uint       `gorm:"primaryKey"`
	Code           string     `gorm:"size:32;uniqueIndex;not null"`
	RewardFreeSpin int        `gorm:"not null;default:10"` // số lượt quay tặng
	MaxUses        *int       `gorm:""`                    // NULL => vô hạn
	UsedCount      int        `gorm:"not null;default:0"`
	ExpiresAt      *time.Time `gorm:"index"` // hết hạn sau 24h
	IsActive       bool       `gorm:"not null;default:true"`
	CreatedBy      *uint
	CreatedAt      time.Time
}
type PromoCodeUse struct {
	ID          uint      `gorm:"primaryKey"`
	PromoCodeID uint      `gorm:"index;not null"`
	UserID      uint      `gorm:"index;not null"`
	UsedAt      time.Time `gorm:"autoCreateTime"`

	// Mỗi (code, user) chỉ 1 lần
	_            struct{} `gorm:"uniqueIndex:uniq_code_user,composite"`
	PromoCodeID2 uint     `gorm:"-"` // dummy to keep tag line compile-friendly in older gorm
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
	UserID    uint      `gorm:"not null;uniqueIndex:uniq_user_code" json:"-"`
	Code      string    `gorm:"size:10;not null;uniqueIndex:uniq_user_code" json:"code"`
	Qty       int64     `gorm:"not null;default:0"   json:"qty"`
	CreatedAt time.Time `json:"-"`
	UpdatedAt time.Time `json:"-"`
}
type PromoBonusCode struct {
	ID         uint       `gorm:"primaryKey"`
	Code       string     `gorm:"size:32;uniqueIndex;not null"`
	BonusCoins int        `gorm:"not null;default:10"` // ⭐ mặc định 10
	MaxUses    int        `gorm:"not null;default:1"`  // ⭐ one-shot
	UsedCount  int        `gorm:"not null;default:0"`
	ExpiresAt  *time.Time `gorm:"index"`
	IsActive   bool       `gorm:"not null;default:true"`
	CreatedBy  *uint
	CreatedAt  time.Time
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
type DashOverview struct {
	TotalAssets           int64 `json:"totalAssets"` // coins + bonusCoins
	F1Count               int64 `json:"f1Count"`
	F1CommissionTotal     int64 `json:"f1CommissionTotal"`     // depth=1
	SystemCommissionTotal int64 `json:"systemCommissionTotal"` // depth 1..9
	SystemCount           int64 `json:"systemCount"`
}

type DayEarning struct {
	Day    int   `json:"day"`
	Amount int64 `json:"amount"`
}

type DownlineDashboardResp struct {
	UserID   uint              `json:"userId"`
	Username string            `json:"username"`
	Overview DashboardOverview `json:"overview"`
	Month    string            `json:"month"` // YYYY-MM
	Earnings []DayEarning      `json:"earnings"`
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
		&Notification{},
		&PromoCode{}, &PromoCodeUse{}, &PromoBonusCode{},
	); err != nil {
		log.Fatal("❌ AutoMigrate error:", err)
	}
	collapseInventoryDuplicates()
	seedVipTiers()
	fmt.Println("✅ DB migrated")
}

/* ===== HELPERS ===== */
func getAnyAdmin(tx *gorm.DB) (User, error) {
	var admin User
	err := tx.Where("role = ?", "admin").Order("id ASC").First(&admin).Error
	return admin, err
}

// kind = "f1" | "system"
func lbDepthCond(kind string) (string, []any) {
	switch strings.ToLower(kind) {
	case "f1":
		return "ct.depth = 1", nil
	default:
		return "ct.depth BETWEEN 1 AND 9", nil
	}
}

// Trả về danh sách ID F1 và toàn bộ F1..F9 (không trùng)
func downlineIDsByDepth(tx *gorm.DB, root uint, maxDepth int) (f1IDs []uint, allIDs []uint, err error) {
	current := []uint{root}
	seen := map[uint]bool{}

	for depth := 1; depth <= maxDepth; depth++ {
		var next []uint
		if err := tx.Model(&User{}).
			Where("referred_by IN ?", current).
			Pluck("id", &next).Error; err != nil {
			return nil, nil, err
		}
		if len(next) == 0 {
			break
		}
		if depth == 1 {
			f1IDs = append(f1IDs, next...)
		}
		for _, id := range next {
			if !seen[id] {
				seen[id] = true
				allIDs = append(allIDs, id)
			}
		}
		current = next
	}
	return
}

func randCode(n int) (string, error) {
	const letters = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, n)
	for i := range b {
		num, err := rand.Int(rand.Reader, big.NewInt(int64(len(letters))))
		if err != nil {
			return "", err
		}
		b[i] = letters[num.Int64()]
	}
	return string(b), nil
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
		if err := tx.Select("id, username, coins, v_ip_level, referred_by").
			First(&u, *cur).Error; err != nil {
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
func overviewStatsHandler(c *gin.Context) {
	uid := uint(c.MustGet("claims").(jwt.MapClaims)["sub"].(float64))

	var user User
	if err := DB.First(&user, uid).Error; err != nil {
		c.JSON(404, gin.H{"error": "User không tồn tại"})
		return
	}

	// a) KPI theo phần bạn nhận (UPLINE)
	var f1CommissionUser int64
	DB.Model(&CommissionTxn{}).
		Where("beneficiary_id = ? AND kind = ? AND depth = 1", uid, "UPLINE").
		Select("COALESCE(SUM(amount),0)").Scan(&f1CommissionUser)

	var systemCommissionUser int64
	DB.Model(&CommissionTxn{}).
		Where("beneficiary_id = ? AND kind = ? AND depth BETWEEN 1 AND 9", uid, "UPLINE").
		Select("COALESCE(SUM(amount),0)").Scan(&systemCommissionUser)

	// b) KPI tổng phát sinh trong cây của bạn (bao gồm ADMIN)
	f1IDs, allIDs, err := downlineIDsByDepth(DB, uid, 9)
	if err != nil {
		c.JSON(500, gin.H{"error": "Không lấy được thống kê"})
		return
	}
	var f1CommissionGross, systemCommissionGross int64
	if len(f1IDs) > 0 {
		DB.Model(&CommissionTxn{}).
			Where("buyer_id IN ?", f1IDs).
			Select("COALESCE(SUM(amount),0)").Scan(&f1CommissionGross)
	}
	if len(allIDs) > 0 {
		DB.Model(&CommissionTxn{}).
			Where("buyer_id IN ?", allIDs).
			Select("COALESCE(SUM(amount),0)").Scan(&systemCommissionGross)
	}

	totalAssets := user.Coins + user.BonusCoins

	c.JSON(200, gin.H{
		"totalAssets": totalAssets,
		"f1Count":     len(f1IDs),
		"systemCount": len(allIDs),

		// bạn nhận (UPLINE)
		"f1CommissionUser":     f1CommissionUser,
		"systemCommissionUser": systemCommissionUser,

		// tổng phát sinh (bao gồm ADMIN)
		"f1CommissionGross":     f1CommissionGross,
		"systemCommissionGross": systemCommissionGross,
	})
}
func publicLeaderboardHandler(c *gin.Context) {
	kind := strings.ToLower(c.DefaultQuery("kind", "f1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "100"))
	if limit <= 0 || limit > 1000 {
		limit = 100
	}

	// đổi TÊN CỘT ở đây nếu schema khác:
	benefCol := "ct.beneficiary_id" // ví dụ: "ct.beneficiary_id" nếu bạn đặt khác
	amountCol := "ct.amount"
	depthCol := "ct.depth"

	depthCond := fmt.Sprintf("%s = 1", depthCol)
	if kind != "f1" {
		depthCond = fmt.Sprintf("%s BETWEEN 1 AND 9", depthCol)
	}

	type aggRow struct {
		UserID   uint
		Username string
		Score    int64
	}
	var agg []aggRow

	// Tổng hoa hồng theo user
	err := DB.Table("commission_txns AS ct").
		Select(fmt.Sprintf("%s AS user_id, u.username, SUM(%s) AS score", benefCol, amountCol)).
		Joins("JOIN users u ON u.id = "+benefCol).
		Where("u.role = ?", "user").
		Where(depthCond).
		Group("user_id, u.username").
		Order("score DESC").
		Limit(limit).
		Scan(&agg).Error

	if err != nil {
		// LOG lỗi thật để biết vì sao 500 (sai cột/bảng, v.v.)
		log.Println("leaderboard query error:", err)
		// Tránh vỡ FE: trả rỗng
		c.JSON(200, gin.H{"rows": []LeaderboardRow{}})
		return
	}

	rows := make([]LeaderboardRow, 0, len(agg))
	for i, r := range agg {
		rows = append(rows, LeaderboardRow{
			Rank: i + 1, Username: r.Username, Score: r.Score,
		})
	}
	c.JSON(200, gin.H{"rows": rows})
}
func privateMyLeaderboardHandler(c *gin.Context) {
	uid := uint(c.MustGet("claims").(jwt.MapClaims)["sub"].(float64))
	kind := strings.ToLower(c.DefaultQuery("kind", "f1"))

	// lấy username (không lỗi cũng tiếp tục)
	var uname string
	_ = DB.Table("users").Select("username").Where("id = ?", uid).Scan(&uname).Error

	// ĐỔI tên cột nếu schema bạn khác
	benefCol := "ct.beneficiary_id"
	amountCol := "ct.amount"
	depthCol := "ct.depth"

	depthCond := fmt.Sprintf("%s = 1", depthCol) // F1
	if kind != "f1" {
		depthCond = fmt.Sprintf("%s BETWEEN 1 AND 9", depthCol) // Hệ thống
	}

	// Tổng điểm của chính mình (không phụ thuộc role, vì người dùng hiện tại là user)
	var my struct{ Score int64 }
	_ = DB.Table("commission_txns AS ct").
		Where(benefCol+" = ?", uid).
		Where(depthCond).
		Select("COALESCE(SUM(" + amountCol + "),0) AS score").
		Scan(&my).Error

	// Subquery tổng điểm MỖI USER THƯỜNG (exclude admin)
	sub := DB.Table("commission_txns AS ct").
		Joins("JOIN users u ON u.id = "+benefCol).
		Where("u.role = ?", "user").
		Where(depthCond).
		Select(benefCol + " AS user_id, SUM(" + amountCol + ") AS score").
		Group("user_id")

	// Rank = 1 + số người (user thường) có điểm > mình
	var higher int64
	if err := DB.Table("(?) AS t", sub).
		Where("t.score > ?", my.Score).
		Count(&higher).Error; err != nil {
		log.Println("my leaderboard rank error:", err)
		c.JSON(200, MyRankResp{Rank: 0, Username: uname, Score: my.Score})
		return
	}

	c.JSON(200, MyRankResp{
		Rank:     int(higher) + 1, // FE hiển thị # nếu > 100
		Username: uname,
		Score:    my.Score,
	})
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

func redeemBonusCodeHandler(c *gin.Context) {
	uid := uint(c.MustGet("claims").(jwt.MapClaims)["sub"].(float64))

	var body struct {
		Code string `json:"code"`
	}
	if err := c.BindJSON(&body); err != nil {
		c.JSON(400, gin.H{"error": "Thiếu mã code"})
		return
	}
	code := strings.ToUpper(strings.TrimSpace(body.Code)) // 👈 quan trọng

	var p PromoBonusCode
	if err := DB.Where("code = ? AND is_active = ?", code, true).First(&p).Error; err != nil {
		c.JSON(400, gin.H{"error": "Code không tồn tại hoặc đã bị vô hiệu"})
		return
	}
	if p.ExpiresAt != nil && time.Now().UTC().After(*p.ExpiresAt) {
		c.JSON(400, gin.H{"error": "Code đã hết hạn"})
		return
	}
	if p.UsedCount >= p.MaxUses {
		c.JSON(400, gin.H{"error": "Code đã được sử dụng"})
		return
	}

	err := DB.Transaction(func(tx *gorm.DB) error {
		// tăng used_count (optimistic)
		res := tx.Model(&PromoBonusCode{}).
			Where("id = ? AND used_count = ?", p.ID, p.UsedCount).
			UpdateColumn("used_count", gorm.Expr("used_count + 1"))
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected == 0 {
			return errors.New("conflict")
		}

		// cộng bonus_coins
		return tx.Model(&User{}).
			Where("id = ?", uid).
			UpdateColumn("bonus_coins", gorm.Expr("COALESCE(bonus_coins,0)+?", p.BonusCoins)).Error
	})
	if err != nil {
		c.JSON(400, gin.H{"error": "Không thể sử dụng code"})
		return
	}

	c.JSON(200, gin.H{
		"message":    "Nhận coin bonus thành công",
		"bonusCoins": p.BonusCoins,
	})
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
func isInSubtree(ownerID, targetID uint) (bool, error) {
	if ownerID == targetID {
		return true, nil
	}
	ids := []uint{ownerID}
	for d := 1; d <= 9; d++ {
		var us []User
		if err := DB.Where("referred_by IN ?", ids).Select("id").Find(&us).Error; err != nil {
			return false, err
		}
		next := make([]uint, 0, len(us))
		for _, u := range us {
			if u.ID == targetID {
				return true, nil
			}
			next = append(next, u.ID)
		}
		ids = next
		if len(ids) == 0 {
			break
		}
	}
	return false, nil
}
func downlineDashboardHandler(c *gin.Context) {
	ownerID := uint(c.MustGet("claims").(jwt.MapClaims)["sub"].(float64))
	targetID64, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	targetID := uint(targetID64)

	// bắt buộc nằm trong F1..F9 của owner
	ok, err := isInSubtree(ownerID, targetID)
	if err != nil {
		c.JSON(500, gin.H{"error": "Lỗi kiểm tra quyền xem"})
		return
	}
	if !ok {
		c.JSON(403, gin.H{"error": "Không có quyền xem người này"})
		return
	}

	// tháng: YYYY-MM, mặc định tháng hiện tại
	month := strings.TrimSpace(c.Query("month"))
	now := time.Now()
	if month == "" {
		month = fmt.Sprintf("%04d-%02d", now.Year(), int(now.Month()))
	}
	// range time của tháng
	firstDay, _ := time.Parse("2006-01", month)
	nextMonth := firstDay.AddDate(0, 1, 0)

	// lấy user
	var u User
	if err := DB.Select("id, username, coins, bonus_coins, v_ip_level").
		First(&u, targetID).Error; err != nil {
		c.JSON(404, gin.H{"error": "User không tồn tại"})
		return
	}

	// KPI
	ov := DashboardOverview{}

	// tổng tài sản = coins + bonusCoins (nếu không có bonusCoins thì để 0)
	type tmp struct{ Coins, BonusCoins int64 }
	var t tmp
	_ = DB.Model(&User{}).Where("id=?", targetID).
		Select("coins, IFNULL(bonus_coins,0) as bonus_coins").Scan(&t).Error
	ov.TotalAssets = t.Coins + t.BonusCoins

	// F1 count (trực tiếp)
	DB.Model(&User{}).Where("referred_by = ?", targetID).Count(&ov.F1Count)

	// 👇 SystemCount: tất cả tuyến dưới depth 1..9 (dùng closure table 'downlines')
	// Đổi tên bảng/cột nếu schema của bạn khác.
	_ = DB.Table("downlines AS dl").
		Where("dl.ancestor_id = ? AND dl.depth BETWEEN 1 AND 9", targetID).
		Count(&ov.SystemCount).Error

	// Hoa hồng F1 (depth=1) & hệ thống (1..9) - tổng toàn thời gian
	DB.Model(&CommissionTxn{}).
		Where("beneficiary_id = ? AND depth = 1", targetID).
		Select("COALESCE(SUM(amount),0)").Scan(&ov.F1CommissionTotal)

	DB.Model(&CommissionTxn{}).
		Where("beneficiary_id = ? AND depth BETWEEN 1 AND 9", targetID).
		Select("COALESCE(SUM(amount),0)").Scan(&ov.SystemCommissionTotal)

	// Thu nhập theo ngày trong tháng (group by day)
	type row struct {
		D int
		S int64
	}
	var rs []row
	DB.Model(&CommissionTxn{}).
		Where("beneficiary_id = ? AND created_at >= ? AND created_at < ?",
			targetID, firstDay, nextMonth).
		Select("DAY(created_at) AS d, COALESCE(SUM(amount),0) AS s").
		Group("DAY(created_at)").Order("d").
		Scan(&rs)

	// build 1..last day
	lastDay := time.Date(firstDay.Year(), firstDay.Month()+1, 0, 0, 0, 0, 0, time.Local).Day()
	earn := make([]DayEarning, lastDay)
	for i := 1; i <= lastDay; i++ {
		earn[i-1] = DayEarning{Day: i, Amount: 0}
	}
	for _, r := range rs {
		if r.D >= 1 && r.D <= lastDay {
			earn[r.D-1].Amount = r.S
		}
	}

	resp := DownlineDashboardResp{
		UserID:   u.ID,
		Username: u.Username,
		Month:    month,
		Overview: ov,
		Earnings: earn,
	}
	c.JSON(200, resp)
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
			 COALESCE(t.note, '') AS note,  
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

// Trả true nếu viewerID là tổ tiên (F1..F9) của targetID
func isAncestorWithin(tx *gorm.DB, viewerID, targetID uint, maxDepth int) (bool, error) {
	cur := targetID
	for d := 0; d < maxDepth; d++ {
		var ref *uint
		if err := tx.Model(&User{}).
			Select("referred_by").
			Where("id = ?", cur).
			Take(&ref).Error; err != nil {
			return false, err
		}
		if ref == nil || *ref == 0 {
			return false, nil
		}
		if *ref == viewerID {
			return true, nil
		}
		cur = *ref
	}
	return false, nil
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
			"kyc_status":     "VERIFIED",
			"kyc_full_name":  fullName,
			"kyc_dob":        dob,
			"kyc_number":     number,
			"kyc_front_path": fFront,
			"kyc_back_path":  fBack,
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
func init() {
	mathrand.Seed(time.Now().UnixNano())
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
func collapseInventoryDuplicates() {
	type Row struct {
		UserID uint
		Code   string
		Total  int64
		Cnt    int64
	}
	var dups []Row
	DB.Model(&InventoryItem{}).
		Select("user_id, code, SUM(qty) AS total, COUNT(*) AS cnt").
		Group("user_id, code").
		Having("COUNT(*) > 1").
		Scan(&dups)

	for _, r := range dups {
		_ = DB.Transaction(func(tx *gorm.DB) error {
			// xoá hết bản ghi trùng
			if err := tx.Where("user_id=? AND code=?", r.UserID, r.Code).Delete(&InventoryItem{}).Error; err != nil {
				return err
			}
			// tạo lại 1 dòng duy nhất với tổng Qty
			return tx.Create(&InventoryItem{UserID: r.UserID, Code: r.Code, Qty: r.Total}).Error
		})
	}
}

/* ===== AUTH & PROFILE ===== */
func registerHandler(c *gin.Context) {
	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
		Ref      string `json:"ref"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	username := strings.ToLower(strings.TrimSpace(req.Username))
	if username == "" {
		c.JSON(400, gin.H{"error": "Thiếu username"})
		return
	}
	if req.Password == "" {
		c.JSON(400, gin.H{"error": "Thiếu mật khẩu"})
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

		// chỉ lưu username + pass, còn name / phone để trống
		u := User{
			Username:     username,
			Name:         "",
			Phone:        "",
			PasswordHash: string(hash),
			Role:         "user",
			Coins:        0,
			/*	BonusCoins:   100, // tặng coin bonus*/
			TotalTopup:   0,
			VIPLevel:     0,
			ReferralCode: &code,
			ReferredBy:   referredBy,
			/*FreeSpins:    10, // tặng lượt quay */
		}
		if err := tx.Create(&u).Error; err != nil {
			return fmt.Errorf("create user: %w", err)
		}

		/*// Gửi thông báo chào mừng
		_ = tx.Create(&Notification{
			UserID: u.ID,
			Title:  "Chào mừng!",
			Body:   "Bạn đã nhận được 100 coin bonus và 10 lượt quay miễn phí khi đăng ký.",
		}).Error*/

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
	// Khóa hàng tồn
	var it InventoryItem
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("user_id = ? AND code = ?", userID, code).
		First(&it).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return fmt.Errorf("vật phẩm %s không đủ", code)
		}
		return err
	}
	if it.Qty < qty {
		return fmt.Errorf("vật phẩm %s không đủ", code)
	}
	// Trừ có điều kiện để tránh âm do race
	return tx.Model(&InventoryItem{}).
		Where("user_id = ? AND code = ? AND qty >= ?", userID, code, qty).
		Update("qty", gorm.Expr("qty - ?", qty)).Error
}

func addToInventory(tx *gorm.DB, userID uint, code string, qty int64) error {
	if qty == 0 {
		return nil
	}
	return tx.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "user_id"}, {Name: "code"}},
		DoUpdates: clause.Assignments(map[string]interface{}{
			"qty":        gorm.Expr("qty + ?", qty),
			"updated_at": time.Now(),
		}),
	}).Create(&InventoryItem{
		UserID: userID,
		Code:   code,
		Qty:    qty,
	}).Error
}

// trả về: result:"COIN"|"DRAGON_BALL", code, amount, coins, inv(map)
func chestOpenHandler(c *gin.Context) {
	uid := uint(c.MustGet("claims").(jwt.MapClaims)["sub"].(float64))

	var user User
	if err := DB.First(&user, uid).Error; err != nil {
		c.JSON(404, gin.H{"error": "User không tồn tại"})
		return
	}

	type result struct {
		Result               string         `json:"result"` // "DRAGON_BALL" | "EVENT_CARD"
		Code                 *string        `json:"code,omitempty"`
		Amount               int64          `json:"amount"` // số lượng item nhận được
		Coins                int64          `json:"coins"`
		BonusCoins           int64          `json:"bonusCoins"`
		FreeSpins            int            `json:"freeSpins"`
		Inv                  map[string]int `json:"inv"`
		UsedFreeSpin         bool           `json:"used_free_spin,omitempty"`
		RemainingFreeSpins   int            `json:"remaining_free_spins,omitempty"`
		MilestoneRewarded    bool           `json:"milestoneRewarded,omitempty"`
		MilestoneRewardCoins int64          `json:"milestoneRewardCoins,omitempty"`
		ChestOpens           int64          `json:"chest_opens"`
		RemainingUntilBonus  int64          `json:"remaining_until_bonus"`
	}

	var out result
	out.Inv = map[string]int{}

	const spinCost int64 = 50        // phí mở nếu không có freeSpin
	const milestoneEvery int64 = 100 // mốc thưởng theo lượt mở
	const milestoneReward int64 = 1000

	if err := DB.Transaction(func(tx *gorm.DB) error {
		// Khóa hàng user
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&user, uid).Error; err != nil {
			return err
		}

		// ✅ CHẶT CHẼ: nếu không có freeSpins và tổng (bonus+coins) < spinCost → chặn luôn
		if user.FreeSpins <= 0 && user.BonusCoins+user.Coins < spinCost {
			return fmt.Errorf("Số dư không đủ")
		}

		// 1) Trừ freeSpins hoặc bonus/coins
		usedFree := false
		if user.FreeSpins > 0 {
			if err := tx.Model(&User{}).
				Where("id = ?", user.ID).
				Update("free_spins", gorm.Expr("free_spins - 1")).Error; err != nil {
				return err
			}
			user.FreeSpins -= 1
			usedFree = true
		} else {
			// dùng bonus trước, còn thiếu trừ coins
			if err := spendForSystem(tx, &user, spinCost); err != nil {
				return err
			}
		}
		out.UsedFreeSpin = usedFree

		// 2) Random phần thưởng: 10% DB1..DB7, 90% thẻ sự kiện EV (1..5)
		rewardKind := "EVENT_CARD"
		var rewardCode string
		var rewardAmt int64

		r := mathrand.Intn(100) // 0..99
		if r < 10 {
			rewardKind = "DRAGON_BALL"
			rewardCode = fmt.Sprintf("DB%d", 1+mathrand.Intn(7))
			rewardAmt = 1
		} else {
			rewardKind = "EVENT_CARD"
			rewardCode = "EV"
			rewardAmt = int64(1 + mathrand.Intn(5)) // 1..5
		}

		// Cộng item vào túi
		if err := addToInventory(tx, user.ID, rewardCode, rewardAmt); err != nil {
			return err
		}
		out.Inv[rewardCode] += int(rewardAmt)

		// 3) Log giao dịch mở rương
		cost := int64(0)
		if !usedFree {
			cost = spinCost
		}
		if err := tx.Create(&ChestTxn{
			UserID:       user.ID,
			Cost:         cost,
			RewardKind:   rewardKind,
			RewardCode:   rewardCode,
			RewardAmount: rewardAmt,
		}).Error; err != nil {
			return err
		}

		// 4) Tăng bộ đếm mở rương
		if err := tx.Model(&User{}).
			Where("id = ?", user.ID).
			Update("chest_open_count", gorm.Expr("chest_open_count + 1")).Error; err != nil {
			return fmt.Errorf("increase chest_open_count: %w", err)
		}

		// 5) Đọc lại coins/bonus/freeSpins/count
		if err := tx.Select("coins, bonus_coins, free_spins, chest_open_count").
			First(&user, user.ID).Error; err != nil {
			return err
		}

		// 6) Thưởng mốc 100/200/...
		count := int64(user.ChestOpenCount)
		if count%milestoneEvery == 0 {
			// (giữ nguyên cộng vào coins; nếu muốn cộng bonus, đổi tên cột ở đây)
			if err := tx.Model(&User{}).
				Where("id = ?", user.ID).
				Update("coins", gorm.Expr("coins + ?", milestoneReward)).Error; err != nil {
				return fmt.Errorf("milestone add coins: %w", err)
			}
			out.MilestoneRewarded = true
			out.MilestoneRewardCoins = milestoneReward

			_ = tx.Create(&Notification{
				UserID: user.ID,
				Title:  "Chúc mừng đạt mốc mở rương!",
				Body:   fmt.Sprintf("Bạn đạt %d lượt mở rương và nhận %d coin thưởng.", user.ChestOpenCount, milestoneReward),
			}).Error

			// reload coins
			if err := tx.Select("coins").First(&user, user.ID).Error; err != nil {
				return err
			}
		}

		// 7) Còn bao nhiêu lượt tới mốc
		remaining := milestoneEvery - (int64(user.ChestOpenCount) % milestoneEvery)
		if remaining == 0 {
			remaining = milestoneEvery
		}

		// 8) Gán output
		out.Result = rewardKind
		if rewardKind == "DRAGON_BALL" {
			out.Code = &rewardCode
		}
		out.Amount = rewardAmt
		out.Coins = user.Coins
		out.BonusCoins = user.BonusCoins
		out.FreeSpins = user.FreeSpins
		out.RemainingFreeSpins = user.FreeSpins
		out.ChestOpens = int64(user.ChestOpenCount)
		out.RemainingUntilBonus = remaining

		return nil
	}); err != nil {
		msg := err.Error()
		if strings.Contains(msg, "Số dư không đủ") {
			c.JSON(400, gin.H{"error": msg})
			return
		}
		c.JSON(500, gin.H{"error": "Mở rương thất bại"})
		return
	}

	c.JSON(200, out)
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

	// Request body
	var req struct {
		Code         string `json:"code"`
		Qty          int64  `json:"qty"`
		PricePerUnit int64  `json:"pricePerUnit"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": "Dữ liệu không hợp lệ"})
		return
	}
	req.Code = strings.ToUpper(strings.TrimSpace(req.Code))
	if req.Code == "" || req.Qty <= 0 || req.PricePerUnit <= 0 {
		c.JSON(400, gin.H{"error": "Dữ liệu không hợp lệ"})
		return
	}

	// ✅ Chỉ cho phép DB1..DB7 hoặc EV
	isDB := strings.HasPrefix(req.Code, "DB") && len(req.Code) == 3 && req.Code[2] >= '1' && req.Code[2] <= '7'
	isEV := req.Code == "EV"
	if !(isDB || isEV) {
		c.JSON(400, gin.H{"error": "Mã vật phẩm không hợp lệ (chỉ DB1..DB7 hoặc EV)"})
		return
	}

	if err := DB.Transaction(func(tx *gorm.DB) error {
		// 🔒 Khoá row inventory của user+code để chống race
		var inv InventoryItem
		if err := tx.
			Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("user_id = ? AND code = ?", uid, req.Code).
			First(&inv).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return fmt.Errorf("vật phẩm %s không đủ (còn 0)", req.Code)
			}
			return err
		}

		if inv.Qty < req.Qty {
			return fmt.Errorf("vật phẩm %s không đủ (còn %d)", req.Code, inv.Qty)
		}

		// ✅ Trừ tồn kho an toàn (điều kiện qty >= req.Qty ngay trong SQL)
		res := tx.Model(&InventoryItem{}).
			Where("user_id = ? AND code = ? AND qty >= ?", uid, req.Code, req.Qty).
			Update("qty", gorm.Expr("qty - ?", req.Qty))
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected == 0 {
			// Ai đó vừa trừ mất trong race khác
			return fmt.Errorf("vật phẩm %s không đủ", req.Code)
		}

		// Tạo listing
		ml := MarketListing{
			SellerID:     uid,
			Code:         req.Code,
			Qty:          req.Qty,
			PricePerUnit: req.PricePerUnit,
			IsActive:     true,
		}
		if err := tx.Create(&ml).Error; err != nil {
			return err
		}
		return nil
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

// Trừ amount cho các tác vụ hệ thống: ưu tiên BonusCoins rồi mới Coins
func spendForSystem(tx *gorm.DB, u *User, amount int64) error {
	if amount <= 0 {
		return nil
	}
	if u.BonusCoins+u.Coins < amount {
		return fmt.Errorf("Số dư không đủ")
	}
	useBonus := u.BonusCoins
	if useBonus > amount {
		useBonus = amount
	}
	left := amount - useBonus

	if err := tx.Model(&User{}).Where("id = ?", u.ID).Updates(map[string]any{
		"bonus_coins": gorm.Expr("bonus_coins - ?", useBonus),
		"coins":       gorm.Expr("coins - ?", left),
	}).Error; err != nil {
		return err
	}
	u.BonusCoins -= useBonus
	u.Coins -= left
	return nil
}

// gen 12 ký tự A-Za-z0-9
func genPromoCode(n int) string {
	const letters = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, n)
	for i := range b {
		k, _ := rand.Int(rand.Reader, big.NewInt(int64(len(letters))))
		b[i] = letters[k.Int64()]
	}
	return string(b)
}
func cleanupExpiredPromoCodes() {
	now := time.Now()

	// 1) set inactive nếu hết hạn hoặc dùng hết
	_ = DB.Model(&PromoCode{}).
		Where("(expires_at IS NOT NULL AND expires_at <= ?) OR (max_uses IS NOT NULL AND used_count >= max_uses)", now).
		Update("is_active", false).Error

	// 2) xoá cứng các code inactive để danh sách gọn (nếu muốn chỉ ẩn, bỏ phần Delete này)
	_ = DB.Where("is_active = 0").Delete(&PromoCode{}).Error
}

type createCodeReq struct {
	FreeSpins int `json:"freeSpins"` // mặc định 10
}
type createCodeRes struct {
	Code      string `json:"code"`
	FreeSpins int    `json:"freeSpins"`
}

func myNotificationsHandler(c *gin.Context) {
	uid := uint(c.MustGet("claims").(jwt.MapClaims)["sub"].(float64))
	unreadOnly := strings.TrimSpace(c.Query("unreadOnly")) == "1"

	type Row struct {
		ID        uint      `json:"id"`
		Title     string    `json:"title"`
		Body      string    `json:"body"`
		IsRead    bool      `json:"isRead"`
		CreatedAt time.Time `json:"createdAt"`
	}

	q := DB.Model(&Notification{}).Where("user_id = ?", uid)
	if unreadOnly {
		q = q.Where("is_read = 0")
	}

	var rows []Row
	q.Order("id DESC").Scan(&rows)

	var unread int64
	DB.Model(&Notification{}).Where("user_id = ? AND is_read = 0", uid).Count(&unread)

	c.JSON(200, gin.H{"rows": rows, "unread": unread})
}

func markMyNotificationsReadHandler(c *gin.Context) {
	uid := uint(c.MustGet("claims").(jwt.MapClaims)["sub"].(float64))
	if err := DB.Model(&Notification{}).
		Where("user_id = ? AND is_read = 0", uid).
		Update("is_read", true).Error; err != nil {
		c.JSON(500, gin.H{"error": "Cập nhật thất bại"})
		return
	}
	c.JSON(200, gin.H{"message": "Đã đánh dấu đã đọc"})
}
func genPromoString(n int) string {
	const letters = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, n)
	for i := range b {
		k, _ := rand.Int(rand.Reader, big.NewInt(int64(len(letters))))
		b[i] = letters[k.Int64()]
	}
	return string(b)
}

type CreatePromoReq struct {
	RewardFreeSpin int  `json:"rewardFreeSpin"`          // mặc định 10
	DurationHours  *int `json:"durationHours,omitempty"` // chỉ áp dụng với UNLIMITED (nil => 24h)
	MaxUses        *int `json:"maxUses,omitempty"`       // nil => vô hạn; =1 => one-use
	Count          int  `json:"count"`                   // số code muốn tạo (mặc định 1). Với one-use có thể 10/50/100/500
}

// POST /admin/promo-codes
func adminCreatePromoCodeHandler(c *gin.Context) {
	// Yêu cầu adminRequired() đã gắn ở router
	claims := c.MustGet("claims").(jwt.MapClaims)
	adminID := uint(claims["sub"].(float64))

	var req CreatePromoReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": "Dữ liệu không hợp lệ"})
		return
	}

	// Mặc định
	if req.RewardFreeSpin <= 0 {
		req.RewardFreeSpin = 10
	}
	// Count: mặc định 1, chặn quá lớn để an toàn
	if req.Count <= 0 {
		req.Count = 1
	}
	if req.Count > 1000 {
		req.Count = 1000
	}

	// Phân loại theo MaxUses & DurationHours:
	// - ONE-USE vô hạn: MaxUses=1, ExpiresAt = NULL
	// - UNLIMITED có hạn: MaxUses=nil, ExpiresAt = now + duration (mặc định 24h)
	var expiresAt *time.Time
	if req.MaxUses == nil {
		// unlimited uses
		dh := 24
		if req.DurationHours != nil && *req.DurationHours > 0 {
			dh = *req.DurationHours
		}
		t := time.Now().UTC().Add(time.Duration(dh) * time.Hour)
		expiresAt = &t
	} else {
		// có MaxUses: nếu =1 và muốn vô thời hạn -> expiresAt = nil
		if *req.MaxUses < 1 {
			c.JSON(400, gin.H{"error": "MaxUses phải >= 1 hoặc bỏ trống để vô hạn"})
			return
		}
		expiresAt = nil
	}

	type Out struct {
		Codes []string `json:"codes"`
	}
	out := Out{Codes: make([]string, 0, req.Count)}

	err := DB.Transaction(func(tx *gorm.DB) error {
		for i := 0; i < req.Count; i++ {
			code, err := randCode(12)
			if err != nil {
				return fmt.Errorf("gen code: %w", err)
			}
			pc := PromoCode{
				Code:           code,
				RewardFreeSpin: req.RewardFreeSpin,
				MaxUses:        req.MaxUses, // nil => vô hạn; 1 => one-use
				UsedCount:      0,
				ExpiresAt:      expiresAt, // nil => vô thời hạn
				IsActive:       true,
				CreatedBy:      &adminID,
			}
			if err := tx.Create(&pc).Error; err != nil {
				// tránh đụng unique, nếu trùng thì thử lại một ít lần
				if strings.Contains(strings.ToLower(err.Error()), "duplicate") {
					i--
					continue
				}
				return err
			}
			out.Codes = append(out.Codes, pc.Code)
		}
		return nil
	})
	if err != nil {
		c.JSON(500, gin.H{"error": "Không tạo được code: " + err.Error()})
		return
	}

	c.JSON(200, gin.H{
		"message":        "Đã tạo gift code",
		"rewardFreeSpin": req.RewardFreeSpin,
		"maxUses":        req.MaxUses, // nil hoặc 1
		"expiresAt":      expiresAt,   // null nếu vô hạn
		"count":          len(out.Codes),
		"codes":          out.Codes, // trả về danh sách code đã tạo
	})
}

// POST /admin/promo-bonus-codes
func adminCreateBonusCodesHandler(c *gin.Context) {
	adminID := uint(c.MustGet("claims").(jwt.MapClaims)["sub"].(float64))

	var body struct {
		Count         int  `json:"count"`         // số code muốn tạo (mặc định 1)
		BonusCoins    *int `json:"bonusCoins"`    // mặc định 10
		DurationHours *int `json:"durationHours"` // nếu có: hết hạn sau N giờ
	}
	_ = c.BindJSON(&body)

	count := body.Count
	if count <= 0 || count > 500 {
		count = 1
	}

	coins := 10
	if body.BonusCoins != nil && *body.BonusCoins > 0 {
		coins = *body.BonusCoins
	}

	var expiresAt *time.Time
	if body.DurationHours != nil && *body.DurationHours > 0 {
		t := time.Now().UTC().Add(time.Duration(*body.DurationHours) * time.Hour)
		expiresAt = &t
	}

	codes := make([]string, 0, count)
	for i := 0; i < count; i++ {
		// sinh code + đảm bảo unique (thử tối đa 8 lần để tránh vòng lặp vô hạn)
		var code string
		for try := 0; try < 8; try++ {
			val, err := randCode(10) // 👈 dùng helper của bạn: (string, error)
			if err != nil {
				c.JSON(500, gin.H{"error": "Không tạo được mã (rng error)"})
				return
			}
			code = strings.ToUpper(val) // optional: chuẩn hoá về chữ hoa

			var exists int64
			if err := DB.Model(&PromoBonusCode{}).Where("code = ?", code).Count(&exists).Error; err != nil {
				c.JSON(500, gin.H{"error": "Lỗi kiểm tra trùng code"})
				return
			}
			if exists == 0 {
				break
			}
			if try == 7 {
				c.JSON(500, gin.H{"error": "Không thể tạo code unique, thử lại sau"})
				return
			}
		}

		p := PromoBonusCode{
			Code:       code,
			BonusCoins: coins,
			MaxUses:    1, // one-shot
			UsedCount:  0,
			ExpiresAt:  expiresAt,
			IsActive:   true,
			CreatedBy:  &adminID,
		}
		if err := DB.Create(&p).Error; err != nil {
			c.JSON(500, gin.H{"error": "Tạo code thất bại"})
			return
		}
		codes = append(codes, code)
	}

	c.JSON(200, gin.H{
		"message":   "Đã tạo code bonus",
		"codes":     codes,
		"bonus":     coins,
		"count":     count,
		"expiresAt": expiresAt,
	})
}

// GET /admin/promo-codes
func adminListActivePromoCodesHandler(c *gin.Context) {
	nowUTC := time.Now().UTC()

	// Map đúng shape FE đang dùng
	type Row struct {
		ID        uint       `json:"id"`
		Code      string     `json:"code"`
		Value     int        `json:"value"` // = RewardFreeSpin
		ExpiresAt *time.Time `json:"expiresAt"`
		CreatedAt time.Time  `json:"createdAt"`
	}

	var rows []Row

	// Lọc:
	// - còn hoạt động
	// - chưa hết hạn (hoặc không có hạn)
	// - chưa dùng hết lượt (MaxUses NULL => vô hạn)
	err := DB.
		Table("promo_codes").
		Select(`
			id,
			code,
			reward_free_spin AS value,
			expires_at,
			created_at
		`).
		Where("is_active = ?", true).
		Where("(expires_at IS NULL OR expires_at > ?)", nowUTC).
		Where("(max_uses IS NULL OR used_count < max_uses)").
		Order("expires_at ASC, created_at DESC").
		Limit(100).
		Scan(&rows).Error

	if err != nil {
		c.JSON(500, gin.H{"error": "Không lấy được danh sách code"})
		return
	}

	c.JSON(200, gin.H{"rows": rows})
}

func redeemCodeHandler(c *gin.Context) {
	uid := uint(c.MustGet("claims").(jwt.MapClaims)["sub"].(float64))
	var req RedeemReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	var outFreeSpins int

	if err := DB.Transaction(func(tx *gorm.DB) error {
		// lock code
		var pc PromoCode
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("code = ? AND is_active = 1", req.Code).
			First(&pc).Error; err != nil {
			return fmt.Errorf("Code không tồn tại hoặc đã bị vô hiệu")
		}

		// còn hạn?
		if pc.ExpiresAt != nil && time.Now().After(*pc.ExpiresAt) {
			return fmt.Errorf("Code đã hết hạn")
		}

		// còn lượt dùng? (nil => vô hạn)
		if pc.MaxUses != nil && pc.UsedCount >= *pc.MaxUses {
			return fmt.Errorf("Code đã dùng hết")
		}

		// chặn user dùng lặp
		// unique (promo_code_id, user_id)
		use := PromoCodeUse{PromoCodeID: pc.ID, UserID: uid}
		if err := tx.Create(&use).Error; err != nil {
			// duplicate
			if strings.Contains(strings.ToLower(err.Error()), "unique") {
				return fmt.Errorf("Bạn đã nhập code này rồi")
			}
			return err
		}

		// cộng free spins cho user
		if err := tx.Model(&User{}).
			Where("id = ?", uid).
			Update("free_spins", gorm.Expr("free_spins + ?", pc.RewardFreeSpin)).Error; err != nil {
			return err
		}

		// tăng UsedCount
		if err := tx.Model(&pc).Update("used_count", gorm.Expr("used_count + 1")).Error; err != nil {
			return err
		}

		// đọc lại để trả về số free_spins hiện tại
		var u User
		if err := tx.Select("free_spins").First(&u, uid).Error; err != nil {
			return err
		}
		outFreeSpins = int(u.FreeSpins)

		// tạo notification cho user
		note := Notification{
			UserID: uid,
			Title:  "Nhận quà gift code",
			Body:   fmt.Sprintf("Bạn được +%d lượt quay miễn phí (code %s).", pc.RewardFreeSpin, pc.Code),
		}
		_ = tx.Create(&note).Error

		return nil
	}); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	c.JSON(200, gin.H{
		"message":   "Nhập code thành công",
		"freeSpins": outFreeSpins,
	})
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
		if buyer.Coins+buyer.BonusCoins < total {
			return fmt.Errorf("Số dư không đủ")
		}
		// trừ tiền hệ thống ở phía người mua (ưu tiên bonus)
		if err := spendForSystem(tx, &buyer, total); err != nil {
			return err
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
		// Trả 401 để FE tự logout
		c.JSON(401, gin.H{"error": "UNAUTHORIZED"})
		return
	}

	// ĐỂ NGUYÊN là hằng số “untyped”, tránh mismatch % giữa int và int64
	const milestoneEvery = 100

	// Go sẽ tự dùng cùng kiểu với ChestOpenCount cho phép toán %
	rem := milestoneEvery - (user.ChestOpenCount % milestoneEvery)
	if rem == 0 {
		rem = milestoneEvery
	}
	totalCoins := user.Coins + user.BonusCoins
	c.JSON(200, gin.H{
		"coins":               user.Coins,
		"totalTopup":          user.TotalTopup,
		"vipLevel":            user.VIPLevel,
		"freeSpins":           user.FreeSpins,
		"chestOpens":          user.ChestOpenCount, // giữ nguyên kiểu của field
		"remainingUntilBonus": rem,                 // cùng kiểu với rem ở trên
		"bonusCoins":          user.BonusCoins,
		"totalCoins":          totalCoins,
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
		// 1) clear tham chiếu/upline để tránh FK
		if err := tx.Model(&User{}).Where("referred_by = ?", uid).Update("referred_by", nil).Error; err != nil {
			return fmt.Errorf("clear referred_by: %w", err)
		}

		// 2) dọn dữ liệu liên quan tới user ở mọi bảng
		// lịch sử giao dịch coin/chuyển
		if err := tx.Where("from_id = ? OR to_id = ?", uid, uid).Delete(&TransferTxn{}).Error; err != nil {
			return fmt.Errorf("del transfer_txns: %w", err)
		}
		if err := tx.Where("user_id = ? OR admin_id = ?", uid, uid).Delete(&CoinTxn{}).Error; err != nil {
			return fmt.Errorf("del coin_txns: %w", err)
		}
		// mua VIP, hoa hồng
		if err := tx.Where("user_id = ?", uid).Delete(&VipPurchaseTxn{}).Error; err != nil {
			return fmt.Errorf("del vip_purchase_txns: %w", err)
		}
		if err := tx.Where("buyer_id = ? OR beneficiary_id = ?", uid, uid).Delete(&CommissionTxn{}).Error; err != nil {
			return fmt.Errorf("del commission_txns: %w", err)
		}
		// rút coin
		if err := tx.Where("user_id = ? OR admin_id = ?", uid, uid).Delete(&WithdrawTxn{}).Error; err != nil {
			return fmt.Errorf("del withdraw_txns: %w", err)
		}
		// referral one-off
		if err := tx.Where("inviter_id = ? OR invitee_id = ?", uid, uid).Delete(&ReferralReward{}).Error; err != nil {
			return fmt.Errorf("del referral_rewards: %w", err)
		}
		// túi đồ, log mở rương, thông báo
		if err := tx.Where("user_id = ?", uid).Delete(&InventoryItem{}).Error; err != nil {
			return fmt.Errorf("del inventory_items: %w", err)
		}
		if err := tx.Where("user_id = ?", uid).Delete(&ChestTxn{}).Error; err != nil {
			return fmt.Errorf("del chest_txns: %w", err)
		}
		if err := tx.Where("user_id = ?", uid).Delete(&Notification{}).Error; err != nil {
			return fmt.Errorf("del notifications: %w", err)
		}
		// promo code uses (nếu có)
		if err := tx.Where("user_id = ?", uid).Delete(&PromoCodeUse{}).Error; err != nil {
			// bảng này có thể rỗng; vẫn nên trả lỗi rõ ràng nếu có
			return fmt.Errorf("del promo_code_uses: %w", err)
		}
		// chợ: rút & xoá listing còn lại
		// (chỉ cần delete; nếu muốn trả vật phẩm thì đã xoá inventory rồi)
		if err := tx.Where("seller_id = ?", uid).Delete(&MarketListing{}).Error; err != nil {
			return fmt.Errorf("del market_listings: %w", err)
		}

		// 3) xoá cứng user
		if err := tx.Unscoped().Delete(&User{}, uid).Error; err != nil {
			return fmt.Errorf("del user: %w", err)
		}
		return nil
	}); err != nil {
		log.Println("admin hard delete error:", err)
		c.JSON(500, gin.H{"error": "Xoá tài khoản thất bại"})
		return
	}

	// 4) (tuỳ chọn) xoá file KYC trên đĩa sau khi TX thành công
	if u.KYCFrontPath != "" {
		_ = os.Remove(filepath.Join(kycAbs, u.KYCFrontPath))
	}
	if u.KYCBackPath != "" {
		_ = os.Remove(filepath.Join(kycAbs, u.KYCBackPath))
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

	const vipInviteMilestoneReward int64 = 500 // thưởng mốc cho F1 khi đạt 10 direct VIP (chỉ 1 lần)

	if user.VIPLevel >= 1 {
		c.JSON(400, gin.H{"error": "Bạn đã là VIP"})
		return
	}
	if user.Coins < price {
		c.JSON(400, gin.H{"error": "Số dư không đủ"})
		return
	}

	if err := DB.Transaction(func(tx *gorm.DB) error {
		// 1) Trừ coin & set VIP = 1
		if err := tx.Model(&User{}).Where("id = ?", user.ID).
			Update("coins", gorm.Expr("coins - ?", price)).Error; err != nil {
			return err
		}
		old := user.VIPLevel
		if err := tx.Model(&User{}).Where("id = ?", user.ID).
			Update("v_ip_level", 1).Error; err != nil {
			return err
		}
		if err := tx.Create(&VipPurchaseTxn{
			UserID: user.ID, Level: 1, Price: price, OldLevel: old,
		}).Error; err != nil {
			return err
		}

		// 2) Tặng 10 lượt quay miễn phí cho NGƯỜI MUA (giữ nguyên)
		if err := tx.Model(&User{}).
			Where("id = ?", user.ID).
			Update("free_spins", gorm.Expr("free_spins + ?", 10)).Error; err != nil {
			return fmt.Errorf("award buyer free spins: %w", err)
		}

		// 3) Chia hoa hồng 9 tầng, mỗi tầng 10% — CHỈ trả cho upline đã VIP
		uplines, _ := getUplines(tx, user.ReferredBy, 9)
		allocated := 0
		for i, up := range uplines {
			if allocated >= 100 {
				break
			}
			if up.VIPLevel < 1 {
				continue
			}
			pct := 10
			if left := 100 - allocated; pct > left {
				pct = left
			}
			amt := (price * int64(pct)) / 100
			if amt > 0 {
				if err := tx.Model(&User{}).Where("id = ?", up.ID).
					Update("coins", gorm.Expr("coins + ?", amt)).Error; err != nil {
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

		// 4) Thưởng mốc cho F1: CHỈ 1 LẦN khi đạt 10 direct VIP
		if user.ReferredBy != nil && *user.ReferredBy != 0 {
			// lock hàng F1 để đọc/ghi cờ an toàn
			var f1 User
			if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
				Select("id, coins, invite10_vip_bonus_paid").
				First(&f1, *user.ReferredBy).Error; err == nil {

				// Đếm số F1 đã VIP (đã bao gồm user hiện tại vì ở trên set VIP xong)
				var directVipCount int64
				if err := tx.Model(&User{}).
					Where("referred_by = ? AND v_ip_level >= 1", f1.ID).
					Count(&directVipCount).Error; err == nil {

					// Nếu vừa đạt 10 và CHƯA trả thưởng trước đó -> thưởng & set cờ
					if directVipCount >= 10 && !f1.Invite10VipBonusPaid {
						if err := tx.Model(&User{}).
							Where("id = ?", f1.ID).
							Update("coins", gorm.Expr("coins + ?", vipInviteMilestoneReward)).Error; err != nil {
							return fmt.Errorf("award F1 milestone: %w", err)
						}
						if err := tx.Model(&User{}).
							Where("id = ?", f1.ID).
							Update("invite10_vip_bonus_paid", true).Error; err != nil {
							return fmt.Errorf("mark F1 milestone paid: %w", err)
						}
						_ = tx.Create(&Notification{
							UserID: f1.ID,
							Title:  "Thưởng mốc mời bạn VIP",
							Body:   fmt.Sprintf("Bạn đã có 10 người mua VIP trực tiếp. Thưởng +%d coin.", vipInviteMilestoneReward),
						}).Error
					}
				}
			}
		}

		// 5) Phần còn lại (nếu còn) -> log cho admin
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

// GET /private/notifications?unreadOnly=1
func listNotificationsHandler(c *gin.Context) {
	uid := uint(c.MustGet("claims").(jwt.MapClaims)["sub"].(float64))
	unreadOnly := c.Query("unreadOnly") == "1"

	q := DB.Where("user_id = ?", uid).Order("id DESC").Limit(50)
	if unreadOnly {
		q = q.Where("is_read = 0")
	}

	var rows []Notification
	if err := q.Find(&rows).Error; err != nil {
		c.JSON(500, gin.H{"error": "Không lấy được thông báo"})
		return
	}

	var unread int64
	DB.Model(&Notification{}).Where("user_id = ? AND is_read = 0", uid).Count(&unread)

	c.JSON(200, gin.H{
		"rows":   rows,
		"unread": unread,
	})
}

// PUT /private/notifications/mark-read  { ids?: number[] }
// Nếu không gửi ids → mark read tất cả
func markReadNotificationsHandler(c *gin.Context) {
	uid := uint(c.MustGet("claims").(jwt.MapClaims)["sub"].(float64))
	var req struct {
		IDs []uint `json:"ids"`
	}

	_ = c.ShouldBindJSON(&req) // optional

	q := DB.Model(&Notification{}).Where("user_id = ?", uid).Where("is_read = 0")
	if len(req.IDs) > 0 {
		q = q.Where("id IN ?", req.IDs)
	}
	if err := q.Update("is_read", true).Error; err != nil {
		c.JSON(500, gin.H{"error": "Không thể cập nhật thông báo"})
		return
	}

	c.Status(204)
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

	// Phí 1% (làm tròn lên)
	fee := (req.Amount*1 + 99) / 100 // ceil(amount*0.01)
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

type kycUpdateReq struct {
	FullName string `json:"fullName"` // đổi key để khớp FE
	IdNumber string `json:"idNumber"`
	Dob      string `json:"issueDate"` // hoặc "dob" nếu muốn
}

func updateKycHandler(c *gin.Context) {
	uid := uint(c.MustGet("claims").(jwt.MapClaims)["sub"].(float64))

	// FE gửi nickname + idNumber
	type kycUpdateReq struct {
		Nickname string `json:"nickname"`
		IdNumber string `json:"idNumber"`
	}
	var req kycUpdateReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	name := strings.TrimSpace(req.Nickname)
	num := strings.TrimSpace(req.IdNumber)
	if name == "" || num == "" {
		c.JSON(400, gin.H{"error": "Thiếu họ tên hoặc số CCCD"})
		return
	}

	var u User
	if err := DB.First(&u, uid).Error; err != nil {
		c.JSON(404, gin.H{"error": "User không tồn tại"})
		return
	}

	// Lưu vào các cột KYC hiện có (không đổi schema)
	up := map[string]any{
		"kyc_full_name": name, // nhận nickname nhưng lưu vào full_name
		"kyc_number":    num,
		"kyc_status":    "VERIFIED", // khớp enum('NONE','VERIFIED')
	}
	if err := DB.Model(&u).Updates(up).Error; err != nil {
		c.JSON(500, gin.H{"error": "Cập nhật KYC thất bại"})
		return
	}
	c.JSON(200, gin.H{"message": "Đã xác minh KYC"})
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

// handlers_dashboard.go
func dashboardOverviewHandler(c *gin.Context) {
	uid := uint(c.MustGet("claims").(jwt.MapClaims)["sub"].(float64))

	var out DashboardOverview
	if err := DB.Transaction(func(tx *gorm.DB) error {
		// 1) total assets
		var u User
		if err := tx.Select("coins, bonus_coins").First(&u, uid).Error; err != nil {
			return err
		}
		out.TotalAssets = u.Coins + u.BonusCoins

		// 2) F1 count
		if err := tx.Model(&User{}).
			Where("referred_by = ?", uid).
			Count(&out.F1Count).Error; err != nil {
			return err
		}

		// 3) F1 commission total (depth=1)
		if err := tx.Model(&CommissionTxn{}).
			Where("beneficiary_id = ? AND depth = 1", uid).
			Select("COALESCE(SUM(amount),0)").Scan(&out.F1CommissionTotal).Error; err != nil {
			return err
		}

		// 4) System commission total (depth 1..9)
		if err := tx.Model(&CommissionTxn{}).
			Where("beneficiary_id = ? AND depth BETWEEN 1 AND 9", uid).
			Select("COALESCE(SUM(amount),0)").Scan(&out.SystemCommissionTotal).Error; err != nil {
			return err
		}

		// 5) System count (F1..F9)
		// MySQL 8+ dùng CTE đệ quy cho nhanh, nếu không dùng được thì duyệt vòng (kèm hàm getUplines/downlines).
		type row struct{ Cnt int64 }
		var r row
		// depth tối đa 9
		q := `
            WITH RECURSIVE downline AS (
                SELECT id, referred_by, 1 AS depth FROM users WHERE referred_by = ?
                UNION ALL
                SELECT u.id, u.referred_by, d.depth+1
                FROM users u JOIN downline d ON u.referred_by = d.id
                WHERE d.depth < 9
            )
            SELECT COUNT(*) AS cnt FROM downline;
        `
		if err := tx.Raw(q, uid).Scan(&r).Error; err != nil {
			// fallback không CTE: đếm tầng bằng vòng lặp (tuỳ DB của bạn)
			// return err
			out.SystemCount = 0 // tối thiểu có giá trị
		} else {
			out.SystemCount = r.Cnt
		}
		return nil
	}); err != nil {
		c.JSON(500, gin.H{"error": "Lấy tổng quan thất bại"})
		return
	}
	c.JSON(200, out)
}
func myDownlinesHandler(c *gin.Context) {
	uid := uint(c.MustGet("claims").(jwt.MapClaims)["sub"].(float64))

	// optional ?depth=1..9 (0 or empty = all)
	var depth int
	if v := strings.TrimSpace(c.Query("depth")); v != "" {
		if n, _ := strconv.Atoi(v); n >= 1 && n <= 9 {
			depth = n
		}
	}

	rows := make([]DownlineRow, 0, 32)

	// Lấy F1..F9 theo vòng lặp (đơn giản, dễ hiểu)
	currentLevel := []uint{uid}
	for d := 1; d <= 9; d++ {
		// nếu filter depth và khác level đang xét -> bỏ qua query
		if depth != 0 && depth != d {
			// nhưng vẫn phải build currentLevel mới cho vòng sau
			var tmpUsers []User
			if len(currentLevel) > 0 {
				_ = DB.Where("referred_by IN ?", currentLevel).Select("id").Find(&tmpUsers).Error
			}
			next := make([]uint, 0, len(tmpUsers))
			for _, u := range tmpUsers {
				next = append(next, u.ID)
			}
			currentLevel = next
			continue
		}

		var users []User
		if len(currentLevel) > 0 {
			if err := DB.
				Where("referred_by IN ?", currentLevel).
				Select("id, username, v_ip_level , created_at").
				Find(&users).Error; err != nil {
				c.JSON(500, gin.H{"error": "Không tải được tuyến dưới"})
				return
			}
		}

		for _, u := range users {
			rows = append(rows, DownlineRow{
				ID:        u.ID,
				Username:  u.Username,
				Depth:     d,
				VIPLevel:  u.VIPLevel,
				CreatedAt: u.CreatedAt.Format(time.RFC3339),
			})
		}

		// chuẩn bị danh sách id cho level tiếp theo
		next := make([]uint, 0, len(users))
		for _, u := range users {
			next = append(next, u.ID)
		}
		currentLevel = next
	}

	c.JSON(200, gin.H{"rows": rows})
}

// handlers_dashboard.go
func dashboardCommissionsHandler(c *gin.Context) {
	uid := uint(c.MustGet("claims").(jwt.MapClaims)["sub"].(float64))

	now := time.Now()
	year, _ := strconv.Atoi(c.Query("year"))
	month, _ := strconv.Atoi(c.Query("month"))
	if year <= 0 {
		year = now.Year()
	}
	if month <= 0 || month > 12 {
		month = int(now.Month())
	}

	loc := time.Local
	start := time.Date(year, time.Month(month), 1, 0, 0, 0, 0, loc)
	end := start.AddDate(0, 1, 0)

	// gom theo ngày các giao dịch hoa hồng mà bạn là beneficiary (depth 1..9)
	type row struct {
		D int
		S int64
	}
	var rows []row
	if err := DB.Model(&CommissionTxn{}).
		Where("beneficiary_id = ? AND created_at >= ? AND created_at < ? AND depth BETWEEN 1 AND 9", uid, start, end).
		Select("DAY(created_at) as d, COALESCE(SUM(amount),0) as s").
		Group("d").Order("d").
		Scan(&rows).Error; err != nil {
		c.JSON(500, gin.H{"error": "Lấy lịch hoa hồng thất bại"})
		return
	}

	// build đủ ngày trong tháng
	daysInMonth := time.Date(year, time.Month(month)+1, 0, 0, 0, 0, 0, loc).Day()
	out := MonthlyCommissionResp{
		Year:  year,
		Month: month,
		Days:  make([]DailyCommission, daysInMonth),
	}
	m := map[int]int64{}
	var total int64 = 0
	for _, r := range rows {
		m[r.D] = r.S
		total += r.S
	}
	for d := 1; d <= daysInMonth; d++ {
		out.Days[d-1] = DailyCommission{Day: d, Amount: m[d]}
	}
	out.MonthTotal = total
	c.JSON(200, out)
}

/* ===== MAIN & CORS ===== */
func main() {
	connectDB()
	cleanupExpiredPromoCodes()

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
	r.GET("/public/leaderboard", publicLeaderboardHandler)

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
	priv.GET("/notifications", listNotificationsHandler)
	priv.PUT("/notifications/mark-read", markReadNotificationsHandler)
	priv.POST("/redeem-code", redeemCodeHandler) // 👈 user nhập code
	priv.GET("/dashboard/overview", dashboardOverviewHandler)
	priv.GET("/dashboard/commissions", dashboardCommissionsHandler)
	priv.POST("/redeem-bonus-code", redeemBonusCodeHandler)
	priv.GET("/downlines", myDownlinesHandler)
	priv.GET("/downlines/:id/dashboard", downlineDashboardHandler)
	priv.GET("/leaderboard/me", privateMyLeaderboardHandler)

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
	admin.POST("/promo-codes", adminCreatePromoCodeHandler)
	admin.GET("/promo-codes", adminListActivePromoCodesHandler)
	admin.POST("/promo-bonus-codes", adminCreateBonusCodesHandler)

	fmt.Println("🚀 Server running at :" + PORT)
	_ = r.Run(":" + PORT)
}
