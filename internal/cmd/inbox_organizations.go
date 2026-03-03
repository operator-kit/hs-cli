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

func newOrganizationsCmd() *cobra.Command {
	listCmd := &cobra.Command{
		Use:   "list",
		Short: "List organizations",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()
			params := url.Values{}
			params.Set("page", strconv.Itoa(page))
			params.Set("pageSize", strconv.Itoa(perPage))
			if q, _ := cmd.Flags().GetString("query"); q != "" {
				params.Set("query", q)
			}

			if isJSON() {
				items, _, err := api.PaginateAll(ctx, apiClient.ListOrganizations, params, "organizations", noPaginate)
				if err != nil {
					return err
				}
				if !isJSONClean() {
					return printRawWithPII(mustMarshal(items))
				}
				return printRawWithPII(mustMarshal(cleanRawItems(items, cleanOrganization)))
			}

			items, _, err := api.PaginateAll(ctx, apiClient.ListOrganizations, params, "organizations", noPaginate)
			if err != nil {
				return err
			}
			orgs := make([]types.Organization, 0, len(items))
			for _, raw := range items {
				var org types.Organization
				json.Unmarshal(raw, &org)
				orgs = append(orgs, org)
			}
			rows := make([]map[string]string, len(orgs))
			for i, org := range orgs {
				rows[i] = map[string]string{
					"id":     strconv.Itoa(org.ID),
					"name":   org.Name,
					"domain": org.Domain,
				}
			}
			return output.Print(getFormat(), []string{"id", "name", "domain"}, rows)
		},
	}
	permission.Annotate(listCmd, "organizations", permission.OpRead)
	listCmd.Flags().String("query", "", "search query")

	getCmd := &cobra.Command{
		Use:   "get <id>",
		Short: "Get organization details",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			data, err := apiClient.GetOrganization(context.Background(), args[0])
			if err != nil {
				return err
			}
			if isJSON() {
				if !isJSONClean() {
					return printRawWithPII(data)
				}
				return printRawWithPII(mustMarshal(cleanRawObject(data, cleanOrganization)))
			}

			var org types.Organization
			json.Unmarshal(data, &org)
			return output.Print(getFormat(), []string{"id", "name", "domain"}, []map[string]string{{
				"id":     strconv.Itoa(org.ID),
				"name":   org.Name,
				"domain": org.Domain,
			}})
		},
	}

	createCmd := &cobra.Command{
		Use:   "create",
		Short: "Create an organization",
		RunE: func(cmd *cobra.Command, args []string) error {
			body, err := organizationBodyFromFlags(cmd, true)
			if err != nil {
				return err
			}
			id, err := apiClient.CreateOrganization(context.Background(), body)
			if err != nil {
				return err
			}
			fmt.Fprintf(output.Out, "Created organization %s\n", id)
			return nil
		},
	}
	permission.Annotate(getCmd, "organizations", permission.OpRead)
	permission.Annotate(createCmd, "organizations", permission.OpWrite)
	createCmd.Flags().String("name", "", "organization name (required)")
	createCmd.Flags().String("domain", "", "organization domain")
	createCmd.Flags().String("json", "", "full organization payload as JSON object")
	createCmd.MarkFlagRequired("name")

	updateCmd := &cobra.Command{
		Use:   "update <id>",
		Short: "Update an organization",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			body, err := organizationBodyFromFlags(cmd, false)
			if err != nil {
				return err
			}
			if len(body) == 0 {
				return fmt.Errorf("no fields to update")
			}
			if err := apiClient.UpdateOrganization(context.Background(), args[0], body); err != nil {
				return err
			}
			fmt.Fprintf(output.Out, "Updated organization %s\n", args[0])
			return nil
		},
	}
	permission.Annotate(updateCmd, "organizations", permission.OpWrite)
	updateCmd.Flags().String("name", "", "organization name")
	updateCmd.Flags().String("domain", "", "organization domain")
	updateCmd.Flags().String("json", "", "full organization payload as JSON object")

	deleteCmd := &cobra.Command{
		Use:   "delete <id>",
		Short: "Delete an organization",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := apiClient.DeleteOrganization(context.Background(), args[0]); err != nil {
				return err
			}
			fmt.Fprintf(output.Out, "Deleted organization %s\n", args[0])
			return nil
		},
	}

	permission.Annotate(deleteCmd, "organizations", permission.OpDelete)

	cmd := &cobra.Command{
		Use:   "organizations",
		Short: "Manage organizations",
	}
	cmd.AddCommand(
		listCmd,
		getCmd,
		createCmd,
		updateCmd,
		deleteCmd,
		newOrganizationConversationsCmd(),
		newOrganizationCustomersCmd(),
		newOrganizationPropertiesCmd(),
	)
	return cmd
}

