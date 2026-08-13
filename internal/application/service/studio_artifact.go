package service

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/base64"
	"encoding/csv"
	"errors"
	"fmt"
	"html"
	"math"
	"regexp"
	"strings"
	"time"

	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
	"github.com/xuri/excelize/v2"
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

type renderedStudioArtifact struct {
	content  string
	filename string
	mimeType string
	size     int64
	metadata types.JSONMap
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
	rendered := s.render(req.Type, title, prompt, version)
	if rendered.metadata == nil {
		rendered.metadata = types.JSONMap{}
	}
	rendered.metadata["generator"] = "deterministic-office-template"
	rendered.metadata["generated_at"] = s.now().Format(time.RFC3339)
	artifact := &types.StudioArtifact{
		TenantID:  tenantID,
		UserID:    userID,
		SessionID: strings.TrimSpace(req.SessionID),
		ParentID:  parentID,
		Type:      req.Type,
		Title:     title,
		Filename:  rendered.filename,
		MimeType:  rendered.mimeType,
		Content:   rendered.content,
		Size:      rendered.size,
		Prompt:    prompt,
		Source:    normalizeStudioSource(req.Source),
		Version:   version,
		Status:    types.StudioArtifactStatusReady,
		Metadata:  rendered.metadata,
	}
	if err := s.repo.Create(ctx, artifact); err != nil {
		return nil, err
	}
	return artifact, nil
}

