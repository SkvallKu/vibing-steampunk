package main

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

var cuaCmd = &cobra.Command{
	Use:   "cua",
	Short: "GUI statuses and titles of a program (SE41): read, write",
	Long: `The CUA of a program — its GUI statuses and titles — in abapGit's format:
the CUA node of a .prog.xml (ADM, and the tables STA, FUN, MEN, MTX, ACT,
BUT, PFK, SET, DOC, TIT, BIV). No release has an ADT resource for it; the
adt-fallback set (0.9.0 or later) stands in.

  vsp cua get ZDEMO_REPORT > zdemo_report.cua.xml
  vsp cua get ZDEMO_REPORT --summary
  vsp cua set ZDEMO_REPORT --file src/zdemo_report.prog.xml --transport TR-EXAMPLE

A write replaces the whole CUA of the program and activates it; read it
first, change it, put it back. --file takes a whole abapGit .prog.xml (only
its CUA node is used), the output of get, or a bare <CUA> element. Statuses
link their keys through codes (ADM.PFKCODE = STA.PFKCODE = PFK.CODE, a key
set code like 000001, not a key number); links that do not hold are
reported as warnings, since SAP writes them without a word and the status
then shows no keys.`,
}

func init() {
	cuaGet := &cobra.Command{
		Use: "get <PROGRAM>", Short: "Read the GUI statuses and titles", Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, _, err := docsClient(cmd)
			if err != nil {
				return err
			}
			version, _ := cmd.Flags().GetString("version")
			cua, err := client.GetProgramCUA(context.Background(), args[0], version)
			if err != nil {
				return err
			}
			if asJSON, _ := cmd.Flags().GetBool("json"); asJSON {
				return printJSON(cua)
			}
			if summary, _ := cmd.Flags().GetBool("summary"); summary {
				fmt.Printf("program   %s\nstatuses  %s\ntitles    %s\n", cua.Program,
					orNone(strings.Join(cua.Statuses, ", ")), orNone(strings.Join(cua.Titles, ", ")))
				return nil
			}
			fmt.Println(cua.Document)
			return nil
		},
	}
	cuaGet.Flags().String("version", "", "active or inactive; the newest you may see when empty")
	cuaGet.Flags().Bool("summary", false, "List the statuses and titles only")

	cuaSet := &cobra.Command{
		Use: "set <PROGRAM>", Short: "Replace the GUI statuses and titles, and activate them", Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			file, _ := cmd.Flags().GetString("file")
			if file == "" {
				return fmt.Errorf("--file is required: an abapGit .prog.xml, the output of cua get, or a <CUA> element")
			}
			doc, err := os.ReadFile(file)
			if err != nil {
				return err
			}
			client, _, err := docsClient(cmd)
			if err != nil {
				return err
			}
			transport, _ := cmd.Flags().GetString("transport")
			w, err := client.WriteProgramCUA(context.Background(), args[0], string(doc), transport)
			if asJSON, _ := cmd.Flags().GetBool("json"); asJSON && w != nil {
				if perr := printJSON(w); perr != nil {
					return perr
				}
				return err
			}
			if w != nil {
				fmt.Printf("program    %s\nstatuses   %s\ntitles     %s\n", w.Program,
					orNone(strings.Join(w.Statuses, ", ")), orNone(strings.Join(w.Titles, ", ")))
				if w.Transport != "" {
					fmt.Printf("transport  %s\n", w.Transport)
				}
				fmt.Printf("activated  %v\n", w.Activated)
				for _, s := range w.Warnings {
					fmt.Printf("warning    %s\n", s)
				}
				for _, s := range w.Notes {
					fmt.Printf("note       %s\n", s)
				}
			}
			return err
		},
	}
	cuaSet.Flags().String("file", "", "The document: an abapGit .prog.xml, the output of cua get, or a <CUA> element")
	cuaSet.Flags().String("transport", "", "Transport request; chosen like the editor would when empty")

	cuaCmd.AddCommand(cuaGet, cuaSet)
	for _, c := range []*cobra.Command{cuaGet, cuaSet} {
		c.Flags().Bool("json", false, "Emit JSON")
	}
	rootCmd.AddCommand(cuaCmd)
}

func orNone(s string) string {
	if s == "" {
		return "(none)"
	}
	return s
}
