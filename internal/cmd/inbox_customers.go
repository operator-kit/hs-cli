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

func newCustomersCmd() *cobra.Command {
	custCmd := &cobra.Command{
		Use:     "customers",
		Aliases: []string{"cust"},
		Short:   "Manage customers",
	}

	listCmd := customersListCmd()
	permission.Annotate(listCmd, "customers", permission.OpRead)
	listCmd.Flags().String("mailbox", "", "filter by mailbox ID")
	listCmd.Flags().String("first-name", "", "filter by first name")
	listCmd.Flags().String("last-name", "", "filter by last name")
	listCmd.Flags().String("modified-since", "", "filter by modified since timestamp")
	listCmd.Flags().String("sort-field", "", "sort field")
	listCmd.Flags().String("sort-order", "", "sort order")
	listCmd.Flags().String("query", "", "search query (e.g. email)")
	listCmd.Flags().String("embed", "", "embed resources (e.g. emails)")
	listCmd.Flags().MarkDeprecated("embed", "does not map to the Inbox API customers endpoint and will be removed")

	createCmd := customersCreateCmd()
	permission.Annotate(createCmd, "customers", permission.OpWrite)
	registerCustomerBodyFlags(createCmd)
	createCmd.MarkFlagRequired("first-name")

	updateCmd := customersUpdateCmd()
	permission.Annotate(updateCmd, "customers", permission.OpWrite)
	updateCmd.Flags().String("first-name", "", "first name")
	updateCmd.Flags().String("last-name", "", "last name")
	updateCmd.Flags().String("phone", "", "phone number")

	overwriteCmd := customersOverwriteCmd()
	permission.Annotate(overwriteCmd, "customers", permission.OpWrite)
	registerCustomerBodyFlags(overwriteCmd)
	overwriteCmd.MarkFlagRequired("first-name")

	deleteCmd := customersDeleteCmd()
	permission.Annotate(deleteCmd, "customers", permission.OpDelete)
	deleteCmd.Flags().Bool("async", false, "delete asynchronously (returns 202)")

	getCmd := customersGetCmd()
	permission.Annotate(getCmd, "customers", permission.OpRead)

	custCmd.AddCommand(listCmd, getCmd, createCmd, updateCmd, overwriteCmd, deleteCmd)
	return custCmd
}

// registerCustomerBodyFlags adds the shared flags for create and overwrite.
func registerCustomerBodyFlags(cmd *cobra.Command) {
	cmd.Flags().String("first-name", "", "first name")
	cmd.Flags().String("last-name", "", "last name")
	cmd.Flags().String("email", "", "email address")
	cmd.Flags().String("phone", "", "phone number")
	cmd.Flags().String("job-title", "", "job title")
	cmd.Flags().String("background", "", "background info")
	cmd.Flags().String("location", "", "location")
	cmd.Flags().String("gender", "", "gender (male, female, unknown)")
	cmd.Flags().String("age", "", "age")
	cmd.Flags().String("photo-url", "", "photo URL")
	cmd.Flags().Int("organization-id", 0, "organization ID")
	cmd.Flags().String("json", "", "raw JSON body (overrides all other flags)")
}

// customerBodyFromFlags builds a request body from flags or --json.
func customerBodyFromFlags(cmd *cobra.Command) (map[string]any, error) {
	if cmd.Flags().Changed("json") {
		raw, _ := cmd.Flags().GetString("json")
		return parseJSONBody(raw)
	}
	body := map[string]any{}
	if v, _ := cmd.Flags().GetString("first-name"); v != "" {
		body["firstName"] = v
	}
	if v, _ := cmd.Flags().GetString("last-name"); v != "" {
		body["lastName"] = v
	}
	if v, _ := cmd.Flags().GetString("email"); v != "" {
		body["emails"] = []map[string]string{{"type": "work", "value": v}}
	}
	if v, _ := cmd.Flags().GetString("phone"); v != "" {
		body["phone"] = v
	}
	if v, _ := cmd.Flags().GetString("job-title"); v != "" {
		body["jobTitle"] = v
	}
	if v, _ := cmd.Flags().GetString("background"); v != "" {
		body["background"] = v
	}
	if v, _ := cmd.Flags().GetString("location"); v != "" {
		body["location"] = v
	}
	if v, _ := cmd.Flags().GetString("gender"); v != "" {
		body["gender"] = v
	}
	if v, _ := cmd.Flags().GetString("age"); v != "" {
		body["age"] = v
	}
	if v, _ := cmd.Flags().GetString("photo-url"); v != "" {
		body["photoUrl"] = v
	}
	if v, _ := cmd.Flags().GetInt("organization-id"); v > 0 {
		body["organization"] = map[string]int{"id": v}
	}
	return body, nil
}

func customersListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List customers",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()
			params := url.Values{}
			params.Set("page", strconv.Itoa(page))
			params.Set("pageSize", strconv.Itoa(perPage))

			if v, _ := cmd.Flags().GetString("query"); v != "" {
				params.Set("query", v)
			}
			if v, _ := cmd.Flags().GetString("mailbox"); v != "" {
				params.Set("mailbox", v)
			}
			if v, _ := cmd.Flags().GetString("first-name"); v != "" {
				params.Set("firstName", v)
			}
			if v, _ := cmd.Flags().GetString("last-name"); v != "" {
				params.Set("lastName", v)
			}
			if v, _ := cmd.Flags().GetString("modified-since"); v != "" {
				params.Set("modifiedSince", v)
			}
			if v, _ := cmd.Flags().GetString("sort-field"); v != "" {
				params.Set("sortField", v)
			}
			if v, _ := cmd.Flags().GetString("sort-order"); v != "" {
				params.Set("sortOrder", v)
			}

			if isJSON() {
				items, _, err := api.PaginateAll(ctx, apiClient.ListCustomers, params, "customers", noPaginate)
				if err != nil {
					return err
				}
				if !isJSONClean() {
					return printRawWithPII(mustMarshal(items))
				}
				return printRawWithPII(mustMarshal(cleanRawItems(items, cleanCustomer)))
			}

			items, pageInfo, err := api.PaginateAll(ctx, apiClient.ListCustomers, params, "customers", noPaginate)
			if err != nil {
				return err
			}
			engine, err := newPIIEngine()
			if err != nil {
				return err
			}

			var customers []types.Customer
			for _, raw := range items {
				var c types.Customer
				json.Unmarshal(raw, &c)
				redactCustomerForOutput(engine, &c)
				customers = append(customers, c)
			}

			cols := []string{"id", "first_name", "last_name", "email", "created"}
			rows := make([]map[string]string, len(customers))
			for i, c := range customers {
				rows[i] = map[string]string{
					"id":         strconv.Itoa(c.ID),
					"first_name": c.FirstName,
					"last_name":  c.LastName,
					"email":      c.Email,
					"created":    c.CreatedAt,
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

func customersGetCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "get <id>",
		Short: "Get customer details",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			data, err := apiClient.GetCustomer(context.Background(), args[0], url.Values{})
			if err != nil {
				return err
			}

			if isJSON() {
				if !isJSONClean() {
					return printRawWithPII(data)
				}
				return printRawWithPII(mustMarshal(cleanRawObject(data, cleanCustomer)))
			}

			var c types.Customer
			json.Unmarshal(data, &c)
			engine, err := newPIIEngine()
			if err != nil {
				return err
			}
			redactCustomerForOutput(engine, &c)

			cols := []string{"id", "first_name", "last_name", "email", "phone", "created"}
			rows := []map[string]string{{
				"id":         strconv.Itoa(c.ID),
				"first_name": c.FirstName,
				"last_name":  c.LastName,
				"email":      c.Email,
				"phone":      c.Phone,
				"created":    c.CreatedAt,
			}}
			return output.Print(getFormat(), cols, rows)
		},
	}
	cmd.Flags().String("embed", "", "embed resources (deprecated)")
	cmd.Flags().MarkDeprecated("embed", "does not map to the Inbox API customers endpoint and will be removed")
	return cmd
}

func customersCreateCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "create",
		Short: "Create a customer",
		RunE: func(cmd *cobra.Command, args []string) error {
			body, err := customerBodyFromFlags(cmd)
			if err != nil {
				return fmt.Errorf("invalid --json payload: %w", err)
			}

			id, err := apiClient.CreateCustomer(context.Background(), body)
			if err != nil {
				return err
			}
			fmt.Fprintf(output.Out, "Created customer %s\n", id)
			return nil
		},
	}
}

func customersUpdateCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "update <id>",
		Short: "Patch a customer",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			body := make([]jsonPatchOp, 0, 3)
			changed := false
			if v, _ := cmd.Flags().GetString("first-name"); v != "" {
				body = append(body, jsonPatchOp{
					Op:    "replace",
					Path:  "/firstName",
					Value: v,
				})
				changed = true
			}
			if v, _ := cmd.Flags().GetString("last-name"); v != "" {
				body = append(body, jsonPatchOp{
					Op:    "replace",
					Path:  "/lastName",
					Value: v,
				})
				changed = true
			}
			if v, _ := cmd.Flags().GetString("phone"); v != "" {
				body = append(body, jsonPatchOp{
					Op:   "add",
					Path: "/phones",
					Value: map[string]string{
						"type":  "work",
						"value": v,
					},
				})
				changed = true
			}
			if !changed {
				return fmt.Errorf("no fields to update")
			}
			if err := apiClient.UpdateCustomer(context.Background(), args[0], body); err != nil {
				return err
			}
			fmt.Fprintf(output.Out, "Updated customer %s\n", args[0])
			return nil
		},
	}
}

func customersOverwriteCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "overwrite <id>",
		Short: "Overwrite a customer",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			body, err := customerBodyFromFlags(cmd)
			if err != nil {
				return fmt.Errorf("invalid --json payload: %w", err)
			}

			if err := apiClient.OverwriteCustomer(context.Background(), args[0], body); err != nil {
				return err
			}
			fmt.Fprintf(output.Out, "Overwrote customer %s\n", args[0])
			return nil
		},
	}
}

func customersDeleteCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "delete <id>",
		Short: "Delete a customer",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			params := url.Values{}
			if async, _ := cmd.Flags().GetBool("async"); async {
				params.Set("async", "true")
			}
			if err := apiClient.DeleteCustomer(context.Background(), args[0], params); err != nil {
				return err
			}
			fmt.Fprintf(output.Out, "Deleted customer %s\n", args[0])
			return nil
		},
	}
}
