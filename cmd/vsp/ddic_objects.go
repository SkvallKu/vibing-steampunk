package main

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/oisee/vibing-steampunk/pkg/adt"
)

var domaCmd = &cobra.Command{
	Use:   "doma",
	Short: "Domains (SE11): read, create, change",
	Long: `A DDIC domain over ADT (/sap/bc/adt/ddic/domains; 7.57 has it, 7.50 not).

  vsp doma get ZDEMO_KIND
  vsp doma create ZDEMO_KIND --package ZPKG --description "Demo kind" --type CHAR --length 2 \
      --fix 01="First kind" --fix 10..15="Ten to fifteen"
  vsp doma set ZDEMO_KIND --add-fix 02="Second kind" --drop-fix 10
  vsp doma set ZDEMO_KIND --fix A=Active --fix I=Inactive   # replaces all fixed values

A write reads the domain, changes what the flags name, puts the whole
domain back and activates it. Texts are the master language's, as ADT
serves them; a new domain's master language is the logon language. --dry-run
shows the result and writes nothing.`,
}

var dtelCmd = &cobra.Command{
	Use:   "dtel",
	Short: "Data elements (SE11): read, create, change",
	Long: `A DDIC data element over ADT (/sap/bc/adt/ddic/dataelements; 7.50 has
it, 7.40 not).

  vsp dtel get ZDEMO_KIND
  vsp dtel create ZDEMO_KIND --package ZPKG --description "Demo kind" --domain ZDEMO_KIND \
      --short Kind --medium "Demo kind" --long "Demo kind" --heading Kind
  vsp dtel create ZDEMO_FLAG --package ZPKG --description "Flag" --type CHAR --length 1 --label Flag
  vsp dtel set ZDEMO_KIND --medium "Kind of demo" --parameter ZKD

The type is one of --domain, --type/--length/--decimals (a predefined type),
or --type-kind with --type-name for a reference (refToDictionaryType,
refToClifType; refToPredefinedAbapType takes --type). --label sets all
four labels at once. A label gets the full length of its field. Texts are
the master language's; a new element's is the logon language.`,
}

