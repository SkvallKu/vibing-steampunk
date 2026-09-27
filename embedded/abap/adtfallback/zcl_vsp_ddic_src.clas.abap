"! <p class="shorttext synchronized">vsp ADT fallback: which structure source</p>
"! The standard's structure source where the release has it, ours where
"! it has not. A resource may ask for one by name (?parser=sap|own), so
"! that ours can be compared with the standard's on a release that has
"! both.
CLASS zcl_vsp_ddic_src DEFINITION
  PUBLIC
  FINAL
  CREATE PRIVATE.

  PUBLIC SECTION.
    CONSTANTS co_sap TYPE string VALUE 'SAP'.
    CONSTANTS co_own TYPE string VALUE 'OWN'.

    "! KIND empty: the standard's if there is one, else ours.
    CLASS-METHODS get
      IMPORTING kind          TYPE csequence OPTIONAL
      RETURNING VALUE(result) TYPE REF TO zif_vsp_ddic_source
      RAISING   cx_adt_res_not_acceptable.

    "! The ?parser= of a request, upper case; empty when not given.
    CLASS-METHODS kind_of
      IMPORTING request       TYPE REF TO if_adt_rest_request
      RETURNING VALUE(result) TYPE string
      RAISING   cx_adt_rest.

  PROTECTED SECTION.
  PRIVATE SECTION.
ENDCLASS.



CLASS zcl_vsp_ddic_src IMPLEMENTATION.

  METHOD get.
    DATA wanted TYPE string.

    wanted = to_upper( kind ).
    IF wanted IS INITIAL.
      IF zcl_vsp_ddic_src_sap=>is_available( ) = abap_true.
        wanted = co_sap.
      ELSE.
        wanted = co_own.
      ENDIF.
    ENDIF.
    CASE wanted.
      WHEN co_sap.
        IF zcl_vsp_ddic_src_sap=>is_available( ) = abap_false.
          zcl_vsp_adt_txt_format=>raise_not_acceptable(
            'This release has no standard structure source; parser=own' ).
        ENDIF.
        CREATE OBJECT result TYPE zcl_vsp_ddic_src_sap.
      WHEN co_own.
        CREATE OBJECT result TYPE zcl_vsp_ddic_src_own.
      WHEN OTHERS.
        zcl_vsp_adt_txt_format=>raise_not_acceptable(
          |Unknown parser { kind }: sap or own| ).
    ENDCASE.
  ENDMETHOD.


  METHOD kind_of.
    request->get_uri_query_parameter(
      EXPORTING name      = 'parser'
                mandatory = abap_false
      IMPORTING value     = result ).
    result = to_upper( result ).
  ENDMETHOD.
ENDCLASS.