func newOrganizationConversationsCmd() *cobra.Command {
	listCmd := &cobra.Command{
		Use:   "list <organization-id>",
		Short: "List conversations for an organization",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()
			params := url.Values{}
			params.Set("page", strconv.Itoa(page))
			params.Set("pageSize", strconv.Itoa(perPage))
			fetch := func(ctx context.Context, p url.Values) (json.RawMessage, error) {
				return apiClient.ListOrganizationConversations(ctx, args[0], p)
			}

			if isJSON() {
				items, _, err := api.PaginateAll(ctx, fetch, params, "conversations", noPaginate)
				if err != nil {
					return err
				}
				if !isJSONClean() {
					return printRawWithPII(mustMarshal(items))
				}
				return printRawWithPII(mustMarshal(cleanRawItems(items, cleanConversation)))
			}

			items, _, err := api.PaginateAll(ctx, fetch, params, "conversations", noPaginate)
			if err != nil {
				return err
			}
			engine, err := newPIIEngine()
			if err != nil {
				return err
			}
			rows := make([]map[string]string, len(items))
			for i, raw := range items {
				var c types.Conversation
				json.Unmarshal(raw, &c)
				if engine.Enabled() {
					c.Subject = redactTextWithPII(engine, c.Subject, knownFromPerson(c.PrimaryCustomer, "customer"))
				}
				rows[i] = map[string]string{
					"id":      strconv.Itoa(c.ID),
					"number":  strconv.Itoa(c.Number),
					"subject": c.Subject,
					"status":  c.Status,
				}
			}
			return output.Print(getFormat(), []string{"id", "number", "subject", "status"}, rows)
		},
	}
	permission.Annotate(listCmd, "organizations", permission.OpRead)

	cmd := &cobra.Command{
		Use:   "conversations",
		Short: "Manage organization conversations",
	}
	cmd.AddCommand(listCmd)
	return cmd
}

func newOrganizationCustomersCmd() *cobra.Command {
	listCmd := &cobra.Command{
		Use:   "list <organization-id>",
		Short: "List customers for an organization",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()
			params := url.Values{}
			params.Set("page", strconv.Itoa(page))
			params.Set("pageSize", strconv.Itoa(perPage))
			fetch := func(ctx context.Context, p url.Values) (json.RawMessage, error) {
				return apiClient.ListOrganizationCustomers(ctx, args[0], p)
			}

			if isJSON() {
				items, _, err := api.PaginateAll(ctx, fetch, params, "customers", noPaginate)
				if err != nil {
					return err
				}
				if !isJSONClean() {
					return printRawWithPII(mustMarshal(items))
				}
				return printRawWithPII(mustMarshal(cleanRawItems(items, cleanCustomer)))
			}

			items, _, err := api.PaginateAll(ctx, fetch, params, "customers", noPaginate)
			if err != nil {
				return err
			}
			engine, err := newPIIEngine()
			if err != nil {
				return err
			}
			rows := make([]map[string]string, len(items))
			for i, raw := range items {
				var c types.Customer
				json.Unmarshal(raw, &c)
				redactCustomerForOutput(engine, &c)
				rows[i] = map[string]string{
					"id":    strconv.Itoa(c.ID),
					"email": c.Email,
					"name":  fmt.Sprintf("%s %s", c.FirstName, c.LastName),
				}
			}
			return output.Print(getFormat(), []string{"id", "name", "email"}, rows)
		},
	}
	permission.Annotate(listCmd, "organizations", permission.OpRead)

	cmd := &cobra.Command{
		Use:   "customers",
		Short: "Manage organization customers",
	}
	cmd.AddCommand(listCmd)
	return cmd
}

