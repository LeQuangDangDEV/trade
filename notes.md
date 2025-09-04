1) Tổng quan

Đăng ký/đăng nhập đã chuyển hoàn toàn sang username + password (không còn email).

Thêm mật khẩu cấp 2 (second password) để khôi phục mật khẩu, và PIN giao dịch (6 số) để xác nhận chuyển coin.

Hồ sơ người dùng (Profile): chỉnh tên/điện thoại/avatar, đổi mật khẩu, cập nhật bảo mật, KYC CCCD tự duyệt (auto-verify).

Ví: xem số dư/VIP/tổng nạp; chuyển coin yêu cầu PIN, phí 0.5% (làm tròn lên).

Kho báu: mở rương (50 coin/lần), tỉ lệ thưởng: 2% ⇒ +100 coin; 3% ⇒ 1 viên DB1..DB7; còn lại +10 coin. Có hợp nhất đủ 7 viên ⇒ +5000 coin.

Chợ: đăng bán DBx, mua, rút lại bài đăng (không cho tự mua bài của mình).

Admin: nạp/rút/xoá người dùng; tìm kiếm người dùng; xem chi tiết user (kể cả trạng thái KYC) và tải ảnh KYC (front/back) qua endpoint riêng (chỉ admin).

Referrals: có referral code/link, tính hoa hồng mua VIP (9 tầng).

2) Cấu hình & Chạy
Backend

File: main.go

Biến cấu hình (hard-code trong main.go, tuỳ chỉnh nếu cần):

PORT = "8080"

DSN = "root@tcp(127.0.0.1:3306)/trade?..."

JWT_SECRET = "change-this-secret"

CORS_ORIGIN = "http://localhost:5173"

UPLOAD_DIR = "uploads" (được serve tĩnh /uploads)

KYC_DIR = "kyc_files" (không public)

Tạo thư mục nếu thiếu: tự động khi khởi chạy.

Chạy:

go run main.go


DB MySQL: AutoMigrate chạy khi khởi động (tạo/alter bảng cần thiết).

Frontend

Vite (Vue 3)

Biến môi trường:
.env hoặc .env.local

VITE_API_BASE=http://localhost:8080


Chạy:

npm install
npm run dev

3) Lược đồ DB (chính)

Bảng users (các trường quan trọng):

username (unique), name, phone, avatar_url

password_hash

role (admin|user)

coins, total_topup, v_ip_level (cột trong DB cho VIPLevel)

second_password_hash, txn_pin_hash

kyc_status (NONE|VERIFIED)

kyc_full_name, kyc_number, kyc_dob

kyc_front_path, kyc_back_path

referral_code (unique), referred_by

timestamps

Bảng khác:

vip_tiers, coin_txns, transfer_txns, referral_rewards, vip_purchase_txns, commission_txns, withdraw_txns, inventory_items, chest_txns, market_listings.

4) API map (đang dùng)
Public

POST /register — body: { username, password, nickname|name, phone, ref? }

POST /login — { username, password } ⇒ { token, user }

GET /vip-tiers

GET /market?code=DBx

POST /forgot-password — { username, secPassword, newPassword }

Private (Bearer token)

GET /private/me

PUT /private/profile — { name, phone, avatarUrl? }

GET /private/wallet

POST /private/upload — multipart, field file ⇒ { url:"/uploads/xxx" }

POST /private/transfer — { toUsername, amount, note?, txnPin }

GET /private/referral-info

POST /private/buy-vip

Lịch sử:

GET /private/history/topups

GET /private/history/withdraws

GET /private/history/transfers

GET /private/history/vip

GET /private/history/commissions

Kho báu:

POST /private/chest-open

GET /private/inventory

POST /private/merge-dragon

Chợ:

POST /private/market/list — { code, qty, pricePerUnit }

POST /private/market/buy — { listingId, qty }

POST /private/market/withdraw — { listingId, qty? }

Mật khẩu/bảo mật:

POST|PUT /private/change-password — { oldPassword, newPassword }

PUT /private/security — { oldSecondPassword?, newSecondPassword?, newTxnPin? }

KYC:

Khuyến nghị: POST /private/kyc-submit (multipart)
fields: fullName, dob (YYYY-MM-DD), number (CCCD), front (file), back (file)
⇒ tự duyệt (VERIFIED).

Tuỳ chọn: PUT /private/kyc (JSON) — { frontPath, backPath } (auto-approve).

Admin (Bearer token + role=admin)

POST /admin/topup

POST /admin/withdraw

GET /admin/users — lọc theo vipLevel, username, nickname

GET /admin/users/:id — chi tiết user (gồm trạng thái/metadata KYC, cờ có ảnh)

DELETE /admin/users/:id — xoá cứng

GET /admin/kyc/:userId/front — trả file (nếu dùng route này)

GET /admin/kyc/:userId/back

GET /admin/kyc-file/:userId/:side — side=front|back (route hiện tại để tải ảnh KYC)

5) Luồng nghiệp vụ nổi bật
Chuyển coin

