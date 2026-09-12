package service

import (
	"context"
	"log"
	"time"

	"github.com/poppbsfs4za/backend-challenge-7solutions/internal/core/port"
)

// UserCounter คือ background job ตามโจทย์ข้อ 6
type UserCounter struct {
	svc      port.UserService
	interval time.Duration
}

func NewUserCounter(svc port.UserService, interval time.Duration) *UserCounter {
	return &UserCounter{svc: svc, interval: interval}
}

// Start บล็อกจนกว่า ctx จะถูกยกเลิก — ให้ผู้เรียกตัดสินใจว่าจะรันใน goroutine ไหม
func (c *UserCounter) Start(ctx context.Context) {
	ticker := time.NewTicker(c.interval)
	defer ticker.Stop()

	log.Printf("user counter started (every %s)", c.interval)

	for {
		select {
		case <-ctx.Done():
			log.Println("user counter stopped")
			return

		case <-ticker.C:
			// ใส่ timeout ให้แต่ละรอบ ไม่ให้ query ค้างข้ามรอบ
			queryCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
			count, err := c.svc.Count(queryCtx)
			cancel()

			if err != nil {
				log.Printf("user counter: %v", err)
				continue
			}
			log.Printf("total users in database: %d", count)
		}
	}
}
