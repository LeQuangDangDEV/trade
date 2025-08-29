package main

import (
	"errors"
	"fmt"
	"log"
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
	"gorm.io/gorm/schema"
)

/* ===== CONFIG ===== */

type Config struct {
	Port        string
	DSN         string
	JWTSecret   string
	CorsOrigins []string
	UploadDir   string
	// Optional seeding admin
	AdminEmail    string
	AdminPassword string
}

func getenv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func splitAndTrimCSV(s string) []string {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

var cfg Config

func loadConfig() {
	cfg = Config{
		Port:        getenv("PORT", "8080"),
		DSN:         getenv("DSN", "root@tcp(127.0.0.1:3306)/trade?charset=utf8mb4&parseTime=True&loc=Local"),
		JWTSecret:   getenv("JWT_SECRET", "change-this-secret"),
		CorsOrigins: splitAndTrimCSV(getenv("CORS_ORIGINS", "http://localhost:5173")),
		UploadDir:   getenv("UPLOAD_DIR", "uploads"),

		AdminEmail:    os.Getenv("ADMIN_EMAIL"),
		AdminPassword: os.Getenv("ADMIN_PASSWORD"),
	}
	if cfg.JWTSecret == "change-this-secret" {
		log.Println("⚠️  JWT_SECRET đang dùng giá trị mặc định. Hãy đặt biến môi trường JWT_SECRET trong môi trường production.")
	}
}

/* ===== DB & MODELS ===== */

var DB *gorm.DB
var uploadsAbs string

type AdminUserRow struct {
	ID         uint   `json:"id"`
	Nickname   string `json:"nickname"`
	Email      string `json:"email"`
	VIPLevel   int    `json:"vipLevel"`
	TotalTopup int64  `json:"totalTopup"`
	Coins      int64  `json:"coins"`
}

type User struct {
	ID           uint      `gorm:"primaryKey" json:"id"`
	Name         string    `json:"name"`
	Email        string    `gorm:"uniqueIndex;size:191" json:"email"`
	Phone        string    `json:"phone"`
	AvatarURL    string    `json:"avatarUrl"`
	PasswordHash string    `json:"-"`
	Role         string    `gorm:"type:enum('admin','user');default:'user';index" json:"role"`
	Coins        int64     `gorm:"not null;default:0" json:"coins"`
	TotalTopup   int64     `gorm:"not null;default:0" json:"totalTopup"`
	VIPLevel     int       `gorm:"not null;default:0" json:"vipLevel"`
	CreatedAt    time.Time `json:"createdAt"`
	UpdatedAt    time.Time `json:"updatedAt"`
}

type VipTier struct {
	ID        uint   `gorm:"primaryKey"`
	Level     int    `gorm:"uniqueIndex;not null"`
	Name      string `gorm:"size:50;not null"`
	MinTopup  int64  `gorm:"not null;default:0"`
	CreatedAt time.Time
	UpdatedAt time.Time
}

type CoinTxn struct {
	ID        uint   `gorm:"primaryKey"`
	UserID    uint   `gorm:"not null;index"`
	AdminID   uint   `gorm:"not null;index"`
	Amount    int64  `gorm:"not null"`
	Note      string `gorm:"size:255"`
	CreatedAt time.Time

	User  User `gorm:"foreignKey:UserID;constraint:OnUpdate:CASCADE,OnDelete:RESTRICT;"`
	Admin User `gorm:"foreignKey:AdminID;constraint:OnUpdate:CASCADE,OnDelete:RESTRICT;"`
}

/* ===== DTO ===== */

type RegisterRequest struct {
	Name     string `json:"name" binding:"required,min=2,max=100"`
	Email    string `json:"email" binding:"required,email"`
	Phone    string `json:"phone" binding:"required,min=8,max=20"`
	Password string `json:"password" binding:"required,min=6,max=100"`
}
type LoginRequest struct {
	Email    string `json:"email" binding:"required,email"`
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
type ChangePasswordRequest struct {
	Current string `json:"current" binding:"required"`
	New     string `json:"new" binding:"required,min=6,max=100"`
}

/* ===== BOOTSTRAP ===== */

func connectDB() {
	db, err := gorm.Open(mysql.Open(cfg.DSN), &gorm.Config{
		NamingStrategy: schema.NamingStrategy{SingularTable: false},
	})
	if err != nil {
		log.Fatal("❌ Cannot connect DB:", err)
	}
	DB = db
	if err := DB.AutoMigrate(&User{}, &VipTier{}, &CoinTxn{}); err != nil {
		log.Fatal("❌ AutoMigrate error:", err)
	}
	seedVipTiers()
	seedAdminIfConfigured()
	fmt.Println("✅ DB migrated")
}

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
	fmt.Println("🌱 Seeded vip_tiers")
}

func seedAdminIfConfigured() {
	if cfg.AdminEmail == "" || cfg.AdminPassword == "" {
		return
	}
	var count int64
	DB.Model(&User{}).Where("email = ?", strings.ToLower(cfg.AdminEmail)).Count(&count)
	if count > 0 {
		return
	}
	hash, _ := bcrypt.GenerateFromPassword([]byte(cfg.AdminPassword), bcrypt.DefaultCost)
	admin := User{
		Name: "Administrator", Email: strings.ToLower(cfg.AdminEmail),
		Phone: "", PasswordHash: string(hash), Role: "admin",
	}
	if err := DB.Create(&admin).Error; err != nil {
		log.Println("⚠️  Seed admin failed:", err)
	} else {
		log.Println("👑 Seeded admin user:", cfg.AdminEmail)
	}
}

/* ===== AUTH HELPERS ===== */

func parseBearerToken(h string) (string, error) {
	if !strings.HasPrefix(h, "Bearer ") {
		return "", errors.New("missing bearer")
	}
	return h[7:], nil
}

func authRequired() gin.HandlerFunc {
	return func(c *gin.Context) {
		tokenStr, err := parseBearerToken(c.GetHeader("Authorization"))
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Missing token"})
			return
		}
		t, err := jwt.Parse(tokenStr, func(t *jwt.Token) (interface{}, error) {
			// enforce HMAC
			if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
			}
			return []byte(cfg.JWTSecret), nil
		})
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

/* ===== VIP LOGIC ===== */

func computeVIPLevel(totalTopup int64) int {
	var tiers []VipTier
	DB.Order("min_topup asc").Find(&tiers)
	level := 0
	for _, t := range tiers {
		if totalTopup >= t.MinTopup && t.Level > level {
			level = t.Level
		}
	}
	return level
}

/* ===== AUTH & PROFILE ===== */

func registerHandler(c *gin.Context) {
	var req RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	var existing User
	if err := DB.Where("email = ?", strings.ToLower(req.Email)).First(&existing).Error; err == nil {
		c.JSON(409, gin.H{"error": "Email đã tồn tại"})
		return
	}
	hash, _ := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	user := User{
		Name: req.Name, Email: strings.ToLower(req.Email), Phone: req.Phone,
		PasswordHash: string(hash),
		Role:         "user", Coins: 0, TotalTopup: 0, VIPLevel: 0,
	}
	if err := DB.Create(&user).Error; err != nil {
		c.JSON(500, gin.H{"error": "Không thể tạo người dùng"})
		return
	}
	c.JSON(201, gin.H{"message": "Đăng ký thành công"})
}

func loginHandler(c *gin.Context) {
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	var user User
	if err := DB.Where("email = ?", strings.ToLower(req.Email)).First(&user).Error; err != nil {
		c.JSON(401, gin.H{"error": "Sai email hoặc mật khẩu"})
		return
	}
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)); err != nil {
		c.JSON(401, gin.H{"error": "Sai email hoặc mật khẩu"})
		return
	}
	claims := jwt.MapClaims{
		"sub": user.ID, "email": user.Email, "role": user.Role,
		"exp": time.Now().Add(24 * time.Hour).Unix(),
	}
	t := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, _ := t.SignedString([]byte(cfg.JWTSecret))
	user.PasswordHash = ""
	c.JSON(200, AuthResponse{Token: signed, User: user})
}

