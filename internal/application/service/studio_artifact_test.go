package service

import (
	"archive/zip"
	"bytes"
	"encoding/base64"
	"strings"
	"testing"

	"github.com/Tencent/WeKnora/internal/types"
	"github.com/stretchr/testify/require"
)

func TestStudioArtifactRenderGeneratesRealPPTX(t *testing.T) {
	svc := &studioArtifactService{}

	artifact := svc.render(types.StudioArtifactTypePPT, "Quarterly Review", "summarize risks and next actions", 2)

	require.Equal(t, "Quarterly-Review-v2.pptx", artifact.filename)
	require.Equal(t, "application/vnd.openxmlformats-officedocument.presentationml.presentation", artifact.mimeType)
	require.Equal(t, "base64", artifact.metadata["content_encoding"])
	require.Greater(t, artifact.size, int64(0))

	data, err := base64.StdEncoding.DecodeString(artifact.content)
	require.NoError(t, err)
	require.Equal(t, artifact.size, int64(len(data)))

	reader, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	require.NoError(t, err)
	names := map[string]bool{}
	for _, file := range reader.File {
		names[file.Name] = true
	}
	require.True(t, names["[Content_Types].xml"])
	require.True(t, names["ppt/presentation.xml"])
	require.True(t, names["ppt/slides/slide1.xml"])
	require.True(t, names["ppt/slides/_rels/slide1.xml.rels"])
}

func TestStudioArtifactRenderGeneratesRealXLSX(t *testing.T) {
	svc := &studioArtifactService{}

	artifact := svc.render(types.StudioArtifactTypeTable, "Action Table", "track owners and status", 1)

	require.Equal(t, "Action-Table-v1.xlsx", artifact.filename)
	require.Equal(t, "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", artifact.mimeType)
	require.Equal(t, "base64", artifact.metadata["content_encoding"])

	data, err := base64.StdEncoding.DecodeString(artifact.content)
	require.NoError(t, err)
	require.Equal(t, artifact.size, int64(len(data)))

	reader, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	require.NoError(t, err)
	names := map[string]bool{}
	for _, file := range reader.File {
		names[file.Name] = true
	}
	require.True(t, names["[Content_Types].xml"])
	require.True(t, names["xl/workbook.xml"])
	require.True(t, names["xl/worksheets/sheet1.xml"])
}

func TestStudioArtifactRenderHTMLStaysPreviewableText(t *testing.T) {
	svc := &studioArtifactService{}

	artifact := svc.render(types.StudioArtifactTypeHTML, "Demo Page", "build a dashboard", 1)

	require.Equal(t, "Demo-Page-v1.html", artifact.filename)
	require.Equal(t, "text/html; charset=utf-8", artifact.mimeType)
	require.Equal(t, "text", artifact.metadata["content_encoding"])
	require.True(t, strings.Contains(artifact.content, "<!doctype html>"))
}