func init() {
	domaGet := &cobra.Command{
		Use: "get <NAME>", Short: "Read a domain", Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, _, err := docsClient(cmd)
			if err != nil {
				return err
			}
			d, err := client.GetDomain(context.Background(), args[0])
			if err != nil {
				return err
			}
			if asJSON, _ := cmd.Flags().GetBool("json"); asJSON {
				return printJSON(d)
			}
			printDomain(d)
			return nil
		},
	}
	domaCreate := &cobra.Command{
		Use: "create <NAME>", Short: "Create a domain and activate it", Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			d := &adt.Domain{}
			d.Name = strings.ToUpper(args[0])
			d.Package, _ = cmd.Flags().GetString("package")
			d.Description, _ = cmd.Flags().GetString("description")
			if d.Package == "" || d.Description == "" {
				return fmt.Errorf("--package and --description are required")
			}
			if err := applyDomainFlags(cmd, d); err != nil {
				return err
			}
			if d.OutputLength == 0 {
				d.OutputLength = d.Length
			}
			if dry, _ := cmd.Flags().GetBool("dry-run"); dry {
				if err := d.Check(); err != nil {
					return err
				}
				return printJSON(d)
			}
			client, _, err := docsClient(cmd)
			if err != nil {
				return err
			}
			transport, _ := cmd.Flags().GetString("transport")
			w, err := client.CreateDomain(context.Background(), d, transport)
			return reportDDICWrite(cmd, w, err)
		},
	}
	domaSet := &cobra.Command{
		Use: "set <NAME>", Short: "Change a domain and activate it", Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, _, err := docsClient(cmd)
			if err != nil {
				return err
			}
			change := func(d *adt.Domain) error {
				if cmd.Flags().Changed("description") {
					d.Description, _ = cmd.Flags().GetString("description")
				}
				return applyDomainFlags(cmd, d)
			}
			if dry, _ := cmd.Flags().GetBool("dry-run"); dry {
				d, err := client.GetDomain(context.Background(), args[0])
				if err != nil {
					return err
				}
				if err := change(d); err != nil {
					return err
				}
				if err := d.Check(); err != nil {
					return err
				}
				return printJSON(d)
			}
			transport, _ := cmd.Flags().GetString("transport")
			w, err := client.UpdateDomain(context.Background(), args[0], transport, change)
			return reportDDICWrite(cmd, w, err)
		},
	}
	for _, c := range []*cobra.Command{domaCreate, domaSet} {
		c.Flags().String("description", "", "Short text")
		c.Flags().String("transport", "", "Transport request; chosen like the editor would when empty")
		c.Flags().Bool("dry-run", false, "Show the result and write nothing")
		c.Flags().String("type", "", "Data type: CHAR, NUMC, DEC, INT4, DATS...")
		c.Flags().Int("length", 0, "Number of characters (digits)")
		c.Flags().Int("decimals", 0, "Decimal places")
		c.Flags().Int("output-length", 0, "Output length; the length when not given at create")
		c.Flags().String("conv-exit", "", "Conversion routine, e.g. ALPHA")
		c.Flags().Bool("lowercase", false, "Lower case allowed")
		c.Flags().Bool("sign", false, "Sign allowed")
		c.Flags().String("value-table", "", "Value table")
		c.Flags().StringArray("fix", nil, `Fixed value LOW="text" or LOW..HIGH="text"; replaces all fixed values`)
	}
	domaCreate.Flags().String("package", "", "Package")
	domaSet.Flags().StringArray("add-fix", nil, `Fixed value LOW="text" to add, or whose text to change`)
	domaSet.Flags().StringArray("drop-fix", nil, "Fixed value (its LOW) to remove")
	domaCmd.AddCommand(domaGet, domaCreate, domaSet)

	dtelGet := &cobra.Command{
		Use: "get <NAME>", Short: "Read a data element", Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, _, err := docsClient(cmd)
			if err != nil {
				return err
			}
			e, err := client.GetDataElement(context.Background(), args[0])
			if err != nil {
				return err
			}
			if asJSON, _ := cmd.Flags().GetBool("json"); asJSON {
				return printJSON(e)
			}
			printDataElement(e)
			return nil
		},
	}
	dtelCreate := &cobra.Command{
		Use: "create <NAME>", Short: "Create a data element and activate it", Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			e := &adt.DataElement{}
			e.Name = strings.ToUpper(args[0])
			e.Package, _ = cmd.Flags().GetString("package")
			e.Description, _ = cmd.Flags().GetString("description")
			if e.Package == "" || e.Description == "" {
				return fmt.Errorf("--package and --description are required")
			}
			if err := applyDataElementFlags(cmd, e); err != nil {
				return err
			}
			if dry, _ := cmd.Flags().GetBool("dry-run"); dry {
				if err := e.Check(); err != nil {
					return err
				}
				return printJSON(e)
			}
			client, _, err := docsClient(cmd)
			if err != nil {
				return err
			}
			transport, _ := cmd.Flags().GetString("transport")
			w, err := client.CreateDataElement(context.Background(), e, transport)
			return reportDDICWrite(cmd, w, err)
		},
	}
	dtelSet := &cobra.Command{
		Use: "set <NAME>", Short: "Change a data element and activate it", Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, _, err := docsClient(cmd)
			if err != nil {
				return err
			}
			change := func(e *adt.DataElement) error {
				if cmd.Flags().Changed("description") {
					e.Description, _ = cmd.Flags().GetString("description")
				}
				return applyDataElementFlags(cmd, e)
			}
			if dry, _ := cmd.Flags().GetBool("dry-run"); dry {
				e, err := client.GetDataElement(context.Background(), args[0])
				if err != nil {
					return err
				}
				if err := change(e); err != nil {
					return err
				}
				if err := e.Check(); err != nil {
					return err
				}
				return printJSON(e)
			}
			transport, _ := cmd.Flags().GetString("transport")
			w, err := client.UpdateDataElement(context.Background(), args[0], transport, change)
			return reportDDICWrite(cmd, w, err)
		},
	}
	for _, c := range []*cobra.Command{dtelCreate, dtelSet} {
		c.Flags().String("description", "", "Short text")
		c.Flags().String("transport", "", "Transport request; chosen like the editor would when empty")
		c.Flags().Bool("dry-run", false, "Show the result and write nothing")
		c.Flags().String("domain", "", "Domain the type comes from")
		c.Flags().String("type", "", "Predefined data type: CHAR, NUMC, DEC, STRING...")
		c.Flags().Int("length", 0, "Length of the predefined type")
		c.Flags().Int("decimals", 0, "Decimals of the predefined type")
		c.Flags().String("type-kind", "", "refToDictionaryType, refToClifType or refToPredefinedAbapType")
		c.Flags().String("type-name", "", "The referenced type")
		c.Flags().String("label", "", "All four field labels, each cut to its field")
		c.Flags().String("short", "", "Short field label (10)")
		c.Flags().String("medium", "", "Medium field label (20)")
		c.Flags().String("long", "", "Long field label (40)")
		c.Flags().String("heading", "", "Heading (55)")
		c.Flags().String("search-help", "", "Search help")
		c.Flags().String("search-help-param", "", "Search help parameter")
		c.Flags().String("parameter", "", "SET/GET parameter ID")
		c.Flags().String("default-name", "", "Default component name")
		c.Flags().Bool("change-doc", false, "Change document")
	}
	dtelCreate.Flags().String("package", "", "Package")
	dtelCmd.AddCommand(dtelGet, dtelCreate, dtelSet)

	for _, c := range []*cobra.Command{domaGet, domaCreate, domaSet, dtelGet, dtelCreate, dtelSet} {
		c.Flags().Bool("json", false, "Emit JSON")
	}
	rootCmd.AddCommand(domaCmd, dtelCmd)
}