func (s *studioArtifactService) render(artifactType, title, prompt string, version int) renderedStudioArtifact {
	slug := studioFilenameSlug(title)
	switch artifactType {
	case types.StudioArtifactTypeHTML:
		content := renderStudioHTML(title, prompt, version)
		return renderedTextArtifact(content, fmt.Sprintf("%s-v%d.html", slug, version), "text/html; charset=utf-8")
	case types.StudioArtifactTypeTable:
		data, err := renderStudioXLSX(title, prompt, version)
		if err == nil {
			return renderedBinaryArtifact(data, fmt.Sprintf("%s-v%d.xlsx", slug, version), "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
		}
		content := renderStudioCSV(title, prompt, version)
		return renderedTextArtifact(content, fmt.Sprintf("%s-v%d.csv", slug, version), "text/csv; charset=utf-8")
	case types.StudioArtifactTypeDoc:
		content := renderStudioDoc(title, prompt, version)
		return renderedTextArtifact(content, fmt.Sprintf("%s-v%d.md", slug, version), "text/markdown; charset=utf-8")
	default:
		data, err := renderStudioPPTX(title, prompt, version)
		if err == nil {
			return renderedBinaryArtifact(data, fmt.Sprintf("%s-v%d.pptx", slug, version), "application/vnd.openxmlformats-officedocument.presentationml.presentation")
		}
		content := renderStudioPPT(title, prompt, version)
		return renderedTextArtifact(content, fmt.Sprintf("%s-v%d.md", slug, version), "text/markdown; charset=utf-8")
	}
}

func renderedTextArtifact(content, filename, mimeType string) renderedStudioArtifact {
	return renderedStudioArtifact{
		content:  content,
		filename: filename,
		mimeType: mimeType,
		size:     int64(len([]byte(content))),
		metadata: types.JSONMap{"content_encoding": "text"},
	}
}

func renderedBinaryArtifact(data []byte, filename, mimeType string) renderedStudioArtifact {
	return renderedStudioArtifact{
		content:  base64.StdEncoding.EncodeToString(data),
		filename: filename,
		mimeType: mimeType,
		size:     int64(len(data)),
		metadata: types.JSONMap{"content_encoding": "base64"},
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

type studioSlide struct {
	title    string
	subtitle string
	bullets  []string
	accent   string
	layout   string
}

func renderStudioPPTX(title, prompt string, version int) ([]byte, error) {
	summary := promptSummary(prompt)
	slides := buildStudioSlides(title, summary, version)
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	files := map[string]string{
		"[Content_Types].xml":                          pptxContentTypes(len(slides)),
		"_rels/.rels":                                  pptxRootRels(),
		"docProps/app.xml":                             pptxAppProps(len(slides)),
		"docProps/core.xml":                            pptxCoreProps(title),
		"ppt/presentation.xml":                         pptxPresentationXML(len(slides)),
		"ppt/_rels/presentation.xml.rels":              pptxPresentationRels(len(slides)),
		"ppt/slideMasters/slideMaster1.xml":            pptxSlideMaster(),
		"ppt/slideMasters/_rels/slideMaster1.xml.rels": pptxSlideMasterRels(),
		"ppt/slideLayouts/slideLayout1.xml":            pptxSlideLayout(),
		"ppt/theme/theme1.xml":                         pptxTheme(),
	}
	for i, slide := range slides {
		files[fmt.Sprintf("ppt/slides/slide%d.xml", i+1)] = pptxSlideXML(i+1, slide)
		files[fmt.Sprintf("ppt/slides/_rels/slide%d.xml.rels", i+1)] = pptxSlideRels()
	}
	for name, content := range files {
		w, err := zw.Create(name)
		if err != nil {
			_ = zw.Close()
			return nil, err
		}
		if _, err := w.Write([]byte(content)); err != nil {
			_ = zw.Close()
			return nil, err
		}
	}
	if err := zw.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func buildStudioSlides(title, summary string, version int) []studioSlide {
	contextPoints := studioPromptPoints(summary, []string{
		"Clarify the business background, audience, and expected decision.",
		"Extract key evidence from the conversation or knowledge base.",
		"Turn loose notes into an editable office deliverable.",
	})
	return []studioSlide{
		{
			title:    title,
			subtitle: fmt.Sprintf("Executive-ready office draft · v%d", version),
			bullets: []string{
				"Built for project reporting, review meetings, and daily office delivery.",
				"Generated as a real editable PPTX file, ready for PowerPoint or WPS.",
				contextPoints[0],
			},
			accent: "2563EB",
			layout: "cover",
		},
		{
			title:    "Executive Summary",
			subtitle: "What this artifact is trying to make clear",
			bullets:  contextPoints,
			accent:   "7C3AED",
			layout:   "summary",
		},
		{
			title:    "Goals & Acceptance",
			subtitle: "Make the work measurable before it becomes busywork",
			bullets: []string{
				"Scope: inputs, processing chain, output format, owners, and handoff rules.",
				"Metrics: latency, accuracy, stability, usability, and review cost.",
				"Fallback: retry, graceful degradation, manual review, and version rollback.",
			},
			accent: "0891B2",
			layout: "cards",
		},
		{
			title:    "Core Solution",
			subtitle: "A practical path from knowledge input to office output",
			bullets: []string{
				"Standardize the workflow: configure, generate, preview, download, archive.",
				"Improve observability: queue radar, task SLA, and per-file trace view.",
				"Use Studio as the office output layer: PPT, HTML, spreadsheet, and document.",
			},
			accent: "16A34A",
			layout: "flow",
		},
		{
			title:    "Processing Blueprint",
			subtitle: "Separate input, generation, review, and reuse",
			bullets: []string{
				"Input: chat context, knowledge base, uploaded references, and template hints.",
				"Process: parse, structure, generate, validate, and version every artifact.",
				"Output: downloadable files, previewable pages, and reusable generation records.",
			},
			accent: "EA580C",
			layout: "timeline",
		},
		{
			title:    "Risks & Governance",
			subtitle: "Design the happy path and the failure path together",
			bullets: []string{
				"Large documents: chunking, async queue, checkpoint resume, and trace diagnosis.",
				"Model failures: health checks, rate limiting, retries, and fallback models.",
				"File governance: records, versions, batch deletion, sharing, and audit trail.",
			},
			accent: "DC2626",
			layout: "risk",
		},
		{
			title:    "Next Roadmap",
			subtitle: "From usable demo to enterprise-grade Studio",
			bullets: []string{
				"Connect Studio generation to the configured LLM for content-aware drafts.",
				"Add template library, corporate themes, team sharing, and approval flow.",
				"Continue hardening queues, SLA alerts, trace observability, and audit closure.",
			},
			accent: "4F46E5",
			layout: "roadmap",
		},
	}
}

func studioPromptPoints(summary string, fallback []string) []string {
	candidates := strings.FieldsFunc(summary, func(r rune) bool {
		return r == '\n' || r == '。' || r == '；' || r == ';' || r == '.' || r == '!' || r == '！' || r == '?' || r == '？'
	})
	points := make([]string, 0, 3)
	for _, candidate := range candidates {
		candidate = strings.TrimSpace(candidate)
		if candidate == "" || candidate == "Generated from the current conversation or knowledge base context" {
			continue
		}
		runes := []rune(candidate)
		if len(runes) > 88 {
			candidate = string(runes[:88]) + "..."
		}
		points = append(points, candidate)
		if len(points) == 3 {
			return points
		}
	}
	for _, item := range fallback {
		if len(points) == 3 {
			break
		}
		points = append(points, item)
	}
	return points
}

func renderStudioXLSX(title, prompt string, version int) ([]byte, error) {
	f := excelize.NewFile()
	defer f.Close()
	sheet := "Studio"
	f.SetSheetName("Sheet1", sheet)
	rows := [][]any{
		{"序号", "模块", "任务", "负责人", "优先级", "状态", "说明"},
		{1, title, "Clarify goals and input materials", "Me", "High", "Todo", promptSummary(prompt)},
		{2, title, "Generate structured first draft", "Studio", "High", "Done", fmt.Sprintf("Template v%d", version)},
		{3, title, "Add evidence and references", "Me", "Medium", "Todo", "Link to knowledge base and trace"},
		{4, title, "Review and export", "Me", "Medium", "Todo", "Keep editable after download"},
		{5, title, "Turn into reusable template", "Team", "Low", "Planned", "Future template marketplace"},
	}
	for r, row := range rows {
		for c, cell := range row {
			name, _ := excelize.CoordinatesToCellName(c+1, r+1)
			_ = f.SetCellValue(sheet, name, cell)
		}
	}
	headerStyle, _ := f.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Bold: true, Color: "FFFFFF"},
		Fill:      excelize.Fill{Type: "pattern", Color: []string{"2563EB"}, Pattern: 1},
		Alignment: &excelize.Alignment{Horizontal: "center"},
	})
	_ = f.SetCellStyle(sheet, "A1", "G1", headerStyle)
	_ = f.SetColWidth(sheet, "A", "A", 8)
	_ = f.SetColWidth(sheet, "B", "D", 22)
	_ = f.SetColWidth(sheet, "E", "F", 12)
	_ = f.SetColWidth(sheet, "G", "G", 46)
	var buf bytes.Buffer
	if err := f.Write(&buf); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func pptxContentTypes(slideCount int) string {
	var slideOverrides strings.Builder
	for i := 1; i <= slideCount; i++ {
		slideOverrides.WriteString(fmt.Sprintf(
			`  <Override PartName="/ppt/slides/slide%d.xml" ContentType="application/vnd.openxmlformats-officedocument.presentationml.slide+xml"/>`,
			i,
		))
		slideOverrides.WriteByte('\n')
	}
	return `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">
  <Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>
  <Default Extension="xml" ContentType="application/xml"/>
  <Override PartName="/docProps/app.xml" ContentType="application/vnd.openxmlformats-officedocument.extended-properties+xml"/>
  <Override PartName="/docProps/core.xml" ContentType="application/vnd.openxmlformats-package.core-properties+xml"/>
  <Override PartName="/ppt/presentation.xml" ContentType="application/vnd.openxmlformats-officedocument.presentationml.presentation.main+xml"/>
  <Override PartName="/ppt/slideMasters/slideMaster1.xml" ContentType="application/vnd.openxmlformats-officedocument.presentationml.slideMaster+xml"/>
  <Override PartName="/ppt/slideLayouts/slideLayout1.xml" ContentType="application/vnd.openxmlformats-officedocument.presentationml.slideLayout+xml"/>
  <Override PartName="/ppt/theme/theme1.xml" ContentType="application/vnd.openxmlformats-officedocument.theme+xml"/>
` + slideOverrides.String() + `
</Types>`
}

func pptxRootRels() string {
	return `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="ppt/presentation.xml"/>
  <Relationship Id="rId2" Type="http://schemas.openxmlformats.org/package/2006/relationships/metadata/core-properties" Target="docProps/core.xml"/>
  <Relationship Id="rId3" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/extended-properties" Target="docProps/app.xml"/>
</Relationships>`
}

func pptxAppProps(slideCount int) string {
	return fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Properties xmlns="http://schemas.openxmlformats.org/officeDocument/2006/extended-properties" xmlns:vt="http://schemas.openxmlformats.org/officeDocument/2006/docPropsVTypes">
  <Application>EggKid Studio</Application>
  <PresentationFormat>On-screen Show (16:9)</PresentationFormat>
  <Slides>%d</Slides>
  <Company>EggKidNotebook</Company>
</Properties>`, slideCount)
}

func pptxCoreProps(title string) string {
	now := time.Now().UTC().Format(time.RFC3339)
	return fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<cp:coreProperties xmlns:cp="http://schemas.openxmlformats.org/package/2006/metadata/core-properties" xmlns:dc="http://purl.org/dc/elements/1.1/" xmlns:dcterms="http://purl.org/dc/terms/" xmlns:dcmitype="http://purl.org/dc/dcmitype/" xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance">
  <dc:title>%s</dc:title>
  <dc:creator>EggKid Studio</dc:creator>
  <cp:lastModifiedBy>EggKid Studio</cp:lastModifiedBy>
  <dcterms:created xsi:type="dcterms:W3CDTF">%s</dcterms:created>
  <dcterms:modified xsi:type="dcterms:W3CDTF">%s</dcterms:modified>
</cp:coreProperties>`, pptxEscape(title), now, now)
}

func pptxPresentationXML(slideCount int) string {
	var slides strings.Builder
	for i := 1; i <= slideCount; i++ {
		slides.WriteString(fmt.Sprintf(`<p:sldId id="%d" r:id="rId%d"/>`, 255+i, i+1))
	}
	return fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<p:presentation xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships" xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main">
  <p:sldMasterIdLst><p:sldMasterId id="2147483648" r:id="rId1"/></p:sldMasterIdLst>
  <p:sldIdLst>%s</p:sldIdLst>
  <p:sldSz cx="12192000" cy="6858000" type="screen16x9"/>
  <p:notesSz cx="6858000" cy="9144000"/>
</p:presentation>`, slides.String())
}

func pptxPresentationRels(slideCount int) string {
	var rels strings.Builder
	rels.WriteString(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?><Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">`)
	rels.WriteString(`<Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/slideMaster" Target="slideMasters/slideMaster1.xml"/>`)
	for i := 1; i <= slideCount; i++ {
		rels.WriteString(fmt.Sprintf(`<Relationship Id="rId%d" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/slide" Target="slides/slide%d.xml"/>`, i+1, i))
	}
	rels.WriteString(`</Relationships>`)
	return rels.String()
}

