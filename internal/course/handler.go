package course

import (
	"errors"

	"github.com/gofiber/fiber/v2"

	"github.com/priyansx01/smartfm-lms/internal/domain"
	"github.com/priyansx01/smartfm-lms/pkg/response"
)

// Handler exposes course HTTP endpoints.
type Handler struct {
	svc *Service
}

// NewHandler creates a new course handler.
func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

// RegisterRoutes mounts course routes.
func (h *Handler) RegisterRoutes(api fiber.Router) {
	courses := api.Group("/courses")
	courses.Get("/", h.List)
	courses.Get("/:id", h.Get)
	courses.Post("/", h.Create)
	courses.Patch("/:id", h.Update)
	courses.Delete("/:id", h.Delete)

	// Modules
	courses.Get("/:id/modules", h.ListModules)
	courses.Post("/:id/modules", h.CreateModule)
	courses.Delete("/:id/modules/:moduleId", h.DeleteModule)

	// Upload + Playback
	courses.Post("/:id/modules/:moduleId/upload-url", h.GetUploadURL)
	courses.Get("/:id/modules/:moduleId/playback-url", h.GetPlaybackURL)
}

// ─── Course Handlers ──────────────────────────────────────────────────────────

func (h *Handler) List(c *fiber.Ctx) error {
	courses, err := h.svc.ListCourses(
		c.Query("status"),
		c.Query("search"),
		c.Query("category"),
	)
	if err != nil {
		return response.InternalError(c, "Failed to list courses")
	}
	if courses == nil {
		courses = []domain.Course{} // never return null
	}
	return response.OK(c, fiber.Map{
		"items":       courses,
		"total":       len(courses),
		"page":        1,
		"per_page":    len(courses),
		"total_pages": 1,
	})
}

func (h *Handler) Get(c *fiber.Ctx) error {
	course, err := h.svc.GetCourse(c.Params("id"))
	if err != nil {
		if errors.Is(err, ErrCourseNotFound) {
			return response.NotFound(c, "Course not found")
		}
		return response.InternalError(c, "Failed to get course")
	}
	return response.OK(c, course)
}

func (h *Handler) Create(c *fiber.Ctx) error {
	var req CreateCourseRequest
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "Invalid request body")
	}
	if req.Title == "" {
		return response.BadRequest(c, "Title is required")
	}

	userID := c.Locals("user_id").(string)
	course, err := h.svc.CreateCourse(userID, req)
	if err != nil {
		return response.InternalError(c, "Failed to create course")
	}
	return response.Created(c, course)
}

func (h *Handler) Update(c *fiber.Ctx) error {
	var req CreateCourseRequest
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "Invalid request body")
	}

	course, err := h.svc.UpdateCourse(c.Params("id"), req)
	if err != nil {
		return response.InternalError(c, "Failed to update course")
	}
	return response.OK(c, course)
}

func (h *Handler) Delete(c *fiber.Ctx) error {
	if err := h.svc.DeleteCourse(c.Params("id")); err != nil {
		return response.InternalError(c, "Failed to delete course")
	}
	return response.NoContent(c)
}

// ─── Module Handlers ──────────────────────────────────────────────────────────

func (h *Handler) ListModules(c *fiber.Ctx) error {
	modules, err := h.svc.ListModules(c.Params("id"))
	if err != nil {
		return response.InternalError(c, "Failed to list modules")
	}
	if modules == nil {
		modules = []domain.Module{}
	}
	return response.OK(c, modules)
}

func (h *Handler) CreateModule(c *fiber.Ctx) error {
	var req CreateModuleRequest
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "Invalid request body")
	}

	module, err := h.svc.CreateModule(c.Params("id"), req)
	if err != nil {
		return response.InternalError(c, "Failed to create module")
	}
	return response.Created(c, module)
}

func (h *Handler) DeleteModule(c *fiber.Ctx) error {
	if err := h.svc.DeleteModule(c.Params("id"), c.Params("moduleId")); err != nil {
		return response.InternalError(c, "Failed to delete module")
	}
	return response.NoContent(c)
}

// ─── Upload + Playback ────────────────────────────────────────────────────────

func (h *Handler) GetUploadURL(c *fiber.Ctx) error {
	var req UploadURLRequest
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "Invalid request body")
	}

	url, objectKey, err := h.svc.GetUploadURL(c.Params("id"), c.Params("moduleId"), req.FileName)
	if err != nil {
		return response.InternalError(c, "Failed to generate upload URL")
	}

	return response.OK(c, fiber.Map{
		"upload_url": url,
		"object_key": objectKey,
		"expires_at": "1h",
	})
}

func (h *Handler) GetPlaybackURL(c *fiber.Ctx) error {
	url, expiresAt, err := h.svc.GetPlaybackURL(c.Params("id"), c.Params("moduleId"))
	if err != nil {
		return response.InternalError(c, "Failed to generate playback URL")
	}

	return response.OK(c, fiber.Map{
		"playback_url": url,
		"expires_at":   expiresAt,
	})
}
