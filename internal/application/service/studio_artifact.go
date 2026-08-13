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
	return []studioSlide{
		{
			title:    title,
			subtitle: fmt.Sprintf("Studio 自动生成 · v%d", version),
			bullets: []string{
				"面向企业汇报、项目复盘和方案评审",
				"可下载为真实 PPTX 文件，便于继续编辑",
				summary,
			},
			accent: "2563EB",
		},
		{
			title: "背景与问题",
			bullets: []string{
				"明确当前业务/项目上下文，避免只停留在零散结论",
				"识别影响效率、质量、稳定性或协同的核心痛点",
				"将问题转化为可追踪、可复盘、可验收的改进项",
			},
			accent: "7C3AED",
		},
		{
			title: "目标与验收标准",
			bullets: []string{
				"定义交付范围：输入、处理链路、输出物和责任人",
				"设置指标：耗时、准确率、稳定性、可用性、用户体验",
				"建立兜底策略：重试、降级、人工审核与版本回滚",
			},
			accent: "0891B2",
		},
		{
			title: "核心方案",
			bullets: []string{
				"沉淀标准化流程：配置、生成、预览、下载、归档",
				"增强可观测性：队列雷达、任务 SLA、单文件 Trace",
				"用 Studio 承接办公产出：PPT、HTML、表格、文档",
			},
			accent: "16A34A",
		},
		{
			title: "流程设计",
			bullets: []string{
				"输入：对话上下文、知识库、上传资料、参考模板",
				"处理：解析、结构化、生成、校验、版本化记录",
				"输出：可下载文件、可预览页面、可复用记录",
			},
			accent: "EA580C",
		},
		{
			title: "风险与治理",
			bullets: []string{
				"大文档解析慢：拆分、异步队列、checkpoint、Trace 诊断",
				"模型调用失败：健康检查、限流、重试、替代模型",
				"文件管理混乱：生成记录、版本号、批量删除、后续共享",
			},
			accent: "DC2626",
		},
		{
			title: "下一步计划",
			bullets: []string{
				"接入真实 LLM 内容生成，让产物从模板走向智能草稿",
				"支持模板库、企业主题、团队共享与审批流",
				"继续完善高并发队列治理、SLA 告警和审计闭环",
			},
			accent: "4F46E5",
		},
	}
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
    </p:spTree>
  </p:cSld>
  <p:clrMapOvr><a:masterClrMapping/></p:clrMapOvr>
</p:sld>`,
		pptxRect(2, "Accent", 0, 0, 12192000, 280000, accent),
		pptxRect(3, "Panel", 700000, 880000, 10800000, 5100000, "FFFFFF"),
		pptxTextBox(4, "Title", 1060000, 1040000, 9700000, 650000, title, "111827", 420000, true),
		pptxTextBox(5, "Subtitle", 1060000, 1660000, 9700000, 360000, subtitle, accent, 185000, false),
		bulletXML.String()+pptxFooter(index, progress, accent),
	)
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
