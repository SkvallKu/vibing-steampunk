package main

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/oisee/vibing-steampunk/pkg/adt"
	"github.com/spf13/cobra"
)

var atcVariantCmd = &cobra.Command{
	Use:   "variant",
	Short: "Code Inspector check variants for ATC: read, write, delete",
	Long: `The global Code Inspector (SCI) check variant that ATC runs by name
(vsp atc ... --variant NAME) on releases without ATC check variant objects.
No ADT resource serves them there; the adt-fallback set (0.10.0 or later)
stands in.

  vsp atc variant get ZVSP_LOGIC > zvsp_logic.chkv.txt
  vsp atc variant set ZVSP_LOGIC --file zvsp_logic.chkv.txt
  vsp atc variant delete ZVSP_LOGIC
  vsp atc checks                       # the checks and their parameters

The document is text, fields separated by TAB:

  description	Logic checks
  check	CL_CI_TEST_SYSUBRC
  param	CL_CI_TEST_SYSUBRC	SELECT	X
  param	CL_CI_TEST_SYSUBRC	ANY	READ TABLE
  check	CL_CI_TEST_NO_ORDER_BY

set creates the variant or replaces its checks; a check's parameters not
named keep the check's own value (vsp atc checks shows them). A table
parameter takes one param line per row. Only variants in the customer
namespace (Z*, Y*) that are not transported are written or deleted, and
a new one is saved without transport.`,
}

func init() {
	get := &cobra.Command{
		Use: "get <NAME>", Short: "Read a check variant", Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, _, err := docsClient(cmd)
			if err != nil {
				return err
			}
			v, err := client.GetCheckVariant(context.Background(), args[0])
			if err != nil {
				return err
			}
			return printCheckVariant(cmd, v)
		},
	}

	set := &cobra.Command{
		Use: "set <NAME>", Short: "Create a check variant or replace its checks", Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			file, _ := cmd.Flags().GetString("file")
			if file == "" {
				return fmt.Errorf("--file is required: the variant in the line format (see vsp atc variant --help)")
			}
			doc, err := os.ReadFile(file)
			if err != nil {
				return err
			}
			client, _, err := docsClient(cmd)
			if err != nil {
				return err
			}
			v, err := client.WriteCheckVariant(context.Background(), args[0], string(doc))
			if err != nil {
				return err
			}
			return printCheckVariant(cmd, v)
		},
	}
	set.Flags().String("file", "", "The variant in the line format")

	del := &cobra.Command{
		Use: "delete <NAME>", Short: "Delete a check variant", Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, _, err := docsClient(cmd)
			if err != nil {
				return err
			}
			if err := client.DeleteCheckVariant(context.Background(), args[0]); err != nil {
				return err
			}
			fmt.Printf("deleted %s\n", strings.ToUpper(args[0]))
			return nil
		},
	}

	checks := &cobra.Command{
		Use: "checks [CLASS]", Short: "List the Code Inspector checks and their default parameters", Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, _, err := docsClient(cmd)
			if err != nil {
				return err
			}
			filter := ""
			if len(args) == 1 {
				filter = args[0]
			}
			list, err := client.ListCodeInspectorChecks(context.Background(), filter)
			if err != nil {
				return err
			}
			if asJSON, _ := cmd.Flags().GetBool("json"); asJSON {
				return printJSON(list)
			}
			for _, ch := range list {
				fmt.Printf("%-34s %s  %s\n", ch.Class, ch.Version, ch.Title)
				if len(args) == 1 {
					for _, p := range ch.Params {
						fmt.Printf("    %-16s %s\n", p.Name, strings.Join(p.Values, " | "))
					}
				}
			}
			return nil
		},
	}

	for _, c := range []*cobra.Command{get, set, del, checks} {
		c.Flags().Bool("json", false, "Emit JSON")
	}
	atcVariantCmd.AddCommand(get, set, del)
	atcCmd.AddCommand(atcVariantCmd, checks)
}

func printCheckVariant(cmd *cobra.Command, v *adt.CheckVariant) error {
	if asJSON, _ := cmd.Flags().GetBool("json"); asJSON {
		return printJSON(v)
	}
	fmt.Print(v.Document)
	if v.Document != "" && !strings.HasSuffix(v.Document, "\n") {
		fmt.Println()
	}
	return nil
}