func meHandler(c *gin.Context) {
	claims := c.MustGet("claims").(jwt.MapClaims)
	email, _ := claims["email"].(string)
	var user User
	if err := DB.Where("email = ?", strings.ToLower(email)).First(&user).Error; err != nil {
		c.JSON(404, gin.H{"error": "User không tồn tại"})
		return
	}
	user.PasswordHash = ""
	c.JSON(200, gin.H{"user": user})
}

func updateProfileHandler(c *gin.Context) {
	claims := c.MustGet("claims").(jwt.MapClaims)
	email, _ := claims["email"].(string)
	var req ProfileUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	var user User
	if err := DB.Where("email = ?", strings.ToLower(email)).First(&user).Error; err != nil {
		c.JSON(404, gin.H{"error": "User không tồn tại"})
		return
	}
	user.Name = req.Name
	user.Phone = req.Phone
	user.AvatarURL = strings.TrimSpace(req.AvatarURL)
	if err := DB.Save(&user).Error; err != nil {
		c.JSON(500, gin.H{"error": "Không thể cập nhật hồ sơ"})
		return
	}
	user.PasswordHash = ""
	c.JSON(200, gin.H{"user": user, "message": "Cập nhật thành công"})
}

func changePasswordHandler(c *gin.Context) {
	claims := c.MustGet("claims").(jwt.MapClaims)
	email, _ := claims["email"].(string)

	var req ChangePasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	var user User
	if err := DB.Where("email = ?", strings.ToLower(email)).First(&user).Error; err != nil {
		c.JSON(404, gin.H{"error": "User không tồn tại"})
		return
	}
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Current)); err != nil {
		c.JSON(400, gin.H{"error": "Mật khẩu hiện tại không đúng"})
		return
	}
	hash, _ := bcrypt.GenerateFromPassword([]byte(req.New), bcrypt.DefaultCost)
	if err := DB.Model(&user).Update("password_hash", string(hash)).Error; err != nil {
		c.JSON(500, gin.H{"error": "Không thể đổi mật khẩu"})
		return
	}
	c.JSON(200, gin.H{"message": "Đổi mật khẩu thành công"})
}

