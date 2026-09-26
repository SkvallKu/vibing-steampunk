"! <p class="shorttext synchronized">vsp ADT fallback: one feature</p>
"! A feature stands in for an ADT resource this release lacks, under
"! /sap/bc/adt/zvsp followed by the tail of the standard path: the text
"! elements of /sap/bc/adt/textelements are served at
"! /sap/bc/adt/zvsp/textelements. A class implementing this interface is
"! found at runtime; a new feature needs no change anywhere else.
INTERFACE zif_vsp_adt_feature
  PUBLIC.

  CONSTANTS co_root TYPE string VALUE '/sap/bc/adt/zvsp'.

  "! The tail of the standard path, e.g. /textelements.
  METHODS get_path
    RETURNING VALUE(result) TYPE string.

  "! Registers the feature's resources, relative to co_root.
  METHODS register_resources
    IMPORTING registry TYPE REF TO if_adt_disc_rest_rc_registry.

  "! The workbench object behind a URI of this feature, so that
  "! /sap/bc/adt/activation can activate it.
  METHODS map_to_wb_object
    IMPORTING uri           TYPE csequence
    RETURNING VALUE(result) TYPE REF TO cl_wb_object
    RAISING   cx_adt_uri_mapping.

ENDINTERFACE.
