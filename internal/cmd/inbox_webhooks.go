package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"

	"github.com/spf13/cobra"

	"github.com/operator-kit/hs-cli/internal/api"
	"github.com/operator-kit/hs-cli/internal/output"
	"github.com/operator-kit/hs-cli/internal/permission"
	"github.com/operator-kit/hs-cli/internal/types"
)

func newWebhooksCmd() *cobra.Command {
	whCmd := &cobra.Command{
		Use:     "webhooks",
		Aliases: []string{"wh"},
		Short:   "Manage webhooks",
	}

	createCmd := webhooksCreateCmd()
	permission.Annotate(createCmd, "webhooks", permission.OpWrite)
	createCmd.Flags().String("url", "", "webhook URL (required)")
	createCmd.Flags().StringSlice("events", nil, "events to subscribe to (required)")
	createCmd.Flags().String("secret", "", "webhook secret (required)")
	createCmd.Flags().String("payload-version", "", "payload version (V1|V2)")
	createCmd.Flags().IntSlice("mailbox-ids", nil, "mailbox IDs to scope the webhook")
	createCmd.Flags().Bool("notification", false, "send lightweight notification payloads")
	createCmd.Flags().String("label", "", "human-readable webhook label")
	createCmd.MarkFlagRequired("url")
	createCmd.MarkFlagRequired("events")
	createCmd.MarkFlagRequired("secret")

	updateCmd := webhooksUpdateCmd()
	permission.Annotate(updateCmd, "webhooks", permission.OpWrite)
	updateCmd.Flags().String("url", "", "webhook URL")
	updateCmd.Flags().StringSlice("events", nil, "events to subscribe to")
	updateCmd.Flags().String("secret", "", "webhook secret")
	updateCmd.Flags().String("payload-version", "", "payload version (V1|V2)")
	updateCmd.Flags().IntSlice("mailbox-ids", nil, "mailbox IDs to scope the webhook")
	updateCmd.Flags().Bool("notification", false, "send lightweight notification payloads")
	updateCmd.Flags().String("label", "", "human-readable webhook label")

	listCmd := webhooksListCmd()
	permission.Annotate(listCmd, "webhooks", permission.OpRead)

	getCmd := webhooksGetCmd()
	permission.Annotate(getCmd, "webhooks", permission.OpRead)

	deleteCmd := webhooksDeleteCmd()
	permission.Annotate(deleteCmd, "webhooks", permission.OpDelete)

	whCmd.AddCommand(listCmd, getCmd, createCmd, updateCmd, deleteCmd)
	return whCmd
}

func webhooksListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List webhooks",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()
			params := url.Values{}
			params.Set("page", strconv.Itoa(page))
			params.Set("pageSize", strconv.Itoa(perPage))

			if isJSON() {
				items, _, err := api.PaginateAll(ctx, apiClient.ListWebhooks, params, "webhooks", noPaginate)
				if err != nil {
					return err
				}
				if !isJSONClean() {
					return output.PrintRaw(mustMarshal(items))
				}
				return output.PrintRaw(mustMarshal(cleanRawItems(items, cleanMinimal)))
			}

			items, pageInfo, err := api.PaginateAll(ctx, apiClient.ListWebhooks, params, "webhooks", noPaginate)
			if err != nil {
				return err
			}

			var webhooks []types.Webhook
			for _, raw := range items {
				var w types.Webhook
				json.Unmarshal(raw, &w)
				webhooks = append(webhooks, w)
			}

			cols := []string{"id", "url", "state", "events"}
			rows := make([]map[string]string, len(webhooks))
			for i, w := range webhooks {
				events := ""
				for j, e := range w.Events {
					if j > 0 {
						events += ", "
					}
					events += e
				}
				rows[i] = map[string]string{
					"id":     strconv.Itoa(w.ID),
					"url":    w.URL,
					"state":  w.State,
					"events": truncate(events, 60),
				}
			}
			if err := output.Print(getFormat(), cols, rows); err != nil {
				return err
			}
			if pageInfo != nil && !noPaginate {
				fmt.Fprintf(output.Out, "\nPage %d of %d (%d total)\n", pageInfo.Number, pageInfo.TotalPages, pageInfo.TotalElements)
			}
			return nil
		},
	}
}

func webhooksGetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "get <id>",
		Short: "Get webhook details",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			data, err := apiClient.GetWebhook(context.Background(), args[0])
			if err != nil {
				return err
			}

			if isJSON() {
				if !isJSONClean() {
					return output.PrintRaw(data)
				}
				return output.PrintRaw(mustMarshal(cleanRawObject(data, cleanMinimal)))
			}

			var w types.Webhook
			json.Unmarshal(data, &w)

			events := ""
			for j, e := range w.Events {
				if j > 0 {
					events += ", "
				}
				events += e
			}
			mailboxIDs := ""
			for j, id := range w.MailboxIDs {
				if j > 0 {
					mailboxIDs += ", "
				}
				mailboxIDs += strconv.Itoa(id)
			}
			notification := "false"
			if w.Notification {
				notification = "true"
			}

			cols := []string{"id", "url", "state", "events", "secret", "payload_version", "mailbox_ids", "notification", "label"}
			rows := []map[string]string{{
				"id":              strconv.Itoa(w.ID),
				"url":             w.URL,
				"state":           w.State,
				"events":          events,
				"secret":          w.Secret,
				"payload_version": w.PayloadVersion,
				"mailbox_ids":     mailboxIDs,
				"notification":    notification,
				"label":           w.Label,
			}}
			return output.Print(getFormat(), cols, rows)
		},
	}
}

func webhooksCreateCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "create",
		Short: "Create a webhook",
		RunE: func(cmd *cobra.Command, args []string) error {
			whURL, _ := cmd.Flags().GetString("url")
			events, _ := cmd.Flags().GetStringSlice("events")
			secret, _ := cmd.Flags().GetString("secret")
			payloadVersion, _ := cmd.Flags().GetString("payload-version")
			mailboxIDs, _ := cmd.Flags().GetIntSlice("mailbox-ids")
			notification, _ := cmd.Flags().GetBool("notification")
			label, _ := cmd.Flags().GetString("label")

			wh := types.WebhookCreate{
				URL:            whURL,
				Events:         events,
				Secret:         secret,
				PayloadVersion: payloadVersion,
				MailboxIDs:     mailboxIDs,
				Label:          label,
			}
			if cmd.Flags().Changed("notification") {
				wh.Notification = &notification
			}

			id, err := apiClient.CreateWebhook(context.Background(), wh)
			if err != nil {
				return err
			}
			fmt.Fprintf(output.Out, "Created webhook %s\n", id)
			return nil
		},
	}
}

func webhooksUpdateCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "update <id>",
		Short: "Update a webhook",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			body := types.WebhookUpdate{}
			changed := false
			if v, _ := cmd.Flags().GetString("url"); v != "" {
				body.URL = v
				changed = true
			}
			if v, _ := cmd.Flags().GetStringSlice("events"); len(v) > 0 {
				body.Events = v
				changed = true
			}
			if v, _ := cmd.Flags().GetString("secret"); v != "" {
				body.Secret = v
				changed = true
			}
			if v, _ := cmd.Flags().GetString("payload-version"); v != "" {
				body.PayloadVersion = v
				changed = true
			}
			if v, _ := cmd.Flags().GetIntSlice("mailbox-ids"); len(v) > 0 {
				body.MailboxIDs = v
				changed = true
			}
			if cmd.Flags().Changed("notification") {
				v, _ := cmd.Flags().GetBool("notification")
				body.Notification = &v
				changed = true
			}
			if v, _ := cmd.Flags().GetString("label"); v != "" {
				body.Label = v
				changed = true
			}
			if !changed {
				return fmt.Errorf("no fields to update")
			}
			if err := apiClient.UpdateWebhook(context.Background(), args[0], body); err != nil {
				return err
			}
			fmt.Fprintf(output.Out, "Updated webhook %s\n", args[0])
			return nil
		},
	}
}

func webhooksDeleteCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "delete <id>",
		Short: "Delete a webhook",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := apiClient.DeleteWebhook(context.Background(), args[0]); err != nil {
				return err
			}
			fmt.Fprintf(output.Out, "Deleted webhook %s\n", args[0])
			return nil
		},
	}
}
