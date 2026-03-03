package cmd

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/url"
	"os"
	"path/filepath"
	"testing"

	"github.com/operator-kit/hs-cli/internal/output"
	"github.com/operator-kit/hs-cli/internal/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConversationAttachmentsUpload(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "note.txt")
	require.NoError(t, os.WriteFile(filePath, []byte("hello attachment"), 0o644))

	mock := &mockClient{
		CreateAttachmentFn: func(ctx context.Context, convID string, threadID string, body any) error {
			assert.Equal(t, "10", convID)
			assert.Equal(t, "99", threadID)
			payload, ok := body.(types.AttachmentCreate)
			require.True(t, ok)
			assert.Equal(t, "note.txt", payload.FileName)
			assert.Equal(t, "text/plain", payload.MimeType)
			assert.Equal(t, base64.StdEncoding.EncodeToString([]byte("hello attachment")), payload.Data)
			return nil
		},
	}

	buf := setupTest(mock)
	defer func() { output.Out = os.Stdout }()

	rootCmd.SetArgs([]string{
		"inbox", "conversations", "attachments", "upload", "10",
		"--thread-id", "99",
		"--file", filePath,
		"--mime-type", "text/plain",
	})
	require.NoError(t, rootCmd.Execute())
	assert.Contains(t, buf.String(), "Uploaded attachment for conversation 10 thread 99")
}

func TestConversationAttachmentsList(t *testing.T) {
	mock := &mockClient{
		GetConversationFn: func(ctx context.Context, convID string, params url.Values) (json.RawMessage, error) {
			assert.Equal(t, "42", convID)
			assert.Equal(t, "threads", params.Get("embed"))
			return json.RawMessage(`{
				"id":42,
				"_embedded":{
					"threads":[
						{"id":1,"attachments":[{"id":1,"filename":"file.txt","mimeType":"text/plain","size":12}]}
					]
				}
			}`), nil
		},
	}

	buf := setupTest(mock)
	defer func() { output.Out = os.Stdout }()

	rootCmd.SetArgs([]string{"inbox", "conversations", "attachments", "list", "42"})
	require.NoError(t, rootCmd.Execute())
	assert.Contains(t, buf.String(), "file.txt")
}

func TestConversationAttachmentsGet(t *testing.T) {
	mock := &mockClient{
		GetAttachmentDataFn: func(ctx context.Context, convID string, attachmentID string) (json.RawMessage, error) {
			assert.Equal(t, "42", convID)
			assert.Equal(t, "7", attachmentID)
			return json.RawMessage(`{"filename":"invoice.pdf","mimeType":"application/pdf","data":"aGVsbG8="}`), nil
		},
	}

	buf := setupTest(mock)
	defer func() { output.Out = os.Stdout }()

	rootCmd.SetArgs([]string{"inbox", "conversations", "attachments", "get", "42", "7"})
	require.NoError(t, rootCmd.Execute())
	assert.Contains(t, buf.String(), "invoice.pdf")
}

func TestConversationAttachmentsDelete(t *testing.T) {
	mock := &mockClient{
		DeleteAttachmentFn: func(ctx context.Context, convID string, attachmentID string) error {
			assert.Equal(t, "42", convID)
			assert.Equal(t, "7", attachmentID)
			return nil
		},
	}

	buf := setupTest(mock)
	defer func() { output.Out = os.Stdout }()

	rootCmd.SetArgs([]string{"inbox", "conversations", "attachments", "delete", "42", "7"})
	require.NoError(t, rootCmd.Execute())
	assert.Contains(t, buf.String(), "Deleted attachment 7 from conversation 42")
}
