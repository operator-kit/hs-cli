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

func newUsersCmd() *cobra.Command {
	usersCmd := &cobra.Command{
		Use:   "users",
		Short: "Manage users",
	}
	listCmd := usersListCmd()
	permission.Annotate(listCmd, "users", permission.OpRead)
	listCmd.Flags().String("email", "", "filter by email")
	listCmd.Flags().String("mailbox", "", "filter by mailbox ID")

	getCmd := usersGetCmd()
	permission.Annotate(getCmd, "users", permission.OpRead)

	meCmd := usersMeCmd()
	permission.Annotate(meCmd, "users", permission.OpRead)

	deleteCmd := usersDeleteCmd()
	permission.Annotate(deleteCmd, "users", permission.OpDelete)

	usersCmd.AddCommand(
		listCmd,
		getCmd,
		meCmd,
		deleteCmd,
		newUsersStatusCmd(),
	)
	return usersCmd
}

func usersListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List users",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()
			params := url.Values{}
			params.Set("page", strconv.Itoa(page))
			params.Set("pageSize", strconv.Itoa(perPage))
			if v, _ := cmd.Flags().GetString("email"); v != "" {
				params.Set("email", v)
			}
			if v, _ := cmd.Flags().GetString("mailbox"); v != "" {
				params.Set("mailbox", v)
			}

			if isJSON() {
				items, _, err := api.PaginateAll(ctx, apiClient.ListUsers, params, "users", noPaginate)
				if err != nil {
					return err
				}
				if !isJSONClean() {
					return printRawWithPII(mustMarshal(items))
				}
				return printRawWithPII(mustMarshal(cleanRawItems(items, cleanUser)))
			}

			items, pageInfo, err := api.PaginateAll(ctx, apiClient.ListUsers, params, "users", noPaginate)
			if err != nil {
				return err
			}
			engine, err := newPIIEngine()
			if err != nil {
				return err
			}

			var users []types.User
			for _, raw := range items {
				var u types.User
				json.Unmarshal(raw, &u)
				redactUserForOutput(engine, &u)
				users = append(users, u)
			}

			cols := []string{"id", "first_name", "last_name", "email", "role"}
			rows := make([]map[string]string, len(users))
			for i, u := range users {
				rows[i] = map[string]string{
					"id":         strconv.Itoa(u.ID),
					"first_name": u.FirstName,
					"last_name":  u.LastName,
					"email":      u.Email,
					"role":       u.Role,
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

func usersGetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "get <id>",
		Short: "Get user details",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			data, err := apiClient.GetUser(context.Background(), args[0])
			if err != nil {
				return err
			}

			if isJSON() {
				if !isJSONClean() {
					return printRawWithPII(data)
				}
				return printRawWithPII(mustMarshal(cleanRawObject(data, cleanUser)))
			}

			var u types.User
			json.Unmarshal(data, &u)
			engine, err := newPIIEngine()
			if err != nil {
				return err
			}
			redactUserForOutput(engine, &u)

			cols := []string{"id", "first_name", "last_name", "email", "role", "type"}
			rows := []map[string]string{{
				"id":         strconv.Itoa(u.ID),
				"first_name": u.FirstName,
				"last_name":  u.LastName,
				"email":      u.Email,
				"role":       u.Role,
				"type":       u.Type,
			}}
			return output.Print(getFormat(), cols, rows)
		},
	}
}

func usersMeCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "me",
		Short: "Get authenticated user",
		RunE: func(cmd *cobra.Command, args []string) error {
			data, err := apiClient.GetResourceOwner(context.Background())
			if err != nil {
				return err
			}
			if isJSON() {
				if !isJSONClean() {
					return printRawWithPII(data)
				}
				return printRawWithPII(mustMarshal(cleanRawObject(data, cleanUser)))
			}

			var u types.User
			json.Unmarshal(data, &u)
			engine, err := newPIIEngine()
			if err != nil {
				return err
			}
			redactUserForOutput(engine, &u)
			return output.Print(getFormat(), []string{"id", "first_name", "last_name", "email", "role"}, []map[string]string{{
				"id":         strconv.Itoa(u.ID),
				"first_name": u.FirstName,
				"last_name":  u.LastName,
				"email":      u.Email,
				"role":       u.Role,
			}})
		},
	}
}

func usersDeleteCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "delete <id>",
		Short: "Delete a user",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := apiClient.DeleteUser(context.Background(), args[0]); err != nil {
				return err
			}
			fmt.Fprintf(output.Out, "Deleted user %s\n", args[0])
			return nil
		},
	}
}

func newUsersStatusCmd() *cobra.Command {
	listCmd := &cobra.Command{
		Use:   "list",
		Short: "List user statuses",
		RunE: func(cmd *cobra.Command, args []string) error {
			params := url.Values{}
			params.Set("page", strconv.Itoa(page))
			params.Set("pageSize", strconv.Itoa(perPage))
			data, err := apiClient.ListUserStatuses(context.Background(), params)
			if err != nil {
				return err
			}

			items, err := extractEmbeddedWithCandidates(data, "statuses", "userStatuses", "status")
			if err != nil {
				if isJSON() {
					return printRawWithPII(data)
				}
				return err
			}
			if isJSON() {
				return printRawWithPII(mustMarshal(items))
			}

			rows := make([]map[string]string, len(items))
			for i, raw := range items {
				var row map[string]any
				json.Unmarshal(raw, &row)
				rows[i] = map[string]string{
					"id":    fmt.Sprintf("%v", row["id"]),
					"name":  fmt.Sprintf("%v", row["name"]),
					"color": fmt.Sprintf("%v", row["color"]),
				}
			}
			return output.Print(getFormat(), []string{"id", "name", "color"}, rows)
		},
	}

	getCmd := &cobra.Command{
		Use:   "get <user-id>",
		Short: "Get a user's current status",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			data, err := apiClient.GetUserStatus(context.Background(), args[0])
			if err != nil {
				return err
			}
			if isJSON() {
				return printRawWithPII(data)
			}
			return output.Print("table", []string{"user_id", "status"}, []map[string]string{{
				"user_id": args[0],
				"status":  truncate(string(data), 120),
			}})
		},
	}

	setCmd := &cobra.Command{
		Use:   "set <user-id>",
		Short: "Set a user's status",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			rawJSON, _ := cmd.Flags().GetString("json")
			status, _ := cmd.Flags().GetString("status")

			body := map[string]any{}
			if rawJSON != "" {
				parsed, err := parseJSONBody(rawJSON)
				if err != nil {
					return fmt.Errorf("invalid --json payload: %w", err)
				}
				body = parsed
			} else {
				if status == "" {
					return fmt.Errorf("either --status or --json is required")
				}
				body["status"] = status
			}

			if err := apiClient.SetUserStatus(context.Background(), args[0], body); err != nil {
				return err
			}
			fmt.Fprintf(output.Out, "Updated status for user %s\n", args[0])
			return nil
		},
	}
	setCmd.Flags().String("status", "", "status value")
	setCmd.Flags().String("json", "", "full status payload as JSON object")

	permission.Annotate(listCmd, "users", permission.OpRead)
	permission.Annotate(getCmd, "users", permission.OpRead)
	permission.Annotate(setCmd, "users", permission.OpWrite)

	cmd := &cobra.Command{
		Use:   "status",
		Short: "Manage user statuses",
	}
	cmd.AddCommand(listCmd, getCmd, setCmd)
	return cmd
}
