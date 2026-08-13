package service

import (
	"context"
	"encoding/csv"
	"errors"
	"fmt"
	"html"
	"regexp"
	"strings"
	"time"

	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
	"gorm.io/gorm"
)

var (
	ErrStudioArtifactInvalidType = errors.New("invalid studio artifact type")
	ErrStudioArtifactEmptyID     = errors.New("studio artifact id is required")
	ErrStudioArtifactNotFound    = errors.New("studio artifact not found")
	ErrStudioArtifactEmptyIDs    = errors.New("studio artifact ids are required")
)

type studioArtifactService struct {
	repo interfaces.StudioArtifactRepository
	now  func() time.Time
}

func NewStudioArtifactService(repo interfaces.StudioArtifactRepository) interfaces.StudioArtifactService {
	return &studioArtifactService{repo: repo, now: time.Now}
}

func (s *studioArtifactService) List(
	ctx context.Context, req types.StudioArtifactListRequest,
) (*types.StudioArtifactListResponse, error) {
	req.Type = strings.TrimSpace(req.Type)
	req.Query = strings.TrimSpace(req.Query)
	if req.Type != "" && !types.IsValidStudioArtifactType(req.Type) {
		return nil, ErrStudioArtifactInvalidType
	}
	if req.Limit <= 0 || req.Limit > 100 {
		req.Limit = 20
	}
	if req.Offset < 0 {
		req.Offset = 0
	}
	return s.repo.List(ctx, req)
}

func (s *studioArtifactService) Get(
	ctx context.Context, tenantID uint64, userID, id string,
) (*types.StudioArtifact, error) {
	if strings.TrimSpace(id) == "" {
		return nil, ErrStudioArtifactEmptyID
	}
	artifact, err := s.repo.Get(ctx, tenantID, userID, id)
	if err != nil {
		return nil, normalizeStudioArtifactRepoError(err)
	}
	return artifact, nil
}

func (s *studioArtifactService) Create(
	ctx context.Context, tenantID uint64, userID string, req types.StudioArtifactCreateRequest,
) (*types.StudioArtifact, error) {
	return s.create(ctx, tenantID, userID, req, "", 1)
}

func (s *studioArtifactService) Regenerate(
	ctx context.Context, tenantID uint64, userID, id string,
) (*types.StudioArtifact, error) {
	origin, err := s.Get(ctx, tenantID, userID, id)
	if err != nil {
		return nil, err
	}
	rootID := origin.ParentID
	if rootID == "" {
		rootID = origin.ID
	}
	version, err := s.repo.NextVersion(ctx, tenantID, userID, rootID)
	if err != nil {
		return nil, err
	}
	req := types.StudioArtifactCreateRequest{
		Type:      origin.Type,
		Title:     origin.Title,
		Prompt:    origin.Prompt,
		SessionID: origin.SessionID,
		Source:    origin.Source,
	}
	return s.create(ctx, tenantID, userID, req, rootID, version)
}

func (s *studioArtifactService) Delete(ctx context.Context, tenantID uint64, userID, id string) error {
	if strings.TrimSpace(id) == "" {
		return ErrStudioArtifactEmptyID
	}
	deleted, err := s.repo.Delete(ctx, tenantID, userID, id)
	if err != nil {
		return err
	}
	if !deleted {
		return ErrStudioArtifactNotFound
	}
	return nil
}

func (s *studioArtifactService) BatchDelete(
	ctx context.Context, tenantID uint64, userID string, ids []string,
) (int64, error) {
	cleaned := make([]string, 0, len(ids))
	seen := map[string]struct{}{}
	for _, id := range ids {
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		cleaned = append(cleaned, id)
	}
	if len(cleaned) == 0 {
		return 0, ErrStudioArtifactEmptyIDs
	}
	return s.repo.BatchDelete(ctx, tenantID, userID, cleaned)
}

func (s *studioArtifactService) create(
	ctx context.Context,
	tenantID uint64,
	userID string,
	req types.StudioArtifactCreateRequest,
	parentID string,
	version int,
) (*types.StudioArtifact, error) {
	req.Type = strings.TrimSpace(req.Type)
	if !types.IsValidStudioArtifactType(req.Type) {
		return nil, ErrStudioArtifactInvalidType
	}
	title := cleanStudioTitle(req.Title)
	if title == "" {
		title = defaultStudioTitle(req.Type)
	}
	prompt := strings.TrimSpace(req.Prompt)
	content, filename, mimeType := s.render(req.Type, title, prompt, version)
	artifact := &types.StudioArtifact{
		TenantID:  tenantID,
		UserID:    userID,
		SessionID: strings.TrimSpace(req.SessionID),
		ParentID:  parentID,
		Type:      req.Type,
		Title:     title,
		Filename:  filename,
		MimeType:  mimeType,
		Content:   content,
		Size:      int64(len([]byte(content))),
		Prompt:    prompt,
		Source:    normalizeStudioSource(req.Source),
		Version:   version,
		Status:    types.StudioArtifactStatusReady,
		Metadata: types.JSONMap{
			"generator":    "deterministic-template",
			"generated_at": s.now().Format(time.RFC3339),
		},
	}
	if err := s.repo.Create(ctx, artifact); err != nil {
		return nil, err
	}
	return artifact, nil
}

