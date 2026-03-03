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

func newTeamsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "teams",
		Short: "Manage teams",
	}

	listCmd := teamsListCmd()
	permission.Annotate(listCmd, "teams", permission.OpRead)

	membersCmd := teamsMembersCmd()
	permission.Annotate(membersCmd, "teams", permission.OpRead)

	cmd.AddCommand(listCmd, membersCmd)
	return cmd
}

func teamsListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List teams",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()
			params := url.Values{}
			params.Set("page", strconv.Itoa(page))

			if isJSON() {
				items, _, err := api.PaginateAll(ctx, apiClient.ListTeams, params, "teams", noPaginate)
				if err != nil {
					return err
				}
				if !isJSONClean() {
					return printRawWithPII(mustMarshal(items))
				}
				return printRawWithPII(mustMarshal(cleanRawItems(items, cleanMinimal)))
			}

			items, _, err := api.PaginateAll(ctx, apiClient.ListTeams, params, "teams", noPaginate)
			if err != nil {
				return err
			}

			teams := make([]types.Team, 0, len(items))
			for _, raw := range items {
				var t types.Team
				json.Unmarshal(raw, &t)
				teams = append(teams, t)
			}

			rows := make([]map[string]string, len(teams))
			for i, t := range teams {
				rows[i] = map[string]string{
					"id":   strconv.Itoa(t.ID),
					"name": t.Name,
				}
			}
			return output.Print(getFormat(), []string{"id", "name"}, rows)
		},
	}
}

func teamsMembersCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "members <team-id>",
		Short: "List members for a team",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()
			params := url.Values{}
			params.Set("page", strconv.Itoa(page))

			fetch := func(ctx context.Context, p url.Values) (json.RawMessage, error) {
				return apiClient.ListTeamMembers(ctx, args[0], p)
			}

			if isJSON() {
				items, _, err := api.PaginateAll(ctx, fetch, params, "users", noPaginate)
				if err != nil {
					return err
				}
				if !isJSONClean() {
					return printRawWithPII(mustMarshal(items))
				}
				return printRawWithPII(mustMarshal(cleanRawItems(items, cleanUser)))
			}

			items, _, err := api.PaginateAll(ctx, fetch, params, "users", noPaginate)
			if err != nil {
				return err
			}
			engine, err := newPIIEngine()
			if err != nil {
				return err
			}

			users := make([]types.User, 0, len(items))
			for _, raw := range items {
				var u types.User
				json.Unmarshal(raw, &u)
				redactUserForOutput(engine, &u)
				users = append(users, u)
			}

			rows := make([]map[string]string, len(users))
			for i, u := range users {
				rows[i] = map[string]string{
					"id":    strconv.Itoa(u.ID),
					"name":  fmt.Sprintf("%s %s", u.FirstName, u.LastName),
					"email": u.Email,
					"role":  u.Role,
				}
			}
			return output.Print(getFormat(), []string{"id", "name", "email", "role"}, rows)
		},
	}
}
