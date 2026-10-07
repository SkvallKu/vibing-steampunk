"! <p class="shorttext synchronized">vsp ADT fallback: component texts</p>
"! The short texts of the components of a class or interface (methods,
"! attributes, types, events) and of their parameters and exceptions.
"! From 7.50 the class source scanner takes them from ABAP Doc
"! (shorttext synchronized) when the source is saved; 7.40 has no such
"! scanner, so a source written through ADT leaves them empty. There is no
"! standard ADT resource for them on any release, so this one is offered
"! in discovery only where the scanner is missing: vsp writes the texts
"! here when discovery has it, and leaves them to the scanner otherwise.
"! The resource itself is served everywhere, for testing.
CLASS zcl_vsp_adt_cltx_feature DEFINITION
  PUBLIC
  FINAL
  CREATE PUBLIC.

  PUBLIC SECTION.
    INTERFACES zif_vsp_adt_feature.

    CONSTANTS co_path TYPE string VALUE '/oo/componenttexts'.

    "! True when saving a class source takes the short texts from ABAP
    "! Doc: CL_SEDI_ABAP_DOC_SHORTTEXT, which the scanner calls, exists.
    CLASS-METHODS source_syncs_texts
      RETURNING VALUE(result) TYPE abap_bool.

  PROTECTED SECTION.
  PRIVATE SECTION.
    CONSTANTS co_handler TYPE string VALUE 'ZCL_VSP_ADT_CLTX_RESOURCE'.
ENDCLASS.



CLASS zcl_vsp_adt_cltx_feature IMPLEMENTATION.

  METHOD zif_vsp_adt_feature~get_path.
    result = co_path.
  ENDMETHOD.


  METHOD zif_vsp_adt_feature~register_resources.
    IF source_syncs_texts( ) = abap_false.
      registry->register_discoverable_resource(
        url             = co_path
        handler_class   = co_handler
        description     = 'Component texts of classes and interfaces'
        category_scheme = zcl_vsp_adt_app=>co_scheme
        category_term   = 'oo/componenttexts' ) ##NO_TEXT.
    ENDIF.
    registry->register_resource(
      template      = co_path && '/{name}'
      handler_class = co_handler ).
  ENDMETHOD.


  METHOD zif_vsp_adt_feature~map_to_wb_object.
    " Texts are not versioned: there is nothing to activate.
    RAISE EXCEPTION TYPE cx_adt_uri_mapping
      EXPORTING
        textid = cx_adt_uri_mapping=>invalid_uri
        uri    = CONV #( uri ).
  ENDMETHOD.


  METHOD source_syncs_texts.
    DATA name TYPE seoclsname.

    SELECT SINGLE clsname FROM seoclass INTO name
      WHERE clsname = 'CL_SEDI_ABAP_DOC_SHORTTEXT'.
    IF sy-subrc = 0.
      result = abap_true.
    ENDIF.
  ENDMETHOD.

ENDCLASS.