func (s *studioArtifactService) render(artifactType, title, prompt string, version int) (string, string, string) {
	slug := studioFilenameSlug(title)
	switch artifactType {
	case types.StudioArtifactTypeHTML:
		return renderStudioHTML(title, prompt, version), fmt.Sprintf("%s-v%d.html", slug, version), "text/html; charset=utf-8"
	case types.StudioArtifactTypeTable:
		return renderStudioCSV(title, prompt, version), fmt.Sprintf("%s-v%d.csv", slug, version), "text/csv; charset=utf-8"
	case types.StudioArtifactTypeDoc:
		return renderStudioDoc(title, prompt, version), fmt.Sprintf("%s-v%d.md", slug, version), "text/markdown; charset=utf-8"
	default:
		return renderStudioPPT(title, prompt, version), fmt.Sprintf("%s-v%d.md", slug, version), "text/markdown; charset=utf-8"
	}
}

func normalizeStudioArtifactRepoError(err error) error {
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return ErrStudioArtifactNotFound
	}
	return err
}

func normalizeStudioSource(source string) string {
	source = strings.TrimSpace(source)
	if source == "" {
		return "studio"
	}
	if len(source) > 32 {
		return source[:32]
	}
	return source
}

func cleanStudioTitle(title string) string {
	title = strings.TrimSpace(title)
	title = strings.Join(strings.Fields(title), " ")
	if len([]rune(title)) > 80 {
		return string([]rune(title)[:80])
	}
	return title
}

func defaultStudioTitle(artifactType string) string {
	switch artifactType {
	case types.StudioArtifactTypeHTML:
		return "Enterprise Dashboard"
	case types.StudioArtifactTypeTable:
		return "Task Tracking Table"
	case types.StudioArtifactTypeDoc:
		return "Office Document"
	default:
		return "PPT Outline"
	}
}

var nonFilenameChars = regexp.MustCompile(`[^a-zA-Z0-9\p{Han}\-_]+`)

func studioFilenameSlug(title string) string {
	title = strings.TrimSpace(title)
	if title == "" {
		return "studio-artifact"
	}
	title = nonFilenameChars.ReplaceAllString(title, "-")
	title = strings.Trim(title, "-_")
	if title == "" {
		return "studio-artifact"
	}
	runes := []rune(title)
	if len(runes) > 48 {
		return string(runes[:48])
	}
	return title
}

func promptSummary(prompt string) string {
	prompt = strings.TrimSpace(prompt)
	if prompt == "" {
		return "Generated from the current conversation or knowledge base context."
	}
	runes := []rune(prompt)
	if len(runes) > 220 {
		return string(runes[:220]) + "..."
	}
	return prompt
}

func renderStudioPPT(title, prompt string, version int) string {
	summary := promptSummary(prompt)
	return fmt.Sprintf(`# %s

Version: v%d

## 1. Cover

- Title: %s
- Audience: team / reviewers / managers
- Goal: explain the background, solution, value, and next steps in five minutes

## 2. Background and Problem

- What is the business or project context?
- What pain points are we seeing?
- Why does this need to be solved now?

## 3. Goals and Success Criteria

- Define the delivery scope
- Set measurable metrics: time, accuracy, stability, experience
- Define acceptance and fallback plans

## 4. Core Solution

- Option A: process standardization and observability
- Option B: stronger model / parsing / indexing capability
- Option C: Studio workspace for office outputs

## 5. Key Design

- Input: chat context, knowledge base, uploaded files, user notes
- Processing: parse, extract, structure, generate, archive
- Output: PPT outline, HTML page, table, office doc

## 6. Expected Benefit

- Reduce time spent on repeated organization
- Improve traceability of knowledge results
- Make generated outputs downloadable, reusable, and versioned

## 7. Risks and Mitigation

- Large document parsing: staged queues, resume, trace visualization
- Model call failures: health checks, retries, fallback templates
- File management: record archive, batch delete, future tagging/favoriting

## 8. Next Steps

- Add real PPTX/XLSX export
- Connect the generator to the configured LLM
- Add template library, version comparison, team sharing review

## Speaker Notes

%s
`, title, version, title, summary)
}

