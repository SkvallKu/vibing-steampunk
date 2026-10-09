"! <p class="shorttext synchronized">vsp ADT fallback: Code Inspector</p>
"! Check variants and the checks that go into them, for ATC on releases
"! that have no ATC check variant objects: there ATC runs a global Code
"! Inspector variant named in the run (checkVariant). 7.5x and later edit
"! CHKV objects at /sap/bc/adt/atc/checkvariants, a different thing (a
"! repository object made of CHKO check objects), so this is no stand-in
"! for it: its own path, offered on every release.
CLASS zcl_vsp_adt_ci_feature DEFINITION
  PUBLIC
  FINAL
  CREATE PUBLIC.

  PUBLIC SECTION.
    INTERFACES zif_vsp_adt_feature.

    CONSTANTS co_path TYPE string VALUE '/ci'.

  PROTECTED SECTION.
  PRIVATE SECTION.
ENDCLASS.



CLASS zcl_vsp_adt_ci_feature IMPLEMENTATION.

  METHOD zif_vsp_adt_feature~get_path.
    result = co_path.
  ENDMETHOD.


  METHOD zif_vsp_adt_feature~register_resources.
    registry->register_discoverable_resource(
      url             = co_path && '/checkvariants'
      handler_class   = 'ZCL_VSP_ADT_CHKV_RESOURCE'
      description     = 'Code Inspector check variants'
      category_scheme = zcl_vsp_adt_app=>co_scheme
      category_term   = 'ci/checkvariants' ) ##NO_TEXT.
    registry->register_resource(
      template      = co_path && '/checkvariants/{name}'
      handler_class = 'ZCL_VSP_ADT_CHKV_RESOURCE' ) ##NO_TEXT.
    registry->register_discoverable_resource(
      url             = co_path && '/checks'
      handler_class   = 'ZCL_VSP_ADT_CICHK_RESOURCE'
      description     = 'Code Inspector checks'
      category_scheme = zcl_vsp_adt_app=>co_scheme
      category_term   = 'ci/checks' ) ##NO_TEXT.
  ENDMETHOD.


  METHOD zif_vsp_adt_feature~map_to_wb_object.
    " A check variant is no workbench object here: nothing to activate.
    RAISE EXCEPTION TYPE cx_adt_uri_mapping
      EXPORTING
        textid = cx_adt_uri_mapping=>invalid_uri
        uri    = CONV #( uri ).
  ENDMETHOD.

ENDCLASS.
