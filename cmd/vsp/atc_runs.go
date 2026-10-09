package main

import (
	"context"
	"fmt"
	"strings"

	"github.com/oisee/vibing-steampunk/pkg/adt"
	"github.com/spf13/cobra"
)

var atcRunsCmd = &cobra.Command{
	Use:   "runs",
	Short: "Your ATC runs and worklists: list, delete",
	Long: `What ATC keeps of your runs: per run its results (3 days), execution
(6 days) and project (17 days), and from 7.50 on the worklist it was merged
into (7 days). ATC's garbage collector removes only what has expired, and
not on every system; ADT cannot delete any of it. The adt-fallback set
(0.11.0 or later) lists and deletes them: only your own, in this system
and client, never a run of a run series or a central run. vsp atc never
deletes on its own.

  vsp atc runs                                  # all of yours
  vsp atc runs --variant ZVSP_TEST* --from 2026-10-09
  vsp atc runs delete --variant ZVSP_TEST* --dry-run
  vsp atc runs delete --variant ZVSP_TEST*
  vsp atc runs delete 0A1B2C3D4E5F6071829A39213D9B1D5B
  vsp atc runs delete --all

--variant and --object take patterns with *; a worklist matches --object
when one of its objects does. --from and --to take a local date or date
and time (2026-10-09, 2026-10-09T11:08) or a UTC time stamp
(20261009110800).`,
	RunE: func(cmd *cobra.Command, args []string) error {
		client, _, err := docsClient(cmd)
		if err != nil {
			return err
		}
		list, err := listATCData(cmd, client)
		if err != nil {
			return err
		}
		if asJSON, _ := cmd.Flags().GetBool("json"); asJSON {
			return printJSON(list)
		}
		for _, d := range list {
			fmt.Println(formatATCData(d))
		}
		if len(list) == 0 {
			fmt.Println("no ATC runs or worklists of yours match")
		}
		return nil
	},
}

func init() {
	del := &cobra.Command{
		Use:   "delete [ID...]",
		Short: "Delete your ATC runs and worklists, by ID or by the filters",
		RunE: func(cmd *cobra.Command, args []string) error {
			all, _ := cmd.Flags().GetBool("all")
			dryRun, _ := cmd.Flags().GetBool("dry-run")
			if len(args) == 0 && !all && !atcDataFiltered(cmd) {
				return fmt.Errorf("name the IDs, narrow with --variant, --object, --from, --to or --kind, or say --all")
			}
			if len(args) > 0 && (all || atcDataFiltered(cmd)) {
				return fmt.Errorf("give either IDs or filters, not both")
			}
			client, _, err := docsClient(cmd)
			if err != nil {
				return err
			}
			list, err := listATCData(cmd, client)
			if err != nil {
				return err
			}
			if len(args) > 0 {
				byID := map[string]adt.ATCData{}
				for _, d := range list {
					byID[d.ID] = d
				}
				var picked []adt.ATCData
				var missing []string
				for _, id := range args {
					d, ok := byID[strings.ToUpper(strings.TrimSpace(id))]
					if !ok {
						missing = append(missing, id)
						continue
					}
					picked = append(picked, d)
				}
				if len(missing) > 0 {
					return fmt.Errorf("not among your ATC runs and worklists: %s", strings.Join(missing, ", "))
				}
				list = picked
			}
			if len(list) == 0 {
				fmt.Println("no ATC runs or worklists of yours match")
				return nil
			}
			failed := 0
			for _, d := range list {
				if dryRun {
					fmt.Printf("would delete %s\n", formatATCData(d))
					continue
				}
				lines, err := client.DeleteATCData(context.Background(), d.Kind, d.ID)
				if err != nil {
					failed++
					fmt.Printf("FAILED  %s: %v\n", formatATCData(d), err)
					continue
				}
				fmt.Printf("deleted %s\n", formatATCData(d))
				for _, line := range lines {
					fmt.Printf("        %s\n", line)
				}
			}
			if failed > 0 {
				return fmt.Errorf("%d of %d not deleted", failed, len(list))
			}
			return nil
		},
	}
	del.Flags().Bool("all", false, "Delete all your ATC runs and worklists")
	del.Flags().Bool("dry-run", false, "Show what would be deleted, delete nothing")

	for _, c := range []*cobra.Command{atcRunsCmd, del} {
		c.Flags().String("variant", "", "Check variant, pattern with *")
		c.Flags().String("object", "", "Object (a run's title, a worklist's objects), pattern with *")
		c.Flags().String("from", "", "Scheduled or changed at or after: local date or date and time, or UTC YYYYMMDDhhmmss")
		c.Flags().String("to", "", "Scheduled or changed at or before: as --from")
		c.Flags().String("kind", "", "run or worklist; both when empty")
	}
	atcRunsCmd.Flags().Bool("json", false, "Emit JSON")
	atcRunsCmd.AddCommand(del)
	atcCmd.AddCommand(atcRunsCmd)
}

func atcDataFiltered(cmd *cobra.Command) bool {
	for _, name := range []string{"variant", "object", "from", "to", "kind"} {
		if v, _ := cmd.Flags().GetString(name); strings.TrimSpace(v) != "" {
			return true
		}
	}
	return false
}

func listATCData(cmd *cobra.Command, client *adt.Client) ([]adt.ATCData, error) {
	var f adt.ATCDataFilter
	var err error
	f.Variant, _ = cmd.Flags().GetString("variant")
	f.Title, _ = cmd.Flags().GetString("object")
	from, _ := cmd.Flags().GetString("from")
	to, _ := cmd.Flags().GetString("to")
	if f.From, err = adt.ATCTimestamp(from); err != nil {
		return nil, err
	}
	if f.To, err = adt.ATCTimestamp(to); err != nil {
		return nil, err
	}
	kind, _ := cmd.Flags().GetString("kind")
	kind = strings.ToLower(strings.TrimSpace(kind))
	if kind != "" && kind != adt.ATCDataRun && kind != adt.ATCDataWorklist {
		return nil, fmt.Errorf("--kind %q: want run or worklist", kind)
	}
	list, err := client.ListATCData(context.Background(), f)
	if err != nil {
		return nil, err
	}
	if kind == "" {
		return list, nil
	}
	var out []adt.ATCData
	for _, d := range list {
		if d.Kind == kind {
			out = append(out, d)
		}
	}
	return out, nil
}

func formatATCData(d adt.ATCData) string {
	if d.Kind == adt.ATCDataRun {
		return fmt.Sprintf("run      %s  %s  %-30s  %d/%d/%d  %s", d.ID, d.On, d.Variant, d.Prio1, d.Prio2, d.Prio3, d.Title)
	}
	return fmt.Sprintf("worklist %s  %s  %-30s  %d obj   %s", d.ID, d.On, d.Variant, d.Objects, d.Title)
}
