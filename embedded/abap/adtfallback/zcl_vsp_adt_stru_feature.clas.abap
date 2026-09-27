"! <p class="shorttext synchronized">vsp ADT fallback: structures feature</p>
"! Stands in for /sap/bc/adt/ddic/structures (structures as source) on
"! releases before 7.50. Registered on 7.50 too, where vsp keeps to the
"! standard resource: there it serves to try ours (?parser=own).
CLASS zcl_vsp_adt_stru_feature DEFINITION
  PUBLIC
  FINAL
  CREATE PUBLIC.

  PUBLIC SECTION.
    INTERFACES zif_vsp_adt_feature.

    CONSTANTS co_path TYPE string VALUE '/ddic/structures'.

  PROTECTED SECTION.
  PRIVATE SECTION.
    CONSTANTS co_handler TYPE string VALUE 'ZCL_VSP_ADT_TABL_RESOURCE'.
ENDCLASS.



CLASS zcl_vsp_adt_stru_feature IMPLEMENTATION.

  METHOD zif_vsp_adt_feature~get_path.
    result = co_path.
  ENDMETHOD.


  METHOD zif_vsp_adt_feature~register_resources.
    DATA accepts TYPE if_adt_discovery_collection=>ty_accepts.

    APPEND zcl_vsp_adt_tabl_resource=>co_structure_type TO accepts.
    registry->register_discoverable_resource(
      url             = co_path
      accepted_types  = accepts
      handler_class   = co_handler
      description     = 'Structures'
      category_scheme = zcl_vsp_adt_app=>co_scheme
      category_term   = 'ddic/structures' ) ##NO_TEXT.
    registry->register_resource(
      template      = co_path && '/{name}'
      handler_class = co_handler ).
    registry->register_resource(
      template      = co_path && '/{name}/source/main'
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
    SPLIT path AT '#' INTO path rest.
    FIND REGEX '^/sap/bc/adt/zvsp/ddic/structures/([^/]+)(/source/main)?$'
      IN path IGNORING CASE SUBMATCHES name.
    IF sy-subrc <> 0.
      RAISE EXCEPTION TYPE cx_adt_uri_mapping
        EXPORTING
          textid = cx_adt_uri_mapping=>invalid_uri
          uri    = path.
    ENDIF.
    name = to_upper( cl_http_utility=>unescape_url( name ) ).

    object_type-objtype_tr = 'TABL'.
    object_type-subtype_wb = 'DS'.
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
