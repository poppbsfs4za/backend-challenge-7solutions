package port

import "time"

// TokenManager แยกออกมาเป็น port เพราะ JWT คือรายละเอียดเทคนิค
// วันหน้าเปลี่ยนไปใช้ PASETO หรือ opaque token แกนกลางไม่ต้องแก้
type TokenManager interface {
	Generate(userID, email string) (token string, expiresAt time.Time, err error)
}