Body: { toUsername, amount, note?, txnPin }

Phí: 0.5% do người gửi trả (làm tròn lên, ceil(amount*0.005)).

Kiểm tra:

Không tự chuyển cho chính mình.

Bắt buộc đã đặt txnPin (6 số).

Số dư đủ amount + fee.

Bảo mật

PUT /private/security:

Đổi/đặt mật khẩu cấp 2: nếu đã có, yêu cầu oldSecondPassword; newSecondPassword ≥ 6 ký tự.

Đặt/đổi PIN 6 số: newTxnPin chính xác 6 chữ số.

Quên mật khẩu

POST /forgot-password:

{ username, secPassword (mật khẩu cấp 2), newPassword }

So khớp secPassword rồi đổi password.

KYC CCCD (tự duyệt)

POST /private/kyc-submit (multipart, đề xuất dùng):

Nhập: fullName, dob, number, upload front, back

Lưu file vào thư mục private kyc_files/

Lưu thông tin (fullname/dob/number) vào users, kyc_status="VERIFIED"

Chỉ admin có thể tải ảnh qua /admin/kyc-file/:userId/:side

Lưu ý hiển thị cho người dùng: bắt buộc Họ & tên, Ngày sinh, Số CCCD phải trùng khớp CCCD.

Kho báu

Mở rương: -50 coin, random thưởng:

2%: +100 coin

3%: +1 viên DBx

95%: +10 coin

Hợp nhất đủ DB1..DB7: +5000 coin.

Chợ

Đăng bán: trừ vật phẩm khỏi túi ⇒ tạo market_listing.

Mua: không cho người bán tự mua; trừ coin người mua ⇒ cộng coin người bán ⇒ cộng vật phẩm cho người mua ⇒ trừ số lượng bài đăng.

Rút lại: trả DBx về túi, trừ khỏi listing; hết thì is_active=false.

6) FE đã chỉnh

src/api.ts:

Chuyển hoàn toàn sang username; thêm AuthError và clear token khi 401.

Thêm API updateSecurity, changePassword, kycSubmit/updateKyc, adminUserDetail, adminKycImage.

Profile.vue:

Form hồ sơ (tên/điện thoại/avatar).

Đổi mật khẩu.

Bảo mật (mật khẩu cấp 2 + PIN).

KYC CCCD auto-verify + nhập fullName/dob/number (nhắc trùng CCCD).

Navbar: hiển thị avatar từ currentUser.avatarUrl (fallback ảnh mặc định).

Home (kho báu/chợ): modal kết quả mở rương; rút listing; chặn tự mua; tải túi ngay khi đăng nhập.

VipAdmin.vue:

Lọc người dùng; nạp/rút/xoá.

Thêm nút Xem chi tiết user và hai nút Xem ảnh CCCD (front/back) (chỉ admin).

7) Thay đổi phá vỡ (breaking changes)

Email không còn dùng cho auth hay chuyển khoản → tất cả thay bằng username:

transfer dùng toUsername

admin filter hỗ trợ username, nickname (tham số email vẫn map vào username để tương thích tạm thời).

Route change password nằm dưới /private/change-password (POST hoặc PUT đều hỗ trợ).

KYC: ưu tiên POST /private/kyc-submit (multipart). Route JSON /private/kyc vẫn tồn tại (auto-approve) nếu cần.

8) Mẹo kiểm thử nhanh

Tạo admin:

UPDATE users SET role='admin' WHERE id=1;


Đặt PIN rồi chuyển:

# login lấy token
# đặt PIN
curl -X PUT http://localhost:8080/private/security \
  -H "Authorization: Bearer <TOKEN>" -H "Content-Type: application/json" \
  -d '{"newTxnPin":"123456"}'
# chuyển
curl -X POST http://localhost:8080/private/transfer \
  -H "Authorization: Bearer <TOKEN>" -H "Content-Type: application/json" \
  -d '{"toUsername":"userB","amount":100,"txnPin":"123456"}'


KYC (multipart):

curl -X POST http://localhost:8080/private/kyc-submit \
  -H "Authorization: Bearer <TOKEN>" \
  -F "fullName=Nguyen Van A" \
  -F "dob=1990-01-01" \
  -F "number=012345678901" \
  -F "front=@/path/front.jpg" \
  -F "back=@/path/back.jpg"

9) Bảo mật & Quy ước

PIN/mật khẩu cấp 2/đăng nhập đều được bcrypt hash.

Ảnh KYC lưu ở thư mục riêng kyc_files/ (không public). Admin tải qua endpoint có kiểm tra role=admin.

JWT chứa sub, username, role, exp. Hết hạn 24h.

CORS mở cho CORS_ORIGIN.

10) Việc tiếp theo (gợi ý)

Rate limit các endpoint nhạy cảm (login, transfer, kyc-submit).

Nhật ký admin khi xem ảnh KYC.

Trang Admin UI: bộ lọc nâng cao, export CSV.

Refill/rollback tool khi fail trong quá trình mua/bán trên chợ (hiện đã dùng transaction).