func pptxSlideMasterRels() string {
	return `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/slideLayout" Target="../slideLayouts/slideLayout1.xml"/>
  <Relationship Id="rId2" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/theme" Target="../theme/theme1.xml"/>
</Relationships>`
}

func pptxSlideRels() string {
	return `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/slideLayout" Target="../slideLayouts/slideLayout1.xml"/>
</Relationships>`
}

func pptxSlideMaster() string {
	return `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<p:sldMaster xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships" xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main">
  <p:cSld><p:bg><p:bgPr><a:solidFill><a:srgbClr val="FFFFFF"/></a:solidFill></p:bgPr></p:bg><p:spTree><p:nvGrpSpPr><p:cNvPr id="1" name=""/><p:cNvGrpSpPr/><p:nvPr/></p:nvGrpSpPr><p:grpSpPr><a:xfrm><a:off x="0" y="0"/><a:ext cx="0" cy="0"/><a:chOff x="0" y="0"/><a:chExt cx="0" cy="0"/></a:xfrm></p:grpSpPr></p:spTree></p:cSld>
  <p:sldLayoutIdLst><p:sldLayoutId id="2147483649" r:id="rId1"/></p:sldLayoutIdLst>
  <p:txStyles><p:titleStyle/><p:bodyStyle/><p:otherStyle/></p:txStyles>
</p:sldMaster>`
}

