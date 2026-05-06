// SmartFM LMS — API Gateway Entry Point
//
// This is the single binary that runs all core services behind a unified
// Fiber HTTP server. Each service registers its routes on the shared router.
//
// Architecture reference: SmartFM_LMS_Architecture.md §2.1
package main

import (
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/gofiber/fiber/v2/middleware/recover"

	"github.com/priyansx01/smartfm-lms/internal/auth"
	"github.com/priyansx01/smartfm-lms/internal/config"
	"github.com/priyansx01/smartfm-lms/internal/course"
	"github.com/priyansx01/smartfm-lms/internal/database"
	"github.com/priyansx01/smartfm-lms/internal/middleware"
	"github.com/priyansx01/smartfm-lms/internal/storage"
)

func main() {
	// ─── Load Config ──────────────────────────────────────────────────────────
	cfg := config.Load()

	log.Printf("🚀 Starting %s (%s) on :%s", cfg.App.Name, cfg.App.Env, cfg.App.Port)

	// ─── Database ─────────────────────────────────────────────────────────────
	db, err := database.Connect(cfg.DB)
	if err != nil {
		log.Fatalf("❌ Database connection failed: %v", err)
	}
	defer db.Close()

	// ─── MinIO Storage ────────────────────────────────────────────────────────
	store, err := storage.NewClient(cfg.MinIO)
	if err != nil {
		// Non-fatal in development — MinIO might not be running
		log.Printf("⚠ MinIO connection failed (video uploads disabled): %v", err)
		store = nil
	}

	// ─── JWT Manager ──────────────────────────────────────────────────────────
	jwtMgr := auth.NewJWTManager(cfg.JWT)

	// ─── Services ─────────────────────────────────────────────────────────────
	authSvc := auth.NewService(db, jwtMgr)
	courseSvc := course.NewService(db, store)

	// ─── Fiber App ────────────────────────────────────────────────────────────
	app := fiber.New(fiber.Config{
		AppName:      cfg.App.Name,
		ErrorHandler: globalErrorHandler,
	})

	// Global middleware
	app.Use(recover.New())
	app.Use(logger.New(logger.Config{
		Format: "${time} | ${status} | ${latency} | ${method} ${path}\n",
	}))
	app.Use(cors.New(cors.Config{
		AllowOrigins:     cfg.CORS.Origins,
		AllowMethods:     "GET,POST,PUT,PATCH,DELETE,OPTIONS",
		AllowHeaders:     "Origin,Content-Type,Accept,Authorization",
		AllowCredentials: true,
	}))

	// ─── Health Check ─────────────────────────────────────────────────────────
	app.Get("/health", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{
			"status":  "ok",
			"service": cfg.App.Name,
			"env":     cfg.App.Env,
		})
	})

	// ─── API v1 Routes ────────────────────────────────────────────────────────
	api := app.Group("/api/v1")

	// Public routes (no auth required)
	authHandler := auth.NewHandler(authSvc)
	api.Post("/auth/login", authHandler.Login)
	api.Post("/auth/refresh", authHandler.Refresh)

	// Protected routes (JWT required)
	api.Use(middleware.JWTAuth(jwtMgr))

	// Auth (protected)
	api.Get("/auth/me", authHandler.Me)

	// Courses (protected)
	courseHandler := course.NewHandler(courseSvc)
	courseHandler.RegisterRoutes(api)

	// ─── Graceful Shutdown ────────────────────────────────────────────────────
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		if err := app.Listen(":" + cfg.App.Port); err != nil {
			log.Fatalf("❌ Server error: %v", err)
		}
	}()

	<-quit
	log.Println("⏳ Shutting down gracefully...")
	if err := app.Shutdown(); err != nil {
		log.Printf("❌ Shutdown error: %v", err)
	}
	log.Println("✅ Server stopped")
}

// globalErrorHandler catches unhandled panics and returns a clean 500.
func globalErrorHandler(c *fiber.Ctx, err error) error {
	code := fiber.StatusInternalServerError
	if e, ok := err.(*fiber.Error); ok {
		code = e.Code
	}
	return c.Status(code).JSON(fiber.Map{
		"success": false,
		"message": err.Error(),
	})
}
