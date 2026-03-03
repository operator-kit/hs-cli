package cmd

import (
	"context"
	"encoding/json"
	"strconv"

	"github.com/spf13/cobra"

	"github.com/operator-kit/hs-cli/internal/output"
	"github.com/operator-kit/hs-cli/internal/permission"
	"github.com/operator-kit/hs-cli/internal/types"
)

func newRatingsCmd() *cobra.Command {
	getCmd := &cobra.Command{
		Use:   "get <id>",
		Short: "Get rating details",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			data, err := apiClient.GetRating(context.Background(), args[0])
			if err != nil {
				return err
			}
			if isJSON() {
				if !isJSONClean() {
					return output.PrintRaw(data)
				}
				return output.PrintRaw(mustMarshal(cleanRawObject(data, cleanMinimal)))
			}

			var r types.Rating
			json.Unmarshal(data, &r)
			return output.Print(getFormat(), []string{"id", "rating", "comments"}, []map[string]string{{
				"id":       strconv.Itoa(r.ID),
				"rating":   r.Rating,
				"comments": r.Comments,
			}})
		},
	}

	permission.Annotate(getCmd, "ratings", permission.OpRead)

	cmd := &cobra.Command{
		Use:   "ratings",
		Short: "Manage ratings",
	}
	cmd.AddCommand(getCmd)
	return cmd
}
