"! <p class="shorttext synchronized">vsp ADT fallback: own ATC data</p>
"! The runs and worklists ATC keeps for the logged-on user, listed and
"! removed one by one (ZCL_VSP_ADT_ATCD_RESOURCE). ADT has no DELETE for
"! them on any release, so this is no stand-in: its own path, offered on
"! every release.
CLASS zcl_vsp_adt_atcd_feature DEFINITION
  PUBLIC
  FINAL
  CREATE PUBLIC.

  PUBLIC SECTION.
    INTERFACES zif_vsp_adt_feature.

    CONSTANTS co_path TYPE string VALUE '/atcdata'.

  PROTECTED SECTION.
  PRIVATE SECTION.
ENDCLASS.



CLASS zcl_vsp_adt_atcd_feature IMPLEMENTATION.

  METHOD zif_vsp_adt_feature~get_path.
    result = co_path.
  ENDMETHOD.


  METHOD zif_vsp_adt_feature~register_resources.
    registry->register_discoverable_resource(
      url             = co_path
      handler_class   = 'ZCL_VSP_ADT_ATCD_RESOURCE'
      description     = 'Own ATC runs and worklists'
      category_scheme = zcl_vsp_adt_app=>co_scheme
      category_term   = 'atcdata' ) ##NO_TEXT.
    registry->register_resource(
      template      = co_path && '/{kind}/{id}'
      handler_class = 'ZCL_VSP_ADT_ATCD_RESOURCE' ) ##NO_TEXT.
  ENDMETHOD.


  METHOD zif_vsp_adt_feature~map_to_wb_object.
    " A run is no workbench object: nothing to activate.
    RAISE EXCEPTION TYPE cx_adt_uri_mapping
      EXPORTING
        textid = cx_adt_uri_mapping=>invalid_uri
        uri    = CONV #( uri ).
  ENDMETHOD.

ENDCLASS.
