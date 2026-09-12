package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/labstack/echo/v4"

	"github.com/poppsfs4za/backend-challenge/internal/adapter/inbound/rest"
	mongoadapter "github.com/poppsfs4za/backend-challenge/internal/adapter/outbound/mongo"
	"github.com/poppsfs4za/backend-challenge/internal/core/service"
	"github.com/poppsfs4za/backend-challenge/pkg/config"
	jwtpkg "github.com/poppsfs4za/backend-challenge/pkg/jwt"
	mw "github.com/poppsfs4za/backend-challenge/pkg/middleware"
	"github.com/poppsfs4za/backend-challenge/pkg/mongodb"
	"github.com/poppsfs4za/backend-challenge/pkg/validator"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	// ctx นี้จะถูกยกเลิกเมื่อกด Ctrl+C หรือ Docker ส่ง SIGTERM
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// ---------- outbound adapters ----------
	client, err := mongodb.New(ctx, cfg.MongoURI)
	if err != nil {
		log.Fatalf("mongo: %v", err)
	}
	log.Println("connected to mongodb")

	userRepo := mongoadapter.NewUserRepository(client.Database(cfg.MongoDatabase))
	if err := userRepo.EnsureIndexes(ctx); err != nil {
		log.Fatalf("ensure indexes: %v", err)
	}
	log.Println("indexes ensured")

	tokenManager := jwtpkg.NewManager(cfg.JWTSecret, cfg.JWTExpire)

	// ---------- core ----------
	userService := service.NewUserService(userRepo, tokenManager)

	// ---------- inbound adapter ----------
	e := echo.New()
	e.HideBanner = true
	e.HidePort = true
	e.Validator = validator.New()
	e.Use(mw.Logging())

	rest.RegisterRoutes(e, rest.NewUserHandler(userService), tokenManager)

	// ---------- background job (โจทย์ข้อ 6) ----------
	counter := service.NewUserCounter(userService, cfg.UserCountInterval)
	go counter.Start(ctx)

	// ---------- start ----------
	go func() {
		if err := e.Start(":" + cfg.AppPort); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("server: %v", err)
		}
	}()
	log.Printf("server listening on :%s", cfg.AppPort)

	<-ctx.Done() // บล็อกจนกว่าจะโดนสั่งปิด
	log.Println("shutting down...")

	// context ใหม่ เพราะตัวเดิมถูก cancel ไปแล้ว
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := e.Shutdown(shutdownCtx); err != nil {
		log.Printf("server shutdown: %v", err)
	}
	if err := client.Disconnect(shutdownCtx); err != nil {
		log.Printf("mongo disconnect: %v", err)
	}
	log.Println("bye")
}
