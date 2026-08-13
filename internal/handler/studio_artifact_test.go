package handler

import (
	"encoding/base64"
	"testing"

	"github.com/Tencent/WeKnora/internal/types"
	"github.com/stretchr/testify/require"
)

func TestStudioArtifactBytesDecodesBase64Content(t *testing.T) {
	payload := []byte{0x50, 0x4b, 0x03, 0x04, 0x01}
	artifact := &types.StudioArtifact{
		Content:  base64.StdEncoding.EncodeToString(payload),
		Metadata: types.JSONMap{"content_encoding": "base64"},
	}

	got, err := studioArtifactBytes(artifact)

	require.NoError(t, err)
	require.Equal(t, payload, got)
}

func TestStudioArtifactBytesKeepsLegacyTextContent(t *testing.T) {
	artifact := &types.StudioArtifact{Content: "plain legacy artifact"}

	got, err := studioArtifactBytes(artifact)

	require.NoError(t, err)
	require.Equal(t, []byte("plain legacy artifact"), got)
}
