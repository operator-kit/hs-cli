package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"

	"github.com/spf13/cobra"

	"github.com/operator-kit/hs-cli/internal/output"
	"github.com/operator-kit/hs-cli/internal/permission"
	"github.com/operator-kit/hs-cli/internal/types"
)

func newPropertiesCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "properties",
		Short: "Manage customer and conversation properties",
	}
	cmd.AddCommand(newCustomerPropertiesCmd(), newConversationPropertiesCmd())
	return cmd
}

func newCustomerPropertiesCmd() *cobra.Command {
	listCmd := &cobra.Command{
		Use:   "list",
		Short: "List customer properties",
		RunE: func(cmd *cobra.Command, args []string) error {
			params := url.Values{}
			params.Set("page", strconv.Itoa(page))
			params.Set("pageSize", strconv.Itoa(perPage))
			data, err := apiClient.ListCustomerProperties(context.Background(), params)
			if err != nil {
				return err
			}
			return printPropertyList(data, "customerProperties", "properties")
		},
	}

	getCmd := &cobra.Command{
		Use:   "get <id>",
		Short: "Get customer property details",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			data, err := apiClient.GetCustomerProperty(context.Background(), args[0])
			if err != nil {
				return err
			}
			return printSingleProperty(data)
		},
	}

	permission.Annotate(listCmd, "properties", permission.OpRead)
	permission.Annotate(getCmd, "properties", permission.OpRead)

	cmd := &cobra.Command{
		Use:   "customers",
		Short: "Manage customer properties",
	}
	cmd.AddCommand(listCmd, getCmd)
	return cmd
}

func newConversationPropertiesCmd() *cobra.Command {
	listCmd := &cobra.Command{
		Use:   "list",
		Short: "List conversation properties",
		RunE: func(cmd *cobra.Command, args []string) error {
			params := url.Values{}
			params.Set("page", strconv.Itoa(page))
			params.Set("pageSize", strconv.Itoa(perPage))
			data, err := apiClient.ListConversationProperties(context.Background(), params)
			if err != nil {
				return err
			}
			return printPropertyList(data, "conversationProperties", "properties")
		},
	}

	getCmd := &cobra.Command{
		Use:   "get <id>",
		Short: "Get conversation property details",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			data, err := apiClient.GetConversationProperty(context.Background(), args[0])
			if err != nil {
				return err
			}
			return printSingleProperty(data)
		},
	}

	permission.Annotate(listCmd, "properties", permission.OpRead)
	permission.Annotate(getCmd, "properties", permission.OpRead)

	cmd := &cobra.Command{
		Use:   "conversations",
		Short: "Manage conversation properties",
	}
	cmd.AddCommand(listCmd, getCmd)
	return cmd
}

func printPropertyList(data json.RawMessage, keys ...string) error {
	items, err := extractEmbeddedWithCandidates(data, keys...)
	if err != nil {
		if isJSON() {
			return output.PrintRaw(data)
		}
		return err
	}
	if isJSON() {
		return output.PrintRaw(mustMarshal(items))
	}

	props := make([]types.Property, 0, len(items))
	for _, raw := range items {
		var p types.Property
		json.Unmarshal(raw, &p)
		props = append(props, p)
	}
	rows := make([]map[string]string, len(props))
	for i, p := range props {
		rows[i] = map[string]string{
			"id":   strconv.Itoa(p.ID),
			"name": p.Name,
			"type": p.Type,
		}
	}
	return output.Print(getFormat(), []string{"id", "name", "type"}, rows)
}

func printSingleProperty(data json.RawMessage) error {
	if isJSON() {
		return output.PrintRaw(data)
	}
	var p types.Property
	if err := json.Unmarshal(data, &p); err != nil {
		return fmt.Errorf("decoding property: %w", err)
	}
	return output.Print(getFormat(), []string{"id", "name", "type"}, []map[string]string{{
		"id":   strconv.Itoa(p.ID),
		"name": p.Name,
		"type": p.Type,
	}})
}