// parseFixValue reads LOW="text" or LOW..HIGH="text". Quotes around the
// text that reach it through the shell's own quoting, as in '01="One"', are
// not part of the text.
func parseFixValue(s string) (adt.DomainFixValue, error) {
	k, text, ok := strings.Cut(s, "=")
	if !ok || strings.TrimSpace(k) == "" {
		return adt.DomainFixValue{}, fmt.Errorf("fixed value %q: want LOW=TEXT or LOW..HIGH=TEXT", s)
	}
	if len(text) >= 2 && text[0] == '"' && text[len(text)-1] == '"' {
		text = text[1 : len(text)-1]
	}
	low, high, _ := strings.Cut(k, "..")
	return adt.DomainFixValue{Low: low, High: high, Text: text}, nil
}

func applyDomainFlags(cmd *cobra.Command, d *adt.Domain) error {
	f := cmd.Flags()
	if f.Changed("type") {
		d.DataType, _ = f.GetString("type")
		d.DataType = strings.ToUpper(d.DataType)
	}
	if f.Changed("length") {
		d.Length, _ = f.GetInt("length")
		if !f.Changed("output-length") && d.OutputLength < d.Length {
			d.OutputLength = d.Length
		}
	}
	if f.Changed("decimals") {
		d.Decimals, _ = f.GetInt("decimals")
	}
	if f.Changed("output-length") {
		d.OutputLength, _ = f.GetInt("output-length")
	}
	if f.Changed("conv-exit") {
		d.ConversionExit, _ = f.GetString("conv-exit")
	}
	if f.Changed("lowercase") {
		d.Lowercase, _ = f.GetBool("lowercase")
	}
	if f.Changed("sign") {
		d.SignExists, _ = f.GetBool("sign")
	}
	if f.Changed("value-table") {
		d.ValueTable, _ = f.GetString("value-table")
	}
	if f.Changed("fix") {
		vals, _ := f.GetStringArray("fix")
		d.FixValues = nil
		for _, s := range vals {
			v, err := parseFixValue(s)
			if err != nil {
				return err
			}
			d.FixValues = append(d.FixValues, v)
		}
	}
	if f.Lookup("drop-fix") != nil && f.Changed("drop-fix") {
		drop, _ := f.GetStringArray("drop-fix")
		for _, low := range drop {
			kept := d.FixValues[:0]
			found := false
			for _, v := range d.FixValues {
				if v.Low == low {
					found = true
					continue
				}
				kept = append(kept, v)
			}
			if !found {
				return fmt.Errorf("fixed value %q: the domain has none", low)
			}
			d.FixValues = kept
		}
	}
	if f.Lookup("add-fix") != nil && f.Changed("add-fix") {
		add, _ := f.GetStringArray("add-fix")
		for _, s := range add {
			v, err := parseFixValue(s)
			if err != nil {
				return err
			}
			replaced := false
			for i := range d.FixValues {
				if d.FixValues[i].Low == v.Low {
					d.FixValues[i].High, d.FixValues[i].Text = v.High, v.Text
					replaced = true
				}
			}
			if !replaced {
				d.FixValues = append(d.FixValues, v)
			}
		}
	}
	// Positions follow the order: a removed value leaves no gap.
	for i := range d.FixValues {
		d.FixValues[i].Position = i + 1
	}
	return nil
}

