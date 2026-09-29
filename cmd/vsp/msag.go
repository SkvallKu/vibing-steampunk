package main

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/oisee/vibing-steampunk/pkg/adt"
)

var msagCmd = &cobra.Command{
	Use:   "msag",
	Short: "Message classes (SE91): read, create, set and delete messages",
	Long: `A message class over ADT (/sap/bc/adt/messageclass).

  vsp msag get ZDEMO
  vsp msag create ZDEMO --package ZPKG --description "Demo messages" [--transport TR-EXAMPLE]
  vsp msag set ZDEMO 001="Enter a date" 002="No authorization for &1"
  vsp msag set ZDEMO --delete 003,004 --dry-run

Texts are written in the logon language, which must be the class's master
language. Messages not named keep their texts. A write shows what is added,
what changes from what, and what is deleted; nothing is locked when nothing
differs.`,
}

var msagGetCmd = &cobra.Command{
	Use:   "get <NAME>",
	Short: "Read a message class",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, _, err := docsClient(cmd)
		if err != nil {
			return err
		}
		mc, err := client.GetMessageClass(context.Background(), args[0])
		if err != nil {
			return err
		}
		if asJSON, _ := cmd.Flags().GetBool("json"); asJSON {
			return printJSON(mc)
		}
		fmt.Printf("%s  %s  (package %s, master language %s)\n", mc.Name, mc.Description, mc.Package, mc.MasterLanguage)
		for _, m := range mc.Messages {
			fmt.Printf("%s  %s\n", m.Number, m.Text)
		}
		return nil
	},
}

var msagCreateCmd = &cobra.Command{
	Use:   "create <NAME>",
	Short: "Create an empty message class",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, _, err := docsClient(cmd)
		if err != nil {
			return err
		}
		pkg, _ := cmd.Flags().GetString("package")
		desc, _ := cmd.Flags().GetString("description")
		if pkg == "" || desc == "" {
			return fmt.Errorf("--package and --description are required")
		}
		transport, _ := cmd.Flags().GetString("transport")
		var chosen adt.TransportChoice
		err = client.CreateObject(context.Background(), adt.CreateObjectOptions{
			ObjectType:  adt.ObjectTypeMessageClass,
			Name:        args[0],
			Description: desc,
			PackageName: pkg,
			Transport:   transport,
			Chosen:      &chosen,
		})
		if err != nil {
			return err
		}
		if transport == "" {
			transport = chosen.Transport
		}
		fmt.Fprintf(os.Stderr, "created message class %s in %s", strings.ToUpper(args[0]), strings.ToUpper(pkg))
		if transport != "" {
			fmt.Fprintf(os.Stderr, ", transport %s", transport)
		}
		fmt.Fprintln(os.Stderr)
		if chosen.Reason != "" {
			fmt.Fprintln(os.Stderr, chosen.Reason)
		}
		return nil
	},
}

var msagSetCmd = &cobra.Command{
	Use:   "set <NAME> [NNN=TEXT ...]",
	Short: "Set and delete messages; numbers not named keep theirs",
	Args:  cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		set := map[string]string{}
		for _, kv := range args[1:] {
			k, v, ok := strings.Cut(kv, "=")
			if !ok || strings.TrimSpace(k) == "" {
				return fmt.Errorf("%q: want NNN=TEXT", kv)
			}
			set[strings.TrimSpace(k)] = v
		}
		del, _ := cmd.Flags().GetStringSlice("delete")
		client, _, err := docsClient(cmd)
		if err != nil {
			return err
		}
		transport, _ := cmd.Flags().GetString("transport")
		dryRun, _ := cmd.Flags().GetBool("dry-run")
		edit, err := client.EditMessageClass(context.Background(), args[0], set, del, transport, dryRun)
		if asJSON, _ := cmd.Flags().GetBool("json"); asJSON && edit != nil {
			if perr := printJSON(edit); perr != nil {
				return perr
			}
			return err
		}
		if edit != nil {
			printMessageEdit(edit)
		}
		return err
	},
}

func printMessageEdit(e *adt.MessageClassEdit) {
	for _, a := range e.Added {
		fmt.Printf("+ %s  %s\n", a.Number, a.Text)
	}
	for _, c := range e.Changed {
		fmt.Printf("~ %s  %s  (was: %s)\n", c.Number, c.New, c.Old)
	}
	for _, d := range e.Deleted {
		fmt.Printf("- %s  deleted\n", d)
	}
	for _, u := range e.Unchanged {
		fmt.Printf("= %s  unchanged\n", u)
	}
	for _, a := range e.Absent {
		fmt.Printf("? %s  not in the class, nothing to delete\n", a)
	}
	for _, n := range e.Notes {
		fmt.Fprintln(os.Stderr, n)
	}
	n := len(e.Added) + len(e.Changed) + len(e.Deleted)
	switch {
	case e.Applied:
		fmt.Fprintf(os.Stderr, "%d message(s) written to %s (%s", n, e.Name, e.Language)
		if e.Transport != "" {
			fmt.Fprintf(os.Stderr, ", transport %s", e.Transport)
		}
		fmt.Fprintln(os.Stderr, ")")
	case n == 0:
		fmt.Fprintln(os.Stderr, "nothing written")
	}
}

func init() {
	for _, c := range []*cobra.Command{msagGetCmd, msagSetCmd} {
		c.Flags().Bool("json", false, "Emit JSON")
	}
	for _, c := range []*cobra.Command{msagCreateCmd, msagSetCmd} {
		c.Flags().String("transport", "", "Transport request; chosen like the editor would when empty")
	}
	msagCreateCmd.Flags().String("package", "", "Package")
	msagCreateCmd.Flags().String("description", "", "Short text")
	msagSetCmd.Flags().StringSlice("delete", nil, "Message numbers to delete")
	msagSetCmd.Flags().Bool("dry-run", false, "Show the plan and write nothing")
	msagCmd.AddCommand(msagGetCmd, msagCreateCmd, msagSetCmd)
	rootCmd.AddCommand(msagCmd)
}
