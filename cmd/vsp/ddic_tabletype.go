package main

import (
	"context"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/oisee/vibing-steampunk/pkg/adt"
)

var ttypCmd = &cobra.Command{
	Use:   "ttyp",
	Short: "Table types (SE11): read, create, change",
	Long: `A DDIC table type over ADT (/sap/bc/adt/ddic/tabletypes; 7.57 has it, 7.50
and 7.40 not — there adt-fallback stands in).

  vsp ttyp get ZDEMO_KV_T
  vsp ttyp create ZDEMO_KV_T --package ZPKG --description "Key/value pairs" \
      --row-type ZDEMO_KV --access hashed --key-fields KEY
  vsp ttyp create ZDEMO_STRINGS --package ZPKG --description "Strings" --type STRING
  vsp ttyp create ZDEMO_FIELDNAME_RNG --package ZPKG --description "Field names" \
      --type-kind rangeTypeOnDataelement --type-name FIELDNAME --range-structure ZDEMO_FIELDNAME_RNG_S
  vsp ttyp set ZDEMO_KV_T --add-seckey BY_VALUE:nonUniqueSorted:VALUE,KEY --drop-seckey OLD

The row type is one of --row-type (a dictionary type: structure, table,
table type or data element), --type/--length/--decimals (a predefined type),
or --type-kind with --type-name for a reference (refToDictionaryType,
refToClassOrInterfaceType; refToPredefinedAbapType takes --type) or a range
(rangeTypeOnDataelement with --type-name, rangeTypeOnPredefinedType with
--type, both with --range-structure). A range table type's key is fixed.

The primary key is --key (standard, rowType, notSpecified) or --key-fields
(its components); --key-kind is unique, nonUnique or notSpecified. Left
open on create: a standard table, standard key, non-unique (unique for
hashed). A secondary key is NAME:ACCESS:FIELD,FIELD[:description] or
NAME:ACCESS:rowType, ACCESS one of uniqueHashed, uniqueSorted,
nonUniqueSorted. A write reads the table type, changes what the flags name,
puts it back whole and activates it. --dry-run shows the result and writes
nothing.`,
}

func init() {
	ttypGet := &cobra.Command{
		Use: "get <NAME>", Short: "Read a table type", Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, _, err := docsClient(cmd)
			if err != nil {
				return err
			}
			t, err := client.GetTableType(context.Background(), args[0])
			if err != nil {
				return err
			}
			if asJSON, _ := cmd.Flags().GetBool("json"); asJSON {
				return printJSON(t)
			}
			printTableType(t)
			return nil
		},
	}
	ttypCreate := &cobra.Command{
		Use: "create <NAME>", Short: "Create a table type and activate it", Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			t := &adt.TableType{}
			t.Name = strings.ToUpper(args[0])
			t.Package, _ = cmd.Flags().GetString("package")
			t.Description, _ = cmd.Flags().GetString("description")
			if t.Package == "" || t.Description == "" {
				return fmt.Errorf("--package and --description are required")
			}
			if err := applyTableTypeFlags(cmd, t); err != nil {
				return err
			}
			if t.TypeKind == "" {
				return fmt.Errorf("name the row type: --row-type, --type or --type-kind")
			}
			if dry, _ := cmd.Flags().GetBool("dry-run"); dry {
				t.FillDefaults()
				if err := t.Check(); err != nil {
					return err
				}
				return printJSON(t)
			}
			client, _, err := docsClient(cmd)
			if err != nil {
				return err
			}
			transport, _ := cmd.Flags().GetString("transport")
			w, err := client.CreateTableTypeFrom(context.Background(), t, transport)
			return reportDDICWrite(cmd, w, err)
		},
	}
	ttypSet := &cobra.Command{
		Use: "set <NAME>", Short: "Change a table type and activate it", Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, _, err := docsClient(cmd)
			if err != nil {
				return err
			}
			change := func(t *adt.TableType) error {
				if cmd.Flags().Changed("description") {
					t.Description, _ = cmd.Flags().GetString("description")
				}
				return applyTableTypeFlags(cmd, t)
			}
			if dry, _ := cmd.Flags().GetBool("dry-run"); dry {
				t, err := client.GetTableType(context.Background(), args[0])
				if err != nil {
					return err
				}
				if err := change(t); err != nil {
					return err
				}
				if err := t.Check(); err != nil {
					return err
				}
				return printJSON(t)
			}
			transport, _ := cmd.Flags().GetString("transport")
			w, err := client.UpdateTableType(context.Background(), args[0], transport, change)
			return reportDDICWrite(cmd, w, err)
		},
	}
	for _, c := range []*cobra.Command{ttypCreate, ttypSet} {
		c.Flags().String("description", "", "Short text")
		c.Flags().String("transport", "", "Transport request; chosen like the editor would when empty")
		c.Flags().Bool("dry-run", false, "Show the result and write nothing")
		c.Flags().String("row-type", "", "Row type: a structure, table, table type or data element")
		c.Flags().String("type", "", "Predefined row type: CHAR, NUMC, DEC, STRING...")
		c.Flags().Int("length", 0, "Length of the predefined type")
		c.Flags().Int("decimals", 0, "Decimals of the predefined type")
		c.Flags().String("type-kind", "", "refToDictionaryType, refToClassOrInterfaceType, refToPredefinedAbapType, rangeTypeOnDataelement, rangeTypeOnPredefinedType")
		c.Flags().String("type-name", "", "The referenced type or class, or the data element of a range")
		c.Flags().String("range-structure", "", "The range structure of a range table type")
		c.Flags().String("access", "", "standard, sorted, hashed, index or notSpecified")
		c.Flags().String("key", "", "Primary key: standard, rowType or notSpecified")
		c.Flags().StringSlice("key-fields", nil, "Primary key components (the key definition becomes keyComponents)")
		c.Flags().String("key-kind", "", "unique, nonUnique or notSpecified")
		c.Flags().String("key-alias", "", "Alias of the primary key")
		c.Flags().Int("initial-rows", 0, "Initial number of rows")
		c.Flags().String("secondary-keys", "", "Further secondary keys in declarations: allowed, notAllowed or notSpecified")
		c.Flags().StringArray("seckey", nil, "Secondary key NAME:ACCESS:FIELD,FIELD[:description]; replaces all secondary keys")
	}
	ttypCreate.Flags().String("package", "", "Package")
	ttypSet.Flags().StringArray("add-seckey", nil, "Secondary key to add, or to replace by name")
	ttypSet.Flags().StringArray("drop-seckey", nil, "Secondary key (its name) to remove")
	ttypCmd.AddCommand(ttypGet, ttypCreate, ttypSet)
	for _, c := range []*cobra.Command{ttypGet, ttypCreate, ttypSet} {
		c.Flags().Bool("json", false, "Emit JSON")
	}
	rootCmd.AddCommand(ttypCmd)
}