func applyDataElementFlags(cmd *cobra.Command, e *adt.DataElement) error {
	f := cmd.Flags()
	kinds := 0
	for _, n := range []string{"domain", "type-kind"} {
		if f.Changed(n) {
			kinds++
		}
	}
	if kinds > 1 {
		return fmt.Errorf("--domain and --type-kind exclude each other")
	}
	switch {
	case f.Changed("domain"):
		e.TypeKind = adt.TypeKindDomain
		e.TypeName, _ = f.GetString("domain")
		e.DataType, e.Length, e.Decimals = "", 0, 0
	case f.Changed("type-kind"):
		e.TypeKind, _ = f.GetString("type-kind")
		e.TypeName, _ = f.GetString("type-name")
		e.DataType, _ = f.GetString("type")
		e.Length, _ = f.GetInt("length")
		e.Decimals, _ = f.GetInt("decimals")
	case f.Changed("type"):
		e.TypeKind = adt.TypeKindPredefined
		e.TypeName = ""
		e.DataType, _ = f.GetString("type")
		e.Length, _ = f.GetInt("length")
		e.Decimals, _ = f.GetInt("decimals")
	case f.Changed("length") || f.Changed("decimals"):
		if e.TypeKind != adt.TypeKindPredefined && e.TypeKind != adt.TypeKindRefToPredefined {
			return fmt.Errorf("--length and --decimals belong to a predefined type (--type)")
		}
		if f.Changed("length") {
			e.Length, _ = f.GetInt("length")
		}
		if f.Changed("decimals") {
			e.Decimals, _ = f.GetInt("decimals")
		}
	}

	setLabel := func(label *string, length *int, limit int, v string) {
		r := []rune(v)
		if len(r) > limit {
			r = r[:limit]
		}
		*label, *length = strings.TrimRight(string(r), " "), limit
	}
	if f.Changed("label") {
		v, _ := f.GetString("label")
		setLabel(&e.ShortLabel, &e.ShortLength, 10, v)
		setLabel(&e.MediumLabel, &e.MediumLength, 20, v)
		setLabel(&e.LongLabel, &e.LongLength, 40, v)
		setLabel(&e.HeadingLabel, &e.HeadingLength, 55, v)
	}
	for _, l := range []struct {
		flag   string
		label  *string
		length *int
	}{
		{"short", &e.ShortLabel, &e.ShortLength},
		{"medium", &e.MediumLabel, &e.MediumLength},
		{"long", &e.LongLabel, &e.LongLength},
		{"heading", &e.HeadingLabel, &e.HeadingLength},
	} {
		if f.Changed(l.flag) {
			*l.label, _ = f.GetString(l.flag)
			*l.length = 0 // the field's full length, filled on write
		}
	}
	if f.Changed("search-help") {
		e.SearchHelp, _ = f.GetString("search-help")
	}
	if f.Changed("search-help-param") {
		e.SearchHelpParameter, _ = f.GetString("search-help-param")
	}
	if f.Changed("parameter") {
		e.SetGetParameter, _ = f.GetString("parameter")
	}
	if f.Changed("default-name") {
		e.DefaultComponentName, _ = f.GetString("default-name")
	}
	if f.Changed("change-doc") {
		e.ChangeDocument, _ = f.GetBool("change-doc")
	}
	return nil
}

