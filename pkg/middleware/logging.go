package middleware

import (
	"log"
	"time"

	"github.com/labstack/echo/v4"
)

// Logging บันทึก HTTP method, path และเวลาที่ใช้ ตามโจทย์ข้อ 5
func Logging() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			start := time.Now()

			err := next(c) // ส่งต่อให้ชั้นถัดไปทำงาน

			// บรรทัดนี้ทำงาน "ขากลับ" หลัง handler เสร็จแล้ว
			log.Printf("method=%s path=%s status=%d duration=%s",
				c.Request().Method,
				c.Request().URL.Path,
				c.Response().Status,
				time.Since(start),
			)
			return err
		}
	}
}