func pptxSlideLayout() string {
	return `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<p:sldLayout xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships" xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main" type="blank" preserve="1">
  <p:cSld name="Blank"><p:spTree><p:nvGrpSpPr><p:cNvPr id="1" name=""/><p:cNvGrpSpPr/><p:nvPr/></p:nvGrpSpPr><p:grpSpPr><a:xfrm><a:off x="0" y="0"/><a:ext cx="0" cy="0"/><a:chOff x="0" y="0"/><a:chExt cx="0" cy="0"/></a:xfrm></p:grpSpPr></p:spTree></p:cSld>
</p:sldLayout>`
}

func pptxTheme() string {
	return `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<a:theme xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" name="EggKid Studio">
  <a:themeElements>
    <a:clrScheme name="EggKid"><a:dk1><a:srgbClr val="111827"/></a:dk1><a:lt1><a:srgbClr val="FFFFFF"/></a:lt1><a:dk2><a:srgbClr val="1F2937"/></a:dk2><a:lt2><a:srgbClr val="F8FAFC"/></a:lt2><a:accent1><a:srgbClr val="2563EB"/></a:accent1><a:accent2><a:srgbClr val="7C3AED"/></a:accent2><a:accent3><a:srgbClr val="16A34A"/></a:accent3><a:accent4><a:srgbClr val="EA580C"/></a:accent4><a:accent5><a:srgbClr val="DC2626"/></a:accent5><a:accent6><a:srgbClr val="0891B2"/></a:accent6><a:hlink><a:srgbClr val="2563EB"/></a:hlink><a:folHlink><a:srgbClr val="7C3AED"/></a:folHlink></a:clrScheme>
    <a:fontScheme name="EggKid"><a:majorFont><a:latin typeface="Microsoft YaHei"/></a:majorFont><a:minorFont><a:latin typeface="Microsoft YaHei"/></a:minorFont></a:fontScheme>
    <a:fmtScheme name="EggKid"><a:fillStyleLst><a:solidFill><a:schemeClr val="phClr"/></a:solidFill></a:fillStyleLst><a:lnStyleLst><a:ln w="9525"><a:solidFill><a:schemeClr val="phClr"/></a:solidFill></a:ln></a:lnStyleLst><a:effectStyleLst><a:effectStyle><a:effectLst/></a:effectStyle></a:effectStyleLst><a:bgFillStyleLst><a:solidFill><a:schemeClr val="phClr"/></a:solidFill></a:bgFillStyleLst></a:fmtScheme>
  </a:themeElements>
</a:theme>`
}

