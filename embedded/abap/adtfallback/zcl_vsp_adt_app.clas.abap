"! <p class="shorttext synchronized">vsp ADT fallback: application</p>
"! The ADT application behind /sap/bc/adt/zvsp (BAdI
"! BADI_ADT_REST_RFC_APPLICATION) and its discovery workspace (BAdI
"! BADI_ADT_DISCOVERY_PROVIDER). It serves /info and whatever the features
"! register; the features are the classes implementing ZIF_VSP_ADT_FEATURE.
CLASS zcl_vsp_adt_app DEFINITION
  PUBLIC
  INHERITING FROM cl_adt_disc_res_app_base
  FINAL
  CREATE PUBLIC.

  PUBLIC SECTION.
    "! The version of the installed set; vsp compares it with its own.
    CONSTANTS co_version TYPE string VALUE '0.8.1'.
    CONSTANTS co_scheme TYPE string
      VALUE 'http://www.sap.com/adt/categories/vsp/adtfallback'.

    TYPES tt_features TYPE STANDARD TABLE OF REF TO zif_vsp_adt_feature
      WITH DEFAULT KEY.

    CLASS-METHODS get_features
      RETURNING VALUE(result) TYPE tt_features.

    "! The feature whose path the URI is under, or nothing.
    CLASS-METHODS get_feature_for_uri
      IMPORTING uri           TYPE csequence
      RETURNING VALUE(result) TYPE REF TO zif_vsp_adt_feature.

    METHODS if_adt_rest_rfc_application~get_static_uri_path REDEFINITION.
    METHODS if_adt_discovery_provider~register_workspaces REDEFINITION.

  PROTECTED SECTION.
    METHODS get_application_title REDEFINITION.
    METHODS register_resources REDEFINITION.

  PRIVATE SECTION.
ENDCLASS.



CLASS zcl_vsp_adt_app IMPLEMENTATION.

  METHOD if_adt_rest_rfc_application~get_static_uri_path.
    result = zif_vsp_adt_feature=>co_root.
  ENDMETHOD.


  METHOD if_adt_discovery_provider~register_workspaces.
    " Discovery is one document for the whole system: a failure here must
    " cost only our workspace, never the discovery of every ADT client.
    TRY.
        super->if_adt_discovery_provider~register_workspaces( registry ).
      CATCH cx_root ##CATCH_ALL ##NO_HANDLER.
    ENDTRY.
  ENDMETHOD.


  METHOD get_application_title.
    result = 'vsp ADT fallback points' ##NO_TEXT.
  ENDMETHOD.


  METHOD register_resources.
    DATA features TYPE tt_features.
    DATA feature TYPE REF TO zif_vsp_adt_feature.

    registry->register_discoverable_resource(
      url             = '/info'
      handler_class   = 'ZCL_VSP_ADT_RES_INFO'
      description     = 'Installed version and features'
      category_scheme = co_scheme
      category_term   = 'info' ) ##NO_TEXT.

    features = get_features( ).
    LOOP AT features INTO feature.
      TRY.
          feature->register_resources( registry ).
        CATCH cx_root ##CATCH_ALL ##NO_HANDLER.
      ENDTRY.
    ENDLOOP.
  ENDMETHOD.


  METHOD get_features.
    DATA names TYPE STANDARD TABLE OF seoclsname WITH DEFAULT KEY.
    DATA name TYPE seoclsname.
    DATA feature TYPE REF TO zif_vsp_adt_feature.

    SELECT DISTINCT clsname FROM seometarel INTO TABLE names
      WHERE refclsname = 'ZIF_VSP_ADT_FEATURE'
        AND reltype    = '1'.
    SORT names.

    LOOP AT names INTO name.
      TRY.
          CREATE OBJECT feature TYPE (name).
          APPEND feature TO result.
        CATCH cx_root ##CATCH_ALL ##NO_HANDLER.
      ENDTRY.
    ENDLOOP.
  ENDMETHOD.


  METHOD get_feature_for_uri.
    DATA features TYPE tt_features.
    DATA feature TYPE REF TO zif_vsp_adt_feature.
    DATA prefix TYPE string.
    DATA next TYPE string.
    DATA path TYPE string.

    path = uri.
    SPLIT path AT '?' INTO path next.

    features = get_features( ).
    LOOP AT features INTO feature.
      prefix = zif_vsp_adt_feature=>co_root && feature->get_path( ).
      IF find( val = path sub = prefix case = abap_false ) <> 0.
        CONTINUE.
      ENDIF.
      IF strlen( path ) = strlen( prefix ).
        result = feature.
        RETURN.
      ENDIF.
      next = substring( val = path off = strlen( prefix ) len = 1 ).
      IF next = '/'.
        result = feature.
        RETURN.
      ENDIF.
    ENDLOOP.
  ENDMETHOD.

ENDCLASS.