func newOrganizationPropertiesCmd() *cobra.Command {
	listCmd := &cobra.Command{
		Use:   "list",
		Short: "List organization properties",
		RunE: func(cmd *cobra.Command, args []string) error {
			params := url.Values{}
			params.Set("page", strconv.Itoa(page))
			params.Set("pageSize", strconv.Itoa(perPage))
			data, err := apiClient.ListOrganizationProperties(context.Background(), params)
			if err != nil {
				return err
			}

			items, err := extractEmbeddedWithCandidates(data, "properties", "organizationProperties")
			if err != nil {
				if isJSON() {
					return printRawWithPII(data)
				}
				return err
			}
			if isJSON() {
				return printRawWithPII(mustMarshal(items))
			}

			props := make([]types.OrganizationProperty, 0, len(items))
			for _, raw := range items {
				var p types.OrganizationProperty
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
		},
	}

	getCmd := &cobra.Command{
		Use:   "get <id>",
		Short: "Get organization property details",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			data, err := apiClient.GetOrganizationProperty(context.Background(), args[0])
			if err != nil {
				return err
			}
			if isJSON() {
				return printRawWithPII(data)
			}
			var p types.OrganizationProperty
			json.Unmarshal(data, &p)
			return output.Print(getFormat(), []string{"id", "name", "type"}, []map[string]string{{
				"id":   strconv.Itoa(p.ID),
				"name": p.Name,
				"type": p.Type,
			}})
		},
	}

	createCmd := &cobra.Command{
		Use:   "create",
		Short: "Create an organization property",
		RunE: func(cmd *cobra.Command, args []string) error {
			body, err := organizationPropertyBodyFromFlags(cmd, true)
			if err != nil {
				return err
			}
			id, err := apiClient.CreateOrganizationProperty(context.Background(), body)
			if err != nil {
				return err
			}
			fmt.Fprintf(output.Out, "Created organization property %s\n", id)
			return nil
		},
	}
	createCmd.Flags().String("name", "", "property name (required)")
	createCmd.Flags().String("type", "", "property type")
	createCmd.Flags().String("json", "", "full property payload as JSON object")
	createCmd.MarkFlagRequired("name")

	updateCmd := &cobra.Command{
		Use:   "update <id>",
		Short: "Update an organization property",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			body, err := organizationPropertyBodyFromFlags(cmd, false)
			if err != nil {
				return err
			}
			if len(body) == 0 {
				return fmt.Errorf("no fields to update")
			}
			if err := apiClient.UpdateOrganizationProperty(context.Background(), args[0], body); err != nil {
				return err
			}
			fmt.Fprintf(output.Out, "Updated organization property %s\n", args[0])
			return nil
		},
	}
	updateCmd.Flags().String("name", "", "property name")
	updateCmd.Flags().String("type", "", "property type")
	updateCmd.Flags().String("json", "", "full property payload as JSON object")

	deleteCmd := &cobra.Command{
		Use:   "delete <id>",
		Short: "Delete an organization property",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := apiClient.DeleteOrganizationProperty(context.Background(), args[0]); err != nil {
				return err
			}
			fmt.Fprintf(output.Out, "Deleted organization property %s\n", args[0])
			return nil
		},
	}

	permission.Annotate(listCmd, "organizations", permission.OpRead)
	permission.Annotate(getCmd, "organizations", permission.OpRead)
	permission.Annotate(createCmd, "organizations", permission.OpWrite)
	permission.Annotate(updateCmd, "organizations", permission.OpWrite)
	permission.Annotate(deleteCmd, "organizations", permission.OpDelete)

	cmd := &cobra.Command{
		Use:   "properties",
		Short: "Manage organization properties",
	}
	cmd.AddCommand(listCmd, getCmd, createCmd, updateCmd, deleteCmd)
	return cmd
}

func organizationBodyFromFlags(cmd *cobra.Command, isCreate bool) (map[string]any, error) {
	if raw, _ := cmd.Flags().GetString("json"); raw != "" {
		return parseJSONBody(raw)
	}
	body := map[string]any{}
	if v, _ := cmd.Flags().GetString("name"); v != "" {
		body["name"] = v
	}
	if v, _ := cmd.Flags().GetString("domain"); v != "" {
		body["domain"] = v
	}
	if isCreate {
		if _, ok := body["name"]; !ok {
			return nil, fmt.Errorf("--name is required")
		}
	}
	return body, nil
}

func organizationPropertyBodyFromFlags(cmd *cobra.Command, isCreate bool) (map[string]any, error) {
	if raw, _ := cmd.Flags().GetString("json"); raw != "" {
		return parseJSONBody(raw)
	}
	body := map[string]any{}
	if v, _ := cmd.Flags().GetString("name"); v != "" {
		body["name"] = v
	}
	if v, _ := cmd.Flags().GetString("type"); v != "" {
		body["type"] = v
	}
	if isCreate {
		if _, ok := body["name"]; !ok {
			return nil, fmt.Errorf("--name is required")
		}
	}
	return body, nil
}
