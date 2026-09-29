"! <p class="shorttext synchronized">vsp ADT fallback: data elements feature</p>
"! Stands in for /sap/bc/adt/ddic/dataelements on 7.40, which does not
"! have it. Registered on 7.50 too, where vsp keeps to the standard
"! resource: there it serves to try ours.
CLASS zcl_vsp_adt_dtel_feature DEFINITION
  PUBLIC
  FINAL
  CREATE PUBLIC.

  PUBLIC SECTION.
    INTERFACES zif_vsp_adt_feature.

    CONSTANTS co_path TYPE string VALUE '/ddic/dataelements'.

  PROTECTED SECTION.
  PRIVATE SECTION.
    CONSTANTS co_handler TYPE string VALUE 'ZCL_VSP_ADT_DTEL_RESOURCE'.
ENDCLASS.



CLASS zcl_vsp_adt_dtel_feature IMPLEMENTATION.

  METHOD zif_vsp_adt_feature~get_path.
    result = co_path.
  ENDMETHOD.


  METHOD zif_vsp_adt_feature~register_resources.
    DATA accepts TYPE if_adt_discovery_collection=>ty_accepts.

    APPEND zcl_vsp_adt_dtel_resource=>co_content_type TO accepts.
    APPEND zcl_vsp_adt_dtel_resource=>co_content_type_v2 TO accepts.
    registry->register_discoverable_resource(
      url             = co_path
      accepted_types  = accepts
      handler_class   = co_handler
      description     = 'Data Elements'
      category_scheme = zcl_vsp_adt_app=>co_scheme
      category_term   = 'ddic/dataelements' ) ##NO_TEXT.
    registry->register_resource(
      template      = co_path && '/{name}'
      handler_class = co_handler ).
  ENDMETHOD.


  METHOD zif_vsp_adt_feature~map_to_wb_object.
    result = zcl_vsp_adt_dd_resource=>to_wb_object( uri        = uri
                                                    collection = co_path
                                                    objtype    = 'DTEL'
                                                    subtype    = 'DE' ).
  ENDMETHOD.

ENDCLASS.
