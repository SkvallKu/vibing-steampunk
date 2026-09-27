"! <p class="shorttext synchronized">vsp ADT fallback: structure source, ours</p>
"! The structure source for a release without the standard's (7.40):
"! the same "define type" language, read and written in plain ABAP.
CLASS zcl_vsp_ddic_src_own DEFINITION
  PUBLIC
  FINAL
  CREATE PUBLIC.

  PUBLIC SECTION.
    INTERFACES zif_vsp_ddic_source.

  PROTECTED SECTION.
  PRIVATE SECTION.
ENDCLASS.



CLASS zcl_vsp_ddic_src_own IMPLEMENTATION.

  METHOD zif_vsp_ddic_source~parse.
    zcl_vsp_adt_txt_format=>raise_not_acceptable(
      'Structure source is not available on this release yet' ).
  ENDMETHOD.


  METHOD zif_vsp_ddic_source~render.
    CLEAR result.
  ENDMETHOD.
ENDCLASS.
