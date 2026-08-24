package handlers

import (
	"context"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"

	"github.com/as76513/JobSonar/services/api/internal/score"
	"github.com/as76513/JobSonar/services/api/internal/store"
)

const maxResumeBytes = 5 << 20

type Jobs interface {
	ListJobs(ctx context.Context) ([]store.Job, error)
	GetJob(ctx context.Context, id uuid.UUID) (store.Job, error)
}

type Companies interface {
	CreateCompany(ctx context.Context, name, ats, token string) (store.Company, error)
}

type Profiles interface {
	GetProfile(ctx context.Context) (store.Profile, error)
	UpsertProfile(ctx context.Context, skills []string) (store.Profile, error)
}

type Applications interface {
	ListApplications(ctx context.Context) ([]store.Application, error)
	CreateApplication(ctx context.Context, jobID uuid.UUID) (store.Application, error)
	UpdateApplicationStatus(ctx context.Context, id uuid.UUID, status string) (store.Application, error)
}

type Resumes interface {
	CreateResume(ctx context.Context, storageURI string) (store.Resume, error)
	LatestResume(ctx context.Context) (store.Resume, error)
}

type Handler struct {
	jobs         Jobs
	companies    Companies
	profiles     Profiles
	applications Applications
	resumes      Resumes
	resumeDir    string
}

func New(jobs Jobs, companies Companies, profiles Profiles, applications Applications, resumes Resumes, resumeDir string) *Handler {
	return &Handler{
		jobs: jobs, companies: companies, profiles: profiles,
		applications: applications, resumes: resumes, resumeDir: resumeDir,
	}
}

func (h *Handler) Mount(app *fiber.App) {
	app.Get("/jobs", h.listJobs)
	app.Get("/jobs/:id", h.getJob)
	app.Post("/companies", h.createCompany)
	app.Get("/profile", h.getProfile)
	app.Put("/profile", h.putProfile)
	app.Post("/profile/resume", h.uploadResume)
	app.Get("/applications", h.listApplications)
	app.Post("/applications", h.createApplication)
	app.Patch("/applications/:id", h.patchApplication)
}

type scoredJob struct {
	store.Job
	Score score.Keyword `json:"score"`
}

func (h *Handler) skills(c *fiber.Ctx) []string {
	if h.profiles == nil {
		return nil
	}
	p, err := h.profiles.GetProfile(c.Context())
	if err != nil {
		return nil
	}
	return p.Skills
}

func decorate(j store.Job, skills []string, includeDesc bool) scoredJob {
	s := score.Overlap(skills, j.Title, j.DescriptionMD)
	s.Semantic = j.Semantic
	if !includeDesc {
		j.DescriptionMD = ""
	}
	return scoredJob{Job: j, Score: s}
}

func semOr(s *float64) float64 {
	if s == nil {
		return -1
	}
	return *s
}

func (h *Handler) listJobs(c *fiber.Ctx) error {
	jobs, err := h.jobs.ListJobs(c.Context())
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}
	skills := h.skills(c)
	out := make([]scoredJob, 0, len(jobs))
	useSemantic := false
	for _, j := range jobs {
		row := decorate(j, skills, false)
		if row.Score.Semantic != nil {
			useSemantic = true
		}
		out = append(out, row)
	}
	sort.SliceStable(out, func(i, k int) bool {
		if out[i].Score.Coverage != out[k].Score.Coverage {
			return out[i].Score.Coverage > out[k].Score.Coverage
		}
		if useSemantic {
			si, sk := semOr(out[i].Score.Semantic), semOr(out[k].Score.Semantic)
			if si != sk {
				return si > sk
			}
		}
		return out[i].LastSeenAt.After(out[k].LastSeenAt)
	})
	return c.JSON(out)
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
	return c.JSON(decorate(job, h.skills(c), true))
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

func (h *Handler) getProfile(c *fiber.Ctx) error {
	p, err := h.profiles.GetProfile(c.Context())
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}
	if h.resumes != nil {
		if r, err := h.resumes.LatestResume(c.Context()); err == nil {
			p.LatestResume = &r
		}
	}
	return c.JSON(p)
}

func (h *Handler) uploadResume(c *fiber.Ctx) error {
	if h.resumes == nil {
		return fiber.NewError(fiber.StatusNotImplemented, "resume upload not configured")
	}
	file, err := c.FormFile("file")
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "file is required")
	}
	if file.Size == 0 || file.Size > maxResumeBytes {
		return fiber.NewError(fiber.StatusBadRequest, "file must be between 1 byte and 5MB")
	}
	ext := strings.ToLower(filepath.Ext(file.Filename))
	if ext != ".pdf" && ext != ".docx" {
		return fiber.NewError(fiber.StatusBadRequest, "file must be a PDF or DOCX")
	}
	dir := h.resumeDir
	if dir == "" {
		dir = "data/resumes"
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, "could not store resume")
	}
	id := uuid.New()
	dest, err := filepath.Abs(filepath.Join(dir, id.String()+ext))
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, "could not store resume")
	}
	if err := c.SaveFile(file, dest); err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, "could not store resume")
	}
	row, err := h.resumes.CreateResume(c.Context(), dest)
	if err != nil {
		_ = os.Remove(dest)
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}
	return c.Status(fiber.StatusAccepted).JSON(row)
}

type putProfileReq struct {
	Skills []string `json:"skills"`
}

func (h *Handler) putProfile(c *fiber.Ctx) error {
	var req putProfileReq
	if err := c.BodyParser(&req); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "invalid json")
	}
	clean := make([]string, 0, len(req.Skills))
	seen := map[string]struct{}{}
	for _, s := range req.Skills {
		s = strings.TrimSpace(s)
		if s == "" {
			continue
		}
		key := strings.ToLower(s)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		clean = append(clean, s)
	}
	p, err := h.profiles.UpsertProfile(c.Context(), clean)
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}
	return c.JSON(p)
}

func (h *Handler) listApplications(c *fiber.Ctx) error {
	apps, err := h.applications.ListApplications(c.Context())
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}
	return c.JSON(apps)
}

type createAppReq struct {
	JobID string `json:"job_id"`
}

func (h *Handler) createApplication(c *fiber.Ctx) error {
	var req createAppReq
	if err := c.BodyParser(&req); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "invalid json")
	}
	jobID, err := uuid.Parse(strings.TrimSpace(req.JobID))
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "invalid job_id")
	}
	app, err := h.applications.CreateApplication(c.Context(), jobID)
	if err != nil {
		if err == store.ErrNotFound {
			return fiber.NewError(fiber.StatusNotFound, "job not found")
		}
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}
	return c.Status(fiber.StatusCreated).JSON(app)
}

type patchAppReq struct {
	Status string `json:"status"`
}

func (h *Handler) patchApplication(c *fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "invalid application id")
	}
	var req patchAppReq
	if err := c.BodyParser(&req); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "invalid json")
	}
	req.Status = strings.ToLower(strings.TrimSpace(req.Status))
	if !store.ValidStatus(req.Status) {
		return fiber.NewError(fiber.StatusBadRequest, "status must be saved, applied, screen, interview, offer, or closed")
	}
	app, err := h.applications.UpdateApplicationStatus(c.Context(), id, req.Status)
	if err != nil {
		if err == store.ErrNotFound {
			return fiber.NewError(fiber.StatusNotFound, "application not found")
		}
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}
	return c.JSON(app)
}
