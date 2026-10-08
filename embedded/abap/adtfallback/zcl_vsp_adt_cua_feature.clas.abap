"! <p class="shorttext synchronized">vsp ADT fallback: GUI statuses feature</p>
"! GUI statuses and titles of programs (CUAD). No release has an ADT
"! resource for them: Eclipse edits them in the embedded SAP GUI. So the
"! path is ours, /cua/programs, and the document is abapGit's.
CLASS zcl_vsp_adt_cua_feature DEFINITION
  PUBLIC
  FINAL
  CREATE PUBLIC.

  PUBLIC SECTION.
    INTERFACES zif_vsp_adt_feature.

    CONSTANTS co_path TYPE string VALUE '/cua'.

  PROTECTED SECTION.
  PRIVATE SECTION.
    CONSTANTS co_handler TYPE string VALUE 'ZCL_VSP_ADT_CUA_RESOURCE'.
ENDCLASS.



CLASS zcl_vsp_adt_cua_feature IMPLEMENTATION.

  METHOD zif_vsp_adt_feature~get_path.
    result = co_path.
  ENDMETHOD.


  METHOD zif_vsp_adt_feature~register_resources.
    DATA accepts TYPE if_adt_discovery_collection=>ty_accepts.
    DATA base TYPE string.

    base = co_path && '/programs'.
    APPEND zcl_vsp_adt_cua_resource=>co_content_type TO accepts.
    registry->register_discoverable_resource(
      url             = base
      accepted_types  = accepts
      handler_class   = co_handler
      description     = 'GUI statuses and titles (programs)'
      category_scheme = zcl_vsp_adt_app=>co_scheme
      category_term   = 'cua/programs' ) ##NO_TEXT.
    registry->register_resource(
      template      = base && '/{name}'
      handler_class = co_handler ).
  ENDMETHOD.


  METHOD zif_vsp_adt_feature~map_to_wb_object.
    DATA path TYPE string.
    DATA rest TYPE string.
    DATA name TYPE string.

    path = uri.
    SPLIT path AT '?' INTO path rest.
    SPLIT path AT '#' INTO path rest.

    FIND REGEX '^/sap/bc/adt/zvsp/cua/programs/([^/]+)$' IN path IGNORING CASE
      SUBMATCHES name.
    IF sy-subrc <> 0.
      RAISE EXCEPTION TYPE cx_adt_uri_mapping
        EXPORTING
          textid = cx_adt_uri_mapping=>invalid_uri
          uri    = path.
    ENDIF.
    name = cl_http_utility=>unescape_url( name ).
    TRANSLATE name TO UPPER CASE.

    cl_wb_object=>create_from_transport_key(
      EXPORTING
        p_object    = 'CUAD'
        p_obj_name  = CONV #( name )
      RECEIVING
        p_wb_object = result
      EXCEPTIONS
        OTHERS      = 1 ).
    IF sy-subrc <> 0.
      RAISE EXCEPTION TYPE cx_adt_uri_mapping
        EXPORTING
          textid = cx_adt_uri_mapping=>invalid_uri
          uri    = path.
    ENDIF.
  ENDMETHOD.

ENDCLASS.