func renderStudioHTML(title, prompt string, version int) string {
	escapedTitle := html.EscapeString(title)
	escapedSummary := html.EscapeString(promptSummary(prompt))
	return fmt.Sprintf(`<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8" />
  <meta name="viewport" content="width=device-width, initial-scale=1" />
  <title>%s</title>
  <style>
    :root { color-scheme: light; font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif; }
    body { margin: 0; background: #f5f7fb; color: #172033; }
    main { max-width: 1080px; margin: 0 auto; padding: 40px 20px; }
    .hero { padding: 28px; border-radius: 28px; background: linear-gradient(135deg, #1f6feb, #8b5cf6); color: #fff; box-shadow: 0 24px 60px rgba(31, 111, 235, .22); }
    .hero h1 { margin: 0 0 10px; font-size: 32px; }
    .hero p { margin: 0; opacity: .9; line-height: 1.7; }
    .grid { display: grid; grid-template-columns: repeat(auto-fit, minmax(220px, 1fr)); gap: 16px; margin-top: 22px; }
    .card { background: #fff; border: 1px solid #e7ecf5; border-radius: 22px; padding: 20px; box-shadow: 0 14px 34px rgba(15, 23, 42, .06); }
    .metric { font-size: 30px; font-weight: 800; color: #1f6feb; }
    .label { color: #667085; font-size: 13px; }
    .timeline { margin-top: 22px; display: grid; gap: 12px; }
    .step { display: grid; grid-template-columns: 110px 1fr; gap: 14px; align-items: center; }
    .bar { height: 10px; border-radius: 999px; background: #e9effb; overflow: hidden; }
    .bar span { display: block; height: 100%%; border-radius: inherit; background: linear-gradient(90deg, #1f6feb, #22c55e); }
    table { width: 100%%; border-collapse: collapse; margin-top: 12px; }
    th, td { text-align: left; padding: 12px; border-bottom: 1px solid #edf1f7; }
    th { color: #667085; font-size: 13px; }
  </style>
</head>
<body>
  <main>
    <section class="hero">
      <h1>%s</h1>
      <p>%s</p>
    </section>
    <section class="grid">
      <div class="card"><div class="metric">v%d</div><div class="label">Current version</div></div>
      <div class="card"><div class="metric">3</div><div class="label">Core artifact types</div></div>
      <div class="card"><div class="metric">5</div><div class="label">Suggested follow-ups</div></div>
    </section>
    <section class="card timeline">
      <h2>Processing chain</h2>
      <div class="step"><strong>Understand</strong><div class="bar"><span style="width: 96%%"></span></div></div>
      <div class="step"><strong>Organize</strong><div class="bar"><span style="width: 78%%"></span></div></div>
      <div class="step"><strong>Generate</strong><div class="bar"><span style="width: 88%%"></span></div></div>
      <div class="step"><strong>Review</strong><div class="bar"><span style="width: 64%%"></span></div></div>
    </section>
    <section class="card">
      <h2>Action table</h2>
      <table>
        <thead><tr><th>Item</th><th>Owner</th><th>Status</th><th>Advice</th></tr></thead>
        <tbody>
          <tr><td>Fill business context</td><td>Owner</td><td>In progress</td><td>Attach source evidence</td></tr>
          <tr><td>Polish page styling</td><td>Frontend</td><td>Todo</td><td>Align to enterprise design system</td></tr>
          <tr><td>Review conclusions</td><td>Reviewer</td><td>Todo</td><td>Flag risks and sources</td></tr>
        </tbody>
      </table>
    </section>
  </main>
</body>
</html>
`, escapedTitle, escapedTitle, escapedSummary, version)
}

func renderStudioCSV(title, prompt string, version int) string {
	var b strings.Builder
	w := csv.NewWriter(&b)
	rows := [][]string{
		{"No.", "Module", "Task", "Owner", "Priority", "Status", "Notes"},
		{"1", title, "Clarify goals and input materials", "Me", "High", "Todo", promptSummary(prompt)},
		{"2", title, "Generate structured first draft", "Studio", "High", "Done", fmt.Sprintf("Template v%d", version)},
		{"3", title, "Add evidence and references", "Me", "Medium", "Todo", "Link to knowledge base and trace"},
		{"4", title, "Review and export", "Me", "Medium", "Todo", "Keep editable after download"},
		{"5", title, "Turn into reusable template", "Team", "Low", "Planned", "Future template marketplace"},
	}
	_ = w.WriteAll(rows)
	w.Flush()
	return "\ufeff" + b.String()
}

func renderStudioDoc(title, prompt string, version int) string {
	return fmt.Sprintf(`# %s

Version: v%d

## Background

%s

## Key Conclusions

1. The current information is enough to produce a reusable office draft.
2. The next step is to add sources, owners, timestamps, and acceptance criteria.
3. For complex tasks, split work into generate, review, archive, and reuse.

## Action Items

| Item | Priority | Status | Notes |
| --- | --- | --- | --- |
| Fill context | High | Todo | Clarify business target and audience |
| Generate formal draft | High | In progress | Can be downloaded from Studio records |
| Review risks | Medium | Todo | Focus on accuracy and confidentiality |

## Risks and Notes

- Do not include internal code, data, or sensitive information.
- For external portfolios or resumes, use abstracted and de-identified experience.
`, title, version, promptSummary(prompt))
}