func pptxSlideXML(index int, slide studioSlide) string {
	accent := slide.accent
	if accent == "" {
		accent = "2563EB"
	}
	if slide.layout == "cover" {
		return pptxCoverSlideXML(index, slide, accent)
	}
	title := pptxEscape(slide.title)
	subtitle := pptxEscape(slide.subtitle)
	if subtitle == "" {
		subtitle = fmt.Sprintf("EggKid Studio · Slide %d", index)
	}
	var bulletXML strings.Builder
	for i, bullet := range slide.bullets {
		if strings.TrimSpace(bullet) == "" {
			continue
		}
		y := 2100000 + i*620000
		size := 255000
		if len([]rune(bullet)) > 90 {
			size = 215000
		}
		bulletXML.WriteString(pptxTextBox(
			20+i,
			fmt.Sprintf("Bullet %d", i+1),
			1060000,
			y,
			9600000,
			520000,
			pptxEscape("• "+bullet),
			"1F2937",
			size,
			false,
		))
	}
	progress := int(math.Min(100, 58+float64(index)*6))
	return fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<p:sld xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships" xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main">
  <p:cSld>
    <p:bg><p:bgPr><a:solidFill><a:srgbClr val="F8FAFC"/></a:solidFill></p:bgPr></p:bg>
    <p:spTree>
      <p:nvGrpSpPr><p:cNvPr id="1" name=""/><p:cNvGrpSpPr/><p:nvPr/></p:nvGrpSpPr>
      <p:grpSpPr><a:xfrm><a:off x="0" y="0"/><a:ext cx="0" cy="0"/><a:chOff x="0" y="0"/><a:chExt cx="0" cy="0"/></a:xfrm></p:grpSpPr>
      %s
      %s
      %s
      %s
      %s
      %s
    </p:spTree>
  </p:cSld>
  <p:clrMapOvr><a:masterClrMapping/></p:clrMapOvr>
</p:sld>`,
		pptxRect(2, "Accent", 0, 0, 12192000, 280000, accent),
		pptxRect(3, "Canvas", 520000, 720000, 11160000, 5480000, "FFFFFF"),
		pptxRect(4, "Left Rail", 520000, 720000, 140000, 5480000, accent),
		pptxTextBox(5, "Title", 960000, 980000, 7600000, 650000, title, "111827", 410000, true),
		pptxTextBox(6, "Subtitle", 960000, 1580000, 8100000, 360000, subtitle, accent, 175000, false),
		pptxSlideVisuals(index, slide, accent)+bulletXML.String()+pptxFooter(index, progress, accent),
	)
}

func pptxCoverSlideXML(index int, slide studioSlide, accent string) string {
	title := pptxEscape(slide.title)
	subtitle := pptxEscape(slide.subtitle)
	if subtitle == "" {
		subtitle = "EggKid Studio · Executive-ready office output"
	}
	var detail strings.Builder
	for i, bullet := range slide.bullets {
		if strings.TrimSpace(bullet) == "" {
			continue
		}
		detail.WriteString(pptxTextBox(30+i, fmt.Sprintf("Cover Point %d", i+1), 980000, 4660000+i*360000, 8500000, 300000, pptxEscape("— "+bullet), "CBD5E1", 150000, false))
	}
	return fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<p:sld xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships" xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main">
  <p:cSld>
    <p:bg><p:bgPr><a:solidFill><a:srgbClr val="0F172A"/></a:solidFill></p:bgPr></p:bg>
    <p:spTree>
      <p:nvGrpSpPr><p:cNvPr id="1" name=""/><p:cNvGrpSpPr/><p:nvPr/></p:nvGrpSpPr>
      <p:grpSpPr><a:xfrm><a:off x="0" y="0"/><a:ext cx="0" cy="0"/><a:chOff x="0" y="0"/><a:chExt cx="0" cy="0"/></a:xfrm></p:grpSpPr>
      %s
      %s
      %s
      %s
      %s
      %s
      %s
    </p:spTree>
  </p:cSld>
  <p:clrMapOvr><a:masterClrMapping/></p:clrMapOvr>
</p:sld>`,
		pptxRect(2, "Accent Bar", 0, 0, 12192000, 360000, accent),
		pptxRect(3, "Hero Glow 1", 7420000, 900000, 3700000, 3700000, "1D4ED8"),
		pptxRect(4, "Hero Glow 2", 8500000, 2060000, 2800000, 2800000, "7C3AED"),
		pptxRect(5, "Title Underline", 980000, 3680000, 2900000, 90000, accent),
		pptxTextBox(6, "Studio Label", 980000, 880000, 5200000, 320000, "EGGKID STUDIO · ENTERPRISE COPILOT", "93C5FD", 145000, true),
		pptxTextBox(7, "Cover Title", 980000, 1540000, 7400000, 1900000, title, "FFFFFF", 540000, true),
		pptxTextBox(8, "Cover Subtitle", 980000, 3820000, 7800000, 460000, subtitle, "BFDBFE", 205000, false)+detail.String()+pptxFooter(index, 64, accent),
	)
}

func pptxSlideVisuals(index int, slide studioSlide, accent string) string {
	switch slide.layout {
	case "summary", "cards":
		return pptxCardRow(50, 890000, 4200000, accent, []string{"Context", "Evidence", "Decision"})
	case "flow":
		return pptxFlowRow(50, accent, []string{"Configure", "Generate", "Preview", "Archive"})
	case "timeline", "roadmap":
		return pptxRoadmap(50, accent, []string{"Now", "Next", "Scale"})
	case "risk":
		return pptxRiskMatrix(50, accent)
	default:
		return pptxTextBox(50, "Slide Badge", 9100000, 980000, 1600000, 300000, fmt.Sprintf("0%d", index), accent, 180000, true)
	}
}

func pptxCardRow(baseID, x, y int, accent string, labels []string) string {
	var out strings.Builder
	for i, label := range labels {
		cardX := x + i*3360000
		out.WriteString(pptxRect(baseID+i*3, label+" Card", cardX, y, 3000000, 980000, "F8FAFC"))
		out.WriteString(pptxRect(baseID+i*3+1, label+" Accent", cardX, y, 3000000, 90000, accent))
		out.WriteString(pptxTextBox(baseID+i*3+2, label+" Label", cardX+220000, y+230000, 2500000, 360000, label, "0F172A", 210000, true))
	}
	return out.String()
}

