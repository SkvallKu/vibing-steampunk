"! <p class="shorttext synchronized">vsp ADT fallback: /info</p>
"! GET /sap/bc/adt/zvsp/info answers in plain text what is installed, so
"! vsp can tell whether this set is older than its own:
"!   version=0.1.0
"!   features=/textelements
CLASS zcl_vsp_adt_res_info DEFINITION
  PUBLIC
  INHERITING FROM cl_adt_rest_resource
  FINAL
  CREATE PUBLIC.

  PUBLIC SECTION.
    METHODS get REDEFINITION.

  PROTECTED SECTION.
  PRIVATE SECTION.
ENDCLASS.



CLASS zcl_vsp_adt_res_info IMPLEMENTATION.

  METHOD get.
    DATA features TYPE zcl_vsp_adt_app=>tt_features.
    DATA feature TYPE REF TO zif_vsp_adt_feature.
    DATA paths TYPE string.
    DATA text TYPE string.
    DATA entity TYPE REF TO if_rest_entity.

    features = zcl_vsp_adt_app=>get_features( ).
    LOOP AT features INTO feature.
      IF paths IS NOT INITIAL.
        paths = paths && ','.
      ENDIF.
      paths = paths && feature->get_path( ).
    ENDLOOP.

    text = |version={ zcl_vsp_adt_app=>co_version }\n| &&
           |features={ paths }\n|.

    entity = response->get_inner_rest_response( )->create_entity( ).
    entity->set_content_type( iv_media_type = 'text/plain' ).
    entity->set_string_data( text ).
  ENDMETHOD.

ENDCLASS.
