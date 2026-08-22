package handlers

import (
	"context"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"

	"github.com/as76513/JobSonar/services/api/internal/store"
)

type Jobs interface {
	ListJobs(ctx context.Context) ([]store.Job, error)
	GetJob(ctx context.Context, id uuid.UUID) (store.Job, error)
}

type Companies interface {
	CreateCompany(ctx context.Context, name, ats, token string) (store.Company, error)
}

type Handler struct {
	jobs      Jobs
	companies Companies
}

func New(jobs Jobs, companies Companies) *Handler {
	return &Handler{jobs: jobs, companies: companies}
}

func (h *Handler) Mount(app *fiber.App) {
	app.Get("/jobs", h.listJobs)
	app.Get("/jobs/:id", h.getJob)
	app.Post("/companies", h.createCompany)
}

func (h *Handler) listJobs(c *fiber.Ctx) error {
	jobs, err := h.jobs.ListJobs(c.Context())
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}
	return c.JSON(jobs)
}

func (h *Handler) getJob(c *fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "invalid job id")
	}
	job, err := h.jobs.GetJob(c.Context(), id)
	if err != nil {
		if err == store.ErrNotFound {
			return fiber.NewError(fiber.StatusNotFound, "job not found")
		}
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}
	return c.JSON(job)
}

type createCompanyReq struct {
	Name       string `json:"name"`
	ATS        string `json:"ats"`
	BoardToken string `json:"board_token"`
}

func (h *Handler) createCompany(c *fiber.Ctx) error {
	var req createCompanyReq
	if err := c.BodyParser(&req); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "invalid json")
	}
	req.Name = strings.TrimSpace(req.Name)
	req.ATS = strings.ToLower(strings.TrimSpace(req.ATS))
	req.BoardToken = strings.TrimSpace(req.BoardToken)
	if req.Name == "" || req.BoardToken == "" {
		return fiber.NewError(fiber.StatusBadRequest, "name and board_token are required")
	}
	if req.ATS != "greenhouse" {
		return fiber.NewError(fiber.StatusBadRequest, "ats must be greenhouse")
	}
	co, err := h.companies.CreateCompany(c.Context(), req.Name, req.ATS, req.BoardToken)
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}
	return c.Status(fiber.StatusCreated).JSON(co)
}