func pptxFlowRow(baseID int, accent string, labels []string) string {
	var out strings.Builder
	for i, label := range labels {
		x := 920000 + i*2500000
		out.WriteString(pptxRect(baseID+i*4, label+" Node", x, 4140000, 2050000, 860000, "EFF6FF"))
		out.WriteString(pptxTextBox(baseID+i*4+1, label+" Number", x+180000, 4300000, 420000, 280000, fmt.Sprintf("%02d", i+1), accent, 170000, true))
		out.WriteString(pptxTextBox(baseID+i*4+2, label+" Label", x+660000, 4290000, 1200000, 300000, label, "0F172A", 175000, true))
		if i < len(labels)-1 {
			out.WriteString(pptxRect(baseID+i*4+3, label+" Connector", x+2050000, 4510000, 450000, 70000, accent))
		}
	}
	return out.String()
}

func pptxRoadmap(baseID int, accent string, labels []string) string {
	var out strings.Builder
	out.WriteString(pptxRect(baseID, "Roadmap Line", 1260000, 4580000, 8700000, 80000, "CBD5E1"))
	for i, label := range labels {
		x := 1460000 + i*3900000
		out.WriteString(pptxRect(baseID+i*4+1, label+" Marker", x, 4330000, 580000, 580000, accent))
		out.WriteString(pptxTextBox(baseID+i*4+2, label+" Label", x-260000, 5100000, 1150000, 300000, label, "0F172A", 180000, true))
	}
	return out.String()
}

func pptxRiskMatrix(baseID int, accent string) string {
	return pptxRect(baseID, "Risk Matrix", 6500000, 2260000, 4000000, 2700000, "FEF2F2") +
		pptxRect(baseID+1, "Risk Matrix Accent", 6500000, 2260000, 4000000, 100000, accent) +
		pptxTextBox(baseID+2, "Risk High", 6820000, 2660000, 3300000, 360000, "High impact · needs fallback", "991B1B", 180000, true) +
		pptxTextBox(baseID+3, "Risk Medium", 6820000, 3380000, 3300000, 360000, "Medium impact · monitor SLA", "B45309", 180000, true) +
		pptxTextBox(baseID+4, "Risk Low", 6820000, 4100000, 3300000, 360000, "Low impact · document owner", "166534", 180000, true)
}

func pptxFooter(index, progress int, accent string) string {
	return pptxTextBox(80, "Footer", 1060000, 6100000, 4600000, 260000, fmt.Sprintf("Slide %d · Stability-first office output", index), "64748B", 135000, false) +
		pptxRect(81, "Progress Track", 7200000, 6200000, 3200000, 80000, "E2E8F0") +
		pptxRect(82, "Progress", 7200000, 6200000, 32000*progress, 80000, accent)
}

func pptxRect(id int, name string, x, y, cx, cy int, color string) string {
	return fmt.Sprintf(`<p:sp><p:nvSpPr><p:cNvPr id="%d" name="%s"/><p:cNvSpPr/><p:nvPr/></p:nvSpPr><p:spPr><a:xfrm><a:off x="%d" y="%d"/><a:ext cx="%d" cy="%d"/></a:xfrm><a:prstGeom prst="rect"><a:avLst/></a:prstGeom><a:solidFill><a:srgbClr val="%s"/></a:solidFill><a:ln><a:noFill/></a:ln></p:spPr><p:txBody><a:bodyPr/><a:lstStyle/><a:p/></p:txBody></p:sp>`,
		id, pptxEscape(name), x, y, cx, cy, color)
}

func pptxTextBox(id int, name string, x, y, cx, cy int, text, color string, fontSize int, bold bool) string {
	boldAttr := ""
	if bold {
		boldAttr = ` b="1"`
	}
	return fmt.Sprintf(`<p:sp><p:nvSpPr><p:cNvPr id="%d" name="%s"/><p:cNvSpPr txBox="1"/><p:nvPr/></p:nvSpPr><p:spPr><a:xfrm><a:off x="%d" y="%d"/><a:ext cx="%d" cy="%d"/></a:xfrm><a:prstGeom prst="rect"><a:avLst/></a:prstGeom><a:noFill/><a:ln><a:noFill/></a:ln></p:spPr><p:txBody><a:bodyPr wrap="square" anchor="t"/><a:lstStyle/><a:p><a:r><a:rPr lang="zh-CN" sz="%d"%s><a:solidFill><a:srgbClr val="%s"/></a:solidFill><a:latin typeface="Microsoft YaHei"/><a:ea typeface="Microsoft YaHei"/></a:rPr><a:t>%s</a:t></a:r></a:p></p:txBody></p:sp>`,
		id, pptxEscape(name), x, y, cx, cy, fontSize/100, boldAttr, color, text)
}

func pptxEscape(value string) string {
	return html.EscapeString(strings.TrimSpace(value))
}