/* ===== VIP / COIN API ===== */

func getVipTiersHandler(c *gin.Context) {
	var tiers []VipTier
	DB.Order("level asc").Find(&tiers)
	c.JSON(200, gin.H{"tiers": tiers})
}

func getWalletHandler(c *gin.Context) {
	claims := c.MustGet("claims").(jwt.MapClaims)
	email, _ := claims["email"].(string)
	var user User
	if err := DB.Where("email = ?", strings.ToLower(email)).First(&user).Error; err != nil {
		c.JSON(404, gin.H{"error": "User không tồn tại"})
		return
	}
	// ensure VIP correct
	newLevel := computeVIPLevel(user.TotalTopup)
	if newLevel != user.VIPLevel {
		DB.Model(&user).Update("vip_level", newLevel)
		user.VIPLevel = newLevel
	}
	c.JSON(200, gin.H{
		"coins":      user.Coins,
		"totalTopup": user.TotalTopup,
		"vipLevel":   user.VIPLevel,
	})
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
		// add coin & total_topup
		if err := tx.Model(&user).
			Updates(map[string]interface{}{
				"coins":       gorm.Expr("coins + ?", req.Amount),
				"total_topup": gorm.Expr("total_topup + ?", req.Amount),
			}).Error; err != nil {
			return err
		}
		if err := tx.First(&user, user.ID).Error; err != nil {
			return err
		}
		newLevel := computeVIPLevel(user.TotalTopup)
		if err := tx.Model(&user).Update("vip_level", newLevel).Error; err != nil {
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

/* ===== LISTING / PAGINATION ===== */

type PageResult struct {
	Total int64       `json:"total"`
	Rows  interface{} `json:"rows"`
}

func parsePage(c *gin.Context) (int, int) {
	page, _ := strconv.Atoi(c.Query("page"))
	if page < 1 {
		page = 1
	}
	size, _ := strconv.Atoi(c.Query("pageSize"))
	if size <= 0 || size > 100 {
		size = 20
	}
	return page, size
}

func adminSearchUsersHandler(c *gin.Context) {
	vipLevel := strings.TrimSpace(c.Query("vipLevel"))
	nickname := strings.TrimSpace(c.Query("nickname"))
	page, size := parsePage(c)

	var users []User
	q := DB.Model(&User{})

	if vipLevel != "" {
		q = q.Where("vip_level = ?", vipLevel)
	}
	if nickname != "" {
		like := "%" + strings.ToLower(nickname) + "%"
		q = q.Where("LOWER(name) LIKE ?", like)
	}

	var total int64
	if err := q.Count(&total).Error; err != nil {
		c.JSON(500, gin.H{"error": "Count failed"})
		return
	}
	if err := q.Order("vip_level desc, id asc").
		Limit(size).Offset((page - 1) * size).
		Find(&users).Error; err != nil {
		c.JSON(500, gin.H{"error": "Query failed"})
		return
	}

	rows := make([]AdminUserRow, 0, len(users))
	for _, u := range users {
		rows = append(rows, AdminUserRow{
			ID: u.ID, Nickname: u.Name, Email: u.Email,
			VIPLevel: u.VIPLevel, TotalTopup: u.TotalTopup, Coins: u.Coins,
		})
	}
	c.JSON(200, PageResult{Total: total, Rows: rows})
}

func listUserTxnsHandler(c *gin.Context) {
	claims := c.MustGet("claims").(jwt.MapClaims)
	userID := uint(claims["sub"].(float64))

	page, size := parsePage(c)
	var total int64
	DB.Model(&CoinTxn{}).Where("user_id = ?", userID).Count(&total)

	var txns []CoinTxn
	if err := DB.Preload("Admin").
		Where("user_id = ?", userID).
		Order("created_at desc").
		Limit(size).Offset((page - 1) * size).
		Find(&txns).Error; err != nil {
		c.JSON(500, gin.H{"error": "Query failed"})
		return
	}
	c.JSON(200, PageResult{Total: total, Rows: txns})
}

func adminListTxnsHandler(c *gin.Context) {
	page, size := parsePage(c)
	userID := strings.TrimSpace(c.Query("userId"))
	adminID := strings.TrimSpace(c.Query("adminId"))

	q := DB.Preload("User").Preload("Admin").Model(&CoinTxn{})
	if userID != "" {
		q = q.Where("user_id = ?", userID)
	}
	if adminID != "" {
		q = q.Where("admin_id = ?", adminID)
	}

	var total int64
	if err := q.Count(&total).Error; err != nil {
		c.JSON(500, gin.H{"error": "Count failed"})
		return
	}
	var txns []CoinTxn
	if err := q.Order("created_at desc").
		Limit(size).Offset((page - 1) * size).
		Find(&txns).Error; err != nil {
		c.JSON(500, gin.H{"error": "Query failed"})
		return
	}
	c.JSON(200, PageResult{Total: total, Rows: txns})
}

/* ===== MAIN & CORS ===== */

func ensureUploadsDirAbs() (string, error) {
	ex, err := os.Executable()
	if err != nil {
		return "", err
	}
	base := filepath.Dir(ex)
	abs := filepath.Join(base, cfg.UploadDir)
	if err := os.MkdirAll(abs, 0755); err != nil {
		return "", err
	}
	return abs, nil
}

func isAllowedOrigin(origin string) bool {
	if origin == "" {
		return false
	}
	for _, o := range cfg.CorsOrigins {
		if strings.EqualFold(o, origin) {
			return true
		}
	}
	return false
}

func main() {
	loadConfig()
	connectDB()
	r := gin.Default()

	// CORS
	r.Use(func(c *gin.Context) {
		origin := c.Request.Header.Get("Origin")
		if origin != "" && isAllowedOrigin(origin) {
			c.Writer.Header().Set("Access-Control-Allow-Origin", origin)
		} else if len(cfg.CorsOrigins) > 0 {
			// fallback: origin đầu tiên trong whitelist (phù hợp cho SSR/cron không gửi header Origin)
			c.Writer.Header().Set("Access-Control-Allow-Origin", cfg.CorsOrigins[0])
		}
		c.Writer.Header().Set("Vary", "Origin")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(204)
			return
		}
		c.Next()
	})

	// serve uploads (optional)
	if abs, err := ensureUploadsDirAbs(); err == nil {
		uploadsAbs = abs
		r.Static("/uploads", uploadsAbs)
	}

	// Public
	r.POST("/register", registerHandler)
	r.POST("/login", loginHandler)
	r.GET("/vip-tiers", getVipTiersHandler)

	// Private
	priv := r.Group("/private")
	priv.Use(authRequired())
	priv.GET("/me", meHandler)
	priv.PUT("/profile", updateProfileHandler)
	priv.GET("/wallet", getWalletHandler)
	priv.POST("/change-password", changePasswordHandler)
	priv.GET("/txns", listUserTxnsHandler)

	// Admin
	admin := r.Group("/admin")
	admin.Use(authRequired(), adminRequired())
	admin.POST("/topup", adminTopupHandler)
	admin.GET("/users", adminSearchUsersHandler) // now paginated
	admin.GET("/txns", adminListTxnsHandler)     // new

	srv := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           r,
		ReadTimeout:       10 * time.Second,
		ReadHeaderTimeout: 10 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
	fmt.Println("🚀 Server running at :" + cfg.Port)
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatal("server error:", err)
	}
}
