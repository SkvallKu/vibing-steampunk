"! <p class="shorttext synchronized">vsp ADT fallback: DDIC structure source</p>
"! A structure body as source and back: the "define type" source 7.50
"! edits structures in, read into the dictionary's own tables and
"! written from them. A table's body is the same source under another
"! header, so the table resource uses it too. Two implementations: the
"! standard's (ZCL_VSP_DDIC_SRC_SAP) where the release has one, and ours
"! (ZCL_VSP_DDIC_SRC_OWN) where it has not; ZCL_VSP_DDIC_SRC picks.
INTERFACE zif_vsp_ddic_source PUBLIC.
  TYPES:
    BEGIN OF ty_definition,
      header             TYPE dd02v,
      fields             TYPE dd03ptab,
      foreign_keys       TYPE dd08vttyp,
      "! As DDIF_TABL_PUT takes them: every key field of the check
      "! table, generic ones included.
      foreign_key_fields TYPE dd05mttyp,
      value_helps        TYPE dd35vttyp,
      value_help_fields  TYPE dd36mttyp,
    END OF ty_definition.

  "! The dictionary tables of a "define type" source. An error in the
  "! source is refused with its line.
  METHODS parse
    IMPORTING name          TYPE ddobjname
              source        TYPE string_table
    RETURNING VALUE(result) TYPE ty_definition
    RAISING   cx_adt_res_not_acceptable.

  "! The "define type" source of a structure or of a table's body, in a
  "! version: A active, I inactive. Empty when there is none.
  METHODS render
    IMPORTING name          TYPE ddobjname
              version       TYPE r3state
    RETURNING VALUE(result) TYPE string_table.
ENDINTERFACE.