// parseSecondaryKey reads NAME:ACCESS:FIELD,FIELD[:description] or
// NAME:ACCESS:rowType[:description].
func parseSecondaryKey(s string) (adt.TableTypeSecondaryKey, error) {
	parts := strings.SplitN(s, ":", 4)
	if len(parts) < 3 || strings.TrimSpace(parts[0]) == "" || strings.TrimSpace(parts[2]) == "" {
		return adt.TableTypeSecondaryKey{}, fmt.Errorf("secondary key %q: want NAME:ACCESS:FIELD,FIELD[:description] or NAME:ACCESS:rowType", s)
	}
	k := adt.TableTypeSecondaryKey{Name: strings.ToUpper(strings.TrimSpace(parts[0])), Access: strings.TrimSpace(parts[1])}
	if strings.TrimSpace(parts[2]) == adt.KeyDefRowType {
		k.Definition = adt.KeyDefRowType
	} else {
		k.Definition = adt.KeyDefComponents
		for _, f := range strings.Split(parts[2], ",") {
			if f = strings.ToUpper(strings.TrimSpace(f)); f != "" {
				k.Components = append(k.Components, f)
			}
		}
	}
	if len(parts) == 4 {
		k.Description = parts[3]
	}
	return k, nil
}

func applyTableTypeFlags(cmd *cobra.Command, t *adt.TableType) error {
	f := cmd.Flags()
	kinds := 0
	for _, n := range []string{"row-type", "type-kind"} {
		if f.Changed(n) {
			kinds++
		}
	}
	if kinds > 1 {
		return fmt.Errorf("--row-type and --type-kind exclude each other")
	}
	str := func(n string) string { v, _ := f.GetString(n); return strings.TrimSpace(v) }
	switch {
	case f.Changed("row-type"):
		t.TypeKind, t.TypeName = adt.RowKindDictionary, strings.ToUpper(str("row-type"))
		t.DataType, t.Length, t.Decimals, t.RangeType = "", 0, 0, ""
	case f.Changed("type-kind"):
		t.TypeKind = str("type-kind")
		t.TypeName = strings.ToUpper(str("type-name"))
		t.DataType = strings.ToUpper(str("type"))
		t.Length, _ = f.GetInt("length")
		t.Decimals, _ = f.GetInt("decimals")
		t.RangeType = strings.ToUpper(str("range-structure"))
	case f.Changed("type"):
		t.TypeKind, t.TypeName, t.RangeType = adt.TypeKindPredefined, "", ""
		t.DataType = strings.ToUpper(str("type"))
		t.Length, _ = f.GetInt("length")
		t.Decimals, _ = f.GetInt("decimals")
	default:
		if f.Changed("length") || f.Changed("decimals") {
			if t.TypeKind != adt.TypeKindPredefined && t.TypeKind != adt.RowKindRangeOnPredefined {
				return fmt.Errorf("--length and --decimals belong to a predefined type (--type)")
			}
			if f.Changed("length") {
				t.Length, _ = f.GetInt("length")
			}
			if f.Changed("decimals") {
				t.Decimals, _ = f.GetInt("decimals")
			}
		}
		if f.Changed("range-structure") {
			t.RangeType = strings.ToUpper(str("range-structure"))
		}
	}
	if f.Changed("access") {
		t.AccessType = str("access")
	}
	if f.Changed("key") && f.Changed("key-fields") {
		return fmt.Errorf("--key and --key-fields exclude each other")
	}
	if f.Changed("key") {
		t.KeyDefinition, t.KeyComponents = str("key"), nil
	}
	if f.Changed("key-fields") {
		fields, _ := f.GetStringSlice("key-fields")
		t.KeyDefinition, t.KeyComponents = adt.KeyDefComponents, nil
		for _, k := range fields {
			if k = strings.ToUpper(strings.TrimSpace(k)); k != "" {
				t.KeyComponents = append(t.KeyComponents, k)
			}
		}
	}
	if f.Changed("key-kind") {
		t.KeyKind = str("key-kind")
	} else if f.Changed("access") && t.AccessType == adt.AccessHashed {
		t.KeyKind = adt.KeyKindUnique
	} else if f.Changed("access") && (t.AccessType == adt.AccessStandard || t.AccessType == adt.AccessIndex) && t.KeyKind == adt.KeyKindUnique {
		t.KeyKind = adt.KeyKindNonUnique
	}
	if f.Changed("key-alias") {
		t.KeyAlias = strings.ToUpper(str("key-alias"))
	}
	if f.Changed("initial-rows") {
		t.InitialRowCount, _ = f.GetInt("initial-rows")
	}
	if f.Changed("secondary-keys") {
		t.SecondaryKeysAllowed = str("secondary-keys")
	}
	if f.Changed("seckey") {
		keys, _ := f.GetStringArray("seckey")
		t.SecondaryKeys = nil
		for _, s := range keys {
			k, err := parseSecondaryKey(s)
			if err != nil {
				return err
			}
			t.SecondaryKeys = append(t.SecondaryKeys, k)
		}
	}
	if f.Lookup("drop-seckey") != nil && f.Changed("drop-seckey") {
		drop, _ := f.GetStringArray("drop-seckey")
		for _, name := range drop {
			name = strings.ToUpper(strings.TrimSpace(name))
			kept := t.SecondaryKeys[:0]
			found := false
			for _, k := range t.SecondaryKeys {
				if k.Name == name {
					found = true
					continue
				}
				kept = append(kept, k)
			}
			if !found {
				return fmt.Errorf("secondary key %s: the table type has none", name)
			}
			t.SecondaryKeys = kept
		}
	}
	if f.Lookup("add-seckey") != nil && f.Changed("add-seckey") {
		add, _ := f.GetStringArray("add-seckey")
		for _, s := range add {
			k, err := parseSecondaryKey(s)
			if err != nil {
				return err
			}
			replaced := false
			for i := range t.SecondaryKeys {
				if t.SecondaryKeys[i].Name == k.Name {
					t.SecondaryKeys[i] = k
					replaced = true
				}
			}
			if !replaced {
				t.SecondaryKeys = append(t.SecondaryKeys, k)
			}
		}
	}
	return nil
}

