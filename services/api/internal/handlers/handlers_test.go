package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"

	"github.com/as76513/JobSonar/services/api/internal/store"
)

type fake struct {
	jobs      []store.Job
	companies []store.Company
}

func (f *fake) ListJobs(context.Context) ([]store.Job, error) { return f.jobs, nil }

func (f *fake) GetJob(_ context.Context, id uuid.UUID) (store.Job, error) {
	for _, j := range f.jobs {
		if j.ID == id {
			return j, nil
		}
	}
	return store.Job{}, store.ErrNotFound
}

func (f *fake) CreateCompany(_ context.Context, name, ats, token string) (store.Company, error) {
	c := store.Company{
		ID: uuid.New(), Name: name, ATS: ats, BoardToken: token, CreatedAt: time.Now().UTC(),
	}
	f.companies = append(f.companies, c)
	return c, nil
}

func setup(t *testing.T, f *fake) *fiber.App {
	t.Helper()
	app := fiber.New(fiber.Config{ErrorHandler: func(c *fiber.Ctx, err error) error {
		code := fiber.StatusInternalServerError
		if e, ok := err.(*fiber.Error); ok {
			code = e.Code
		}
		return c.Status(code).JSON(fiber.Map{"error": err.Error()})
	}})
	New(f, f).Mount(app)
	return app
}

func TestListAndGetJobs(t *testing.T) {
	id := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	f := &fake{jobs: []store.Job{{
		ID: id, Title: "DevOps Engineer", Company: "Example", Location: "Amsterdam",
		Source: "adzuna", SourceURL: "https://example.test/1", Status: "open",
		Sources: []store.JobSource{
			{Source: "adzuna", SourceURL: "https://example.test/1"},
			{Source: "greenhouse", SourceURL: "https://boards.greenhouse.io/x/jobs/1"},
		},
	}}}
	app := setup(t, f)

	resp, err := app.Test(httptest.NewRequest("GET", "/jobs", nil))
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != 200 {
		t.Fatalf("status=%d", resp.StatusCode)
	}
	body, _ := io.ReadAll(resp.Body)
	var listed []store.Job
	if err := json.Unmarshal(body, &listed); err != nil || len(listed) != 1 {
		t.Fatalf("list=%s err=%v", body, err)
	}
	if len(listed[0].Sources) != 2 {
		t.Fatalf("want 2 sources, got %+v", listed[0].Sources)
	}

	resp, err = app.Test(httptest.NewRequest("GET", "/jobs/"+id.String(), nil))
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != 200 {
		t.Fatalf("get status=%d", resp.StatusCode)
	}

	resp, err = app.Test(httptest.NewRequest("GET", "/jobs/"+uuid.NewString(), nil))
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != 404 {
		t.Fatalf("missing status=%d", resp.StatusCode)
	}
}

func TestCreateCompany(t *testing.T) {
	f := &fake{}
	app := setup(t, f)

	req := httptest.NewRequest("POST", "/companies", bytes.NewBufferString(
		`{"name":"Stripe","ats":"greenhouse","board_token":"stripe"}`))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != 201 {
		t.Fatalf("status=%d", resp.StatusCode)
	}
	if len(f.companies) != 1 || f.companies[0].BoardToken != "stripe" {
		t.Fatalf("%+v", f.companies)
	}

	bad := httptest.NewRequest("POST", "/companies", bytes.NewBufferString(
		`{"name":"X","ats":"lever","board_token":"x"}`))
	bad.Header.Set("Content-Type", "application/json")
	resp, err = app.Test(bad)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != 400 {
		t.Fatalf("lever status=%d", resp.StatusCode)
	}
}
