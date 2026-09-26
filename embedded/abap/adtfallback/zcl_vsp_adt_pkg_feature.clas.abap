"! <p class="shorttext synchronized">vsp ADT fallback: packages feature</p>
"! Stands in for /sap/bc/adt/packages on releases whose ADT cannot read,
"! create or delete a package (7.50 has only /packages/settings).
CLASS zcl_vsp_adt_pkg_feature DEFINITION
  PUBLIC
  FINAL
  CREATE PUBLIC.

  PUBLIC SECTION.
    INTERFACES zif_vsp_adt_feature.

    CONSTANTS co_path TYPE string VALUE '/packages'.

  PROTECTED SECTION.
  PRIVATE SECTION.
    CONSTANTS co_handler TYPE string VALUE 'ZCL_VSP_ADT_PKG_RESOURCE'.
ENDCLASS.



CLASS zcl_vsp_adt_pkg_feature IMPLEMENTATION.

  METHOD zif_vsp_adt_feature~get_path.
    result = co_path.
  ENDMETHOD.


  METHOD zif_vsp_adt_feature~register_resources.
    DATA accepts TYPE if_adt_discovery_collection=>ty_accepts.

    APPEND zcl_vsp_adt_pkg_resource=>co_content_type TO accepts.
    registry->register_discoverable_resource(
      url             = co_path
      accepted_types  = accepts
      handler_class   = co_handler
      description     = 'Packages'
      category_scheme = zcl_vsp_adt_app=>co_scheme
      category_term   = 'packages' ) ##NO_TEXT.
    registry->register_resource(
      template      = co_path && '/{name}'
      handler_class = co_handler ).
  ENDMETHOD.


  METHOD zif_vsp_adt_feature~map_to_wb_object.
    DATA path TYPE string.
    DATA rest TYPE string.
    DATA name TYPE string.
    DATA object_type TYPE wbobjtype.
    DATA object_key TYPE seu_objkey.

    path = uri.
    SPLIT path AT '?' INTO path rest.
    FIND REGEX '^/sap/bc/adt/zvsp/packages/([^/]+)$' IN path IGNORING CASE
      SUBMATCHES name.
    IF sy-subrc <> 0.
      RAISE EXCEPTION TYPE cx_adt_uri_mapping
        EXPORTING
          textid = cx_adt_uri_mapping=>invalid_uri
          uri    = path.
    ENDIF.
    name = to_upper( cl_http_utility=>unescape_url( name ) ).

    object_type-objtype_tr = 'DEVC'.
    object_type-subtype_wb = 'K'.
    object_key = name.
    cl_wb_object=>create_from_global_type(
      EXPORTING
        p_object_type = object_type
        p_object_key  = object_key
      RECEIVING
        p_wb_object   = result
      EXCEPTIONS
        OTHERS        = 1 ).
    IF sy-subrc <> 0.
      RAISE EXCEPTION TYPE cx_adt_uri_mapping
        EXPORTING
          textid = cx_adt_uri_mapping=>invalid_uri
          uri    = path.
    ENDIF.
  ENDMETHOD.

ENDCLASS.
