package cmd

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"mime"
	"net/url"
	"os"
	"path/filepath"
	"strconv"

	"github.com/spf13/cobra"

	"github.com/operator-kit/hs-cli/internal/output"
	"github.com/operator-kit/hs-cli/internal/permission"
	"github.com/operator-kit/hs-cli/internal/types"
)

func newConversationAttachmentsCmd() *cobra.Command {
	uploadCmd := &cobra.Command{
		Use:   "upload <conversation-id>",
		Short: "Upload a conversation attachment",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			threadID, _ := cmd.Flags().GetInt("thread-id")
			filePath, _ := cmd.Flags().GetString("file")
			fileName, _ := cmd.Flags().GetString("filename")
			mimeType, _ := cmd.Flags().GetString("mime-type")

			if threadID <= 0 {
				return fmt.Errorf("--thread-id must be a positive integer")
			}

			content, err := os.ReadFile(filePath)
			if err != nil {
				return fmt.Errorf("reading file: %w", err)
			}

			if fileName == "" {
				fileName = filepath.Base(filePath)
			}
			if mimeType == "" {
				mimeType = mime.TypeByExtension(filepath.Ext(fileName))
				if mimeType == "" {
					mimeType = "application/octet-stream"
				}
			}

			body := types.AttachmentCreate{
				FileName: fileName,
				MimeType: mimeType,
				Data:     base64.StdEncoding.EncodeToString(content),
			}

			if err := apiClient.CreateAttachment(context.Background(), args[0], strconv.Itoa(threadID), body); err != nil {
				return err
			}
			fmt.Fprintf(output.Out, "Uploaded attachment for conversation %s thread %d\n", args[0], threadID)
			return nil
		},
	}
	uploadCmd.Flags().Int("thread-id", 0, "thread ID (required)")
	uploadCmd.Flags().String("file", "", "path to file (required)")
	uploadCmd.Flags().String("filename", "", "attachment filename override")
	uploadCmd.Flags().String("mime-type", "", "attachment MIME type")
	uploadCmd.MarkFlagRequired("thread-id")
	uploadCmd.MarkFlagRequired("file")

	listCmd := &cobra.Command{
		Use:   "list <conversation-id>",
		Short: "List attachments for a conversation",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			attachments, err := listConversationAttachments(args[0])
			if err != nil {
				return err
			}

			if isJSON() {
				return output.PrintRaw(mustMarshal(attachments))
			}

			rows := make([]map[string]string, len(attachments))
			for i, a := range attachments {
				rows[i] = map[string]string{
					"id":       strconv.Itoa(a.ID),
					"filename": a.FileName,
					"mime":     a.MimeType,
					"size":     strconv.FormatInt(a.Size, 10),
				}
			}
			return output.Print(getFormat(), []string{"id", "filename", "mime", "size"}, rows)
		},
	}

	getCmd := &cobra.Command{
		Use:   "get <conversation-id> <attachment-id>",
		Short: "Get attachment data",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			data, err := apiClient.GetAttachmentData(context.Background(), args[0], args[1])
			if err != nil {
				return err
			}

			if isJSON() {
				return output.PrintRaw(data)
			}

			var payload map[string]any
			if err := json.Unmarshal(data, &payload); err != nil {
				return output.PrintRaw(data)
			}
			dataLen := 0
			if v, ok := payload["data"].(string); ok {
				dataLen = len(v)
			}
			rows := []map[string]string{{
				"id":         args[1],
				"filename":   asString(payload["filename"]),
				"mime":       asString(payload["mimeType"]),
				"data_bytes": strconv.Itoa(dataLen),
			}}
			return output.Print(getFormat(), []string{"id", "filename", "mime", "data_bytes"}, rows)
		},
	}

	deleteCmd := &cobra.Command{
		Use:   "delete <conversation-id> <attachment-id>",
		Short: "Delete an attachment",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := apiClient.DeleteAttachment(context.Background(), args[0], args[1]); err != nil {
				return err
			}
			fmt.Fprintf(output.Out, "Deleted attachment %s from conversation %s\n", args[1], args[0])
			return nil
		},
	}

	permission.Annotate(uploadCmd, "conversations", permission.OpWrite)
	permission.Annotate(listCmd, "conversations", permission.OpRead)
	permission.Annotate(getCmd, "conversations", permission.OpRead)
	permission.Annotate(deleteCmd, "conversations", permission.OpDelete)

	cmd := &cobra.Command{
		Use:   "attachments",
		Short: "Manage conversation attachments",
	}
	cmd.AddCommand(uploadCmd, listCmd, getCmd, deleteCmd)
	return cmd
}

func listConversationAttachments(conversationID string) ([]types.Attachment, error) {
	params := url.Values{}
	params.Set("embed", "threads")

	convRaw, err := apiClient.GetConversation(context.Background(), conversationID, params)
	if err != nil {
		return nil, err
	}

	threads, err := extractThreadMaps(convRaw)
	if err != nil {
		return nil, err
	}

	out := make([]types.Attachment, 0)
	for _, thread := range threads {
		rawAttachments, ok := thread["attachments"].([]any)
		if !ok {
			continue
		}
		for _, raw := range rawAttachments {
			item, ok := raw.(map[string]any)
			if !ok {
				continue
			}
			out = append(out, types.Attachment{
				ID:       asInt(item["id"]),
				FileName: asString(item["filename"]),
				MimeType: asString(item["mimeType"]),
				Size:     int64(asInt(item["size"])),
			})
		}
	}

	return out, nil
}

func extractThreadMaps(raw json.RawMessage) ([]map[string]any, error) {
	var payload map[string]any
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil, err
	}

	threads := make([]map[string]any, 0)
	if embedded, ok := payload["_embedded"].(map[string]any); ok {
		if embeddedThreads, ok := embedded["threads"].([]any); ok {
			for _, t := range embeddedThreads {
				if thread, ok := t.(map[string]any); ok {
					threads = append(threads, thread)
				}
			}
		}
	}
	if directThreads, ok := payload["threads"].([]any); ok {
		for _, t := range directThreads {
			if thread, ok := t.(map[string]any); ok {
				threads = append(threads, thread)
			}
		}
	}
	return threads, nil
}

func asString(v any) string {
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}

func asInt(v any) int {
	switch t := v.(type) {
	case float64:
		return int(t)
	case int:
		return t
	default:
		return 0
	}
}
