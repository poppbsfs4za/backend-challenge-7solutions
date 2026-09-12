package domain

import "time"

// User คือ entity ตามโจทย์ข้อ 1
type User struct {
	ID        string
	Name      string
	Email     string
	Password  string // เก็บเฉพาะค่าที่ hash แล้วเท่านั้น
	CreatedAt time.Time
}