func renderStudioHTML(title, prompt string, version int) string {
	escapedTitle := html.EscapeString(title)
	escapedSummary := html.EscapeString(promptSummary(prompt))
	points := studioPromptPoints(promptSummary(prompt), []string{
		"Turn fragmented notes into a structured office deliverable.",
		"Keep every output previewable, downloadable, versioned, and reusable.",
		"Use trace, queue, and SLA signals to make slow tasks diagnosable.",
	})
	var pointCards strings.Builder
	for i, point := range points {
		pointCards.WriteString(fmt.Sprintf(`<article class="insight-card">
          <span class="insight-card__index">0%d</span>
          <p>%s</p>
        </article>`, i+1, html.EscapeString(point)))
	}
	return fmt.Sprintf(`<!doctype html>
<html lang="zh-CN">
<head>
  <meta charset="utf-8" />
  <meta name="viewport" content="width=device-width, initial-scale=1" />
  <title>%s</title>
  <style>
    :root {
      color-scheme: light;
      font-family: Inter, "SF Pro Display", "Segoe UI", "Microsoft YaHei", system-ui, sans-serif;
      --ink: #0f172a;
      --muted: #64748b;
      --line: #e2e8f0;
      --blue: #2563eb;
      --violet: #7c3aed;
      --green: #16a34a;
      --amber: #f59e0b;
      --paper: rgba(255, 255, 255, .88);
    }
    * { box-sizing: border-box; }
    body {
      margin: 0;
      min-height: 100vh;
      background:
        radial-gradient(circle at 12%% 8%%, rgba(37, 99, 235, .20), transparent 30%%),
        radial-gradient(circle at 88%% 0%%, rgba(124, 58, 237, .18), transparent 28%%),
        linear-gradient(180deg, #f8fafc 0%%, #eef4ff 100%%);
      color: var(--ink);
    }
    main { max-width: 1180px; margin: 0 auto; padding: 42px 24px 56px; }
    .hero {
      position: relative;
      overflow: hidden;
      min-height: 360px;
      padding: 42px;
      border-radius: 34px;
      background: linear-gradient(135deg, #0f172a 0%%, #172554 52%%, #312e81 100%%);
      color: #fff;
      box-shadow: 0 28px 80px rgba(15, 23, 42, .28);
    }
    .hero::after {
      content: "";
      position: absolute;
      right: -90px;
      top: -120px;
      width: 420px;
      height: 420px;
      border-radius: 999px;
      background: linear-gradient(135deg, rgba(96, 165, 250, .65), rgba(168, 85, 247, .45));
      filter: blur(4px);
    }
    .eyebrow {
      display: inline-flex;
      gap: 8px;
      align-items: center;
      padding: 8px 12px;
      border: 1px solid rgba(191, 219, 254, .28);
      border-radius: 999px;
      background: rgba(255, 255, 255, .10);
      color: #bfdbfe;
      font-size: 12px;
      font-weight: 800;
      letter-spacing: .12em;
      text-transform: uppercase;
    }
    h1 { position: relative; z-index: 1; max-width: 760px; margin: 30px 0 18px; font-size: clamp(38px, 6vw, 70px); line-height: .98; letter-spacing: -.055em; }
    .hero p { position: relative; z-index: 1; max-width: 760px; margin: 0; color: #dbeafe; font-size: 17px; line-height: 1.8; }
    .hero-meta { position: relative; z-index: 1; display: flex; gap: 12px; flex-wrap: wrap; margin-top: 34px; }
    .pill { padding: 10px 14px; border-radius: 999px; background: rgba(255,255,255,.13); color: #e0f2fe; font-size: 13px; }
    .section { margin-top: 24px; }
    .section-head { display: flex; justify-content: space-between; gap: 20px; align-items: end; margin: 36px 0 14px; }
    .section-head h2 { margin: 0; font-size: 24px; letter-spacing: -.02em; }
    .section-head p { max-width: 560px; margin: 0; color: var(--muted); line-height: 1.7; }
    .metrics { display: grid; grid-template-columns: repeat(4, minmax(0, 1fr)); gap: 14px; }
    .metric-card, .panel, .insight-card, .step, .risk-card {
      border: 1px solid rgba(148, 163, 184, .26);
      background: var(--paper);
      box-shadow: 0 18px 45px rgba(15, 23, 42, .08);
      backdrop-filter: blur(18px);
    }
    .metric-card { padding: 22px; border-radius: 24px; }
    .metric-card strong { display: block; font-size: 34px; letter-spacing: -.04em; }
    .metric-card span { display: block; margin-top: 6px; color: var(--muted); font-size: 13px; }
    .insights { display: grid; grid-template-columns: repeat(3, minmax(0, 1fr)); gap: 14px; }
    .insight-card { min-height: 156px; padding: 22px; border-radius: 26px; }
    .insight-card__index { color: var(--blue); font-weight: 900; letter-spacing: .08em; }
    .insight-card p { margin: 18px 0 0; color: #1e293b; line-height: 1.72; }
    .workflow { display: grid; grid-template-columns: repeat(4, 1fr); gap: 12px; }
    .step { position: relative; padding: 22px; border-radius: 24px; overflow: hidden; }
    .step::before { content: ""; position: absolute; left: 0; top: 0; right: 0; height: 5px; background: linear-gradient(90deg, var(--blue), var(--violet)); }
    .step b { display: block; margin-bottom: 8px; font-size: 16px; }
    .step p { margin: 0; color: var(--muted); line-height: 1.62; font-size: 13px; }
    .two-col { display: grid; grid-template-columns: 1.2fr .8fr; gap: 16px; }
    .panel { padding: 26px; border-radius: 28px; }
    .panel h3 { margin: 0 0 14px; font-size: 20px; }
    .roadmap { display: grid; gap: 14px; }
    .roadmap-row { display: grid; grid-template-columns: 110px 1fr; gap: 14px; align-items: center; }
    .bar { height: 12px; border-radius: 999px; background: #dbeafe; overflow: hidden; }
    .bar span { display: block; height: 100%%; border-radius: inherit; background: linear-gradient(90deg, var(--blue), var(--green)); }
    .risk-grid { display: grid; gap: 10px; }
    .risk-card { display: flex; justify-content: space-between; gap: 18px; padding: 14px 16px; border-radius: 18px; }
    .risk-card span { color: var(--muted); }
    table { width: 100%%; border-collapse: collapse; overflow: hidden; border-radius: 20px; }
    th, td { text-align: left; padding: 14px 12px; border-bottom: 1px solid #edf2f7; }
    th { color: var(--muted); font-size: 12px; text-transform: uppercase; letter-spacing: .08em; }
    td { color: #1e293b; }
    @media (max-width: 900px) {
      .metrics, .insights, .workflow, .two-col { grid-template-columns: 1fr; }
      .section-head { display: block; }
      .hero { padding: 30px; }
    }
  </style>
</head>
<body>
  <main>
    <section class="hero">
      <span class="eyebrow">EggKid Studio · Enterprise Copilot</span>
      <h1>%s</h1>
      <p>%s</p>
      <div class="hero-meta">
        <span class="pill">Version v%d</span>
        <span class="pill">Previewable HTML</span>
        <span class="pill">Downloadable artifact</span>
      </div>
    </section>
    <section class="section metrics">
      <article class="metric-card"><strong>4</strong><span>Studio output modes</span></article>
      <article class="metric-card"><strong>7</strong><span>PPT-ready sections</span></article>
      <article class="metric-card"><strong>100%%</strong><span>Downloadable records</span></article>
      <article class="metric-card"><strong>v%d</strong><span>Current artifact version</span></article>
    </section>
    <section class="section">
      <div class="section-head">
        <h2>Key insights</h2>
        <p>Studio turns the current prompt into reusable business material. These cards are intentionally concise so they can be lifted into PPT or a weekly report.</p>
      </div>
      <div class="insights">%s</div>
    </section>
    <section class="section workflow">
      <article class="step"><b>01 · Understand</b><p>Read task intent, target audience, reference files, and output constraints.</p></article>
      <article class="step"><b>02 · Structure</b><p>Convert loose notes into sections, decisions, risks, and next actions.</p></article>
      <article class="step"><b>03 · Generate</b><p>Create real files such as PPTX, HTML, XLSX, and Markdown documents.</p></article>
      <article class="step"><b>04 · Archive</b><p>Keep records versioned, downloadable, deletable, and ready for reuse.</p></article>
    </section>
    <section class="section two-col">
      <article class="panel">
        <h3>Execution roadmap</h3>
        <div class="roadmap">
          <div class="roadmap-row"><b>Now</b><div class="bar"><span style="width: 82%%"></span></div></div>
          <div class="roadmap-row"><b>Next</b><div class="bar"><span style="width: 58%%"></span></div></div>
          <div class="roadmap-row"><b>Scale</b><div class="bar"><span style="width: 36%%"></span></div></div>
        </div>
      </article>
      <article class="panel">
        <h3>Governance notes</h3>
        <div class="risk-grid">
          <div class="risk-card"><b>Large files</b><span>Async queue + trace</span></div>
          <div class="risk-card"><b>Model failures</b><span>Retry + fallback</span></div>
          <div class="risk-card"><b>Reuse</b><span>Version + template</span></div>
        </div>
      </article>
    </section>
    <section class="section panel">
      <h3>Action table</h3>
      <table>
        <thead><tr><th>Item</th><th>Owner</th><th>Status</th><th>Advice</th></tr></thead>
        <tbody>
          <tr><td>Fill business context</td><td>Owner</td><td>In progress</td><td>Attach source evidence and target audience.</td></tr>
          <tr><td>Upgrade generator</td><td>Backend</td><td>Next</td><td>Connect configured LLM for content-aware output.</td></tr>
          <tr><td>Review conclusions</td><td>Reviewer</td><td>Todo</td><td>Flag risks, confidential details, and missing sources.</td></tr>
        </tbody>
      </table>
    </section>
  </main>
</body>
</html>
`, escapedTitle, escapedTitle, escapedSummary, version, version, pointCards.String())
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
