"! <p class="shorttext synchronized">vsp ADT fallback: URI mapping</p>
"! BAdI BADI_ADT_MAP_FROM_OBJREF for /sap/bc/adt/zvsp/*: hands the URI to
"! the feature it belongs to, so that /sap/bc/adt/activation finds the
"! workbench object behind it.
CLASS zcl_vsp_adt_uri_mapper DEFINITION
  PUBLIC
  FINAL
  CREATE PUBLIC.

  PUBLIC SECTION.
    INTERFACES if_badi_interface.
    INTERFACES if_adt_map_from_objref.

  PROTECTED SECTION.
  PRIVATE SECTION.
ENDCLASS.



CLASS zcl_vsp_adt_uri_mapper IMPLEMENTATION.

  METHOD if_adt_map_from_objref~map_to_wb_object.
    DATA feature TYPE REF TO zif_vsp_adt_feature.
    DATA uri_string TYPE string.

    uri_string = uri.
    feature = zcl_vsp_adt_app=>get_feature_for_uri( uri_string ).
    IF feature IS NOT BOUND.
      RAISE EXCEPTION TYPE cx_adt_uri_mapping
        EXPORTING
          textid = cx_adt_uri_mapping=>invalid_uri
          uri    = uri_string.
    ENDIF.
    result = feature->map_to_wb_object( uri ).
  ENDMETHOD.


  METHOD if_adt_map_from_objref~map_to_wb_context.
    CLEAR: wb_context, line, line_offset.
  ENDMETHOD.


  METHOD if_adt_map_from_objref~map_to_include.
    CLEAR: program, include, line, line_offset, end_line, end_offset.
  ENDMETHOD.

ENDCLASS.