func printTableType(t *adt.TableType) {
	fmt.Printf("%s  %s  (package %s, master language %s, %s)\n", t.Name, t.Description, t.Package, t.MasterLanguage, t.Version)
	switch t.TypeKind {
	case adt.TypeKindPredefined, adt.TypeKindRefToPredefined:
		fmt.Printf("row type    %s %s(%d,%d)\n", t.TypeKind, t.DataType, t.Length, t.Decimals)
	case adt.RowKindRangeOnDataElement, adt.RowKindRangeOnPredefined:
		fmt.Printf("row type    %s %s(%d,%d), range structure %s\n", t.TypeKind, strings.TrimSpace(t.TypeName+" "+t.DataType), t.Length, t.Decimals, t.RangeType)
	default:
		fmt.Printf("row type    %s %s", t.TypeKind, t.TypeName)
		if t.DataType != "" {
			fmt.Printf(" (%s)", t.DataType)
		}
		fmt.Println()
	}
	fmt.Printf("access      %s", t.AccessType)
	if t.InitialRowCount > 0 {
		fmt.Printf(", %d initial rows", t.InitialRowCount)
	}
	fmt.Println()
	fmt.Printf("key         %s, %s", t.KeyDefinition, t.KeyKind)
	if len(t.KeyComponents) > 0 {
		fmt.Printf(": %s", strings.Join(t.KeyComponents, ", "))
	}
	if t.KeyAlias != "" {
		fmt.Printf("  alias %s", t.KeyAlias)
	}
	fmt.Println()
	if t.SecondaryKeysAllowed != "" && t.SecondaryKeysAllowed != adt.NotSpecified {
		fmt.Printf("further secondary keys %s\n", t.SecondaryKeysAllowed)
	}
	for _, k := range t.SecondaryKeys {
		fields := strings.Join(k.Components, ", ")
		if k.Definition == adt.KeyDefRowType {
			fields = "row type"
		}
		fmt.Printf("  %-30s %-16s %s", k.Name, k.Access, fields)
		if k.Description != "" {
			fmt.Printf("  %q", k.Description)
		}
		fmt.Println()
	}
}