func printDomain(d *adt.Domain) {
	fmt.Printf("%s  %s  (package %s, master language %s, %s)\n", d.Name, d.Description, d.Package, d.MasterLanguage, d.Version)
	fmt.Printf("type        %s(%d", d.DataType, d.Length)
	if d.Decimals > 0 {
		fmt.Printf(",%d", d.Decimals)
	}
	fmt.Printf(")  output length %d", d.OutputLength)
	if d.ConversionExit != "" {
		fmt.Printf("  conversion %s", d.ConversionExit)
	}
	if d.Lowercase {
		fmt.Print("  lower case")
	}
	if d.SignExists {
		fmt.Print("  signed")
	}
	fmt.Println()
	if d.ValueTable != "" {
		fmt.Printf("value table %s\n", d.ValueTable)
	}
	for _, v := range d.FixValues {
		k := v.Low
		if v.High != "" {
			k += ".." + v.High
		}
		fmt.Printf("  %-12s %s\n", k, v.Text)
	}
	if d.AppendExists {
		fmt.Println("  (and the values of a fixed value append, not shown)")
	}
}

func printDataElement(e *adt.DataElement) {
	fmt.Printf("%s  %s  (package %s, master language %s, %s)\n", e.Name, e.Description, e.Package, e.MasterLanguage, e.Version)
	switch e.TypeKind {
	case adt.TypeKindDomain:
		fmt.Printf("domain      %s  %s(%d,%d)\n", e.TypeName, e.DataType, e.Length, e.Decimals)
	case adt.TypeKindPredefined:
		fmt.Printf("type        %s(%d,%d)\n", e.DataType, e.Length, e.Decimals)
	default:
		fmt.Printf("%s  %s%s\n", e.TypeKind, e.TypeName, e.DataType)
	}
	fmt.Printf("labels      %q (%d)  %q (%d)  %q (%d)  heading %q (%d)\n",
		e.ShortLabel, e.ShortLength, e.MediumLabel, e.MediumLength, e.LongLabel, e.LongLength, e.HeadingLabel, e.HeadingLength)
	if e.SearchHelp != "" {
		fmt.Printf("search help %s  %s\n", e.SearchHelp, e.SearchHelpParameter)
	}
	if e.SetGetParameter != "" {
		fmt.Printf("parameter   %s\n", e.SetGetParameter)
	}
	if e.DefaultComponentName != "" {
		fmt.Printf("default     %s\n", e.DefaultComponentName)
	}
	if e.ChangeDocument {
		fmt.Println("change document")
	}
}

func reportDDICWrite(cmd *cobra.Command, w *adt.DDICWrite, err error) error {
	if w == nil {
		return err
	}
	if asJSON, _ := cmd.Flags().GetBool("json"); asJSON {
		if perr := printJSON(w); perr != nil {
			return perr
		}
		return err
	}
	for _, n := range w.Notes {
		fmt.Fprintln(os.Stderr, n)
	}
	if w.Activation != nil {
		for _, m := range w.Activation.Messages {
			fmt.Fprintf(os.Stderr, "%s %s\n", m.Type, m.ShortText)
		}
	}
	if err != nil {
		if w.Created {
			fmt.Fprintf(os.Stderr, "%s %s was created but not written through: fix and run set, or delete it\n", w.Type, w.Name)
		}
		return err
	}
	verb := "written and activated"
	if w.Created {
		verb = "created and activated"
	}
	fmt.Fprintf(os.Stderr, "%s %s %s", w.Type, w.Name, verb)
	if w.Transport != "" {
		fmt.Fprintf(os.Stderr, ", transport %s", w.Transport)
	}
	fmt.Fprintln(os.Stderr)
	return nil
}
