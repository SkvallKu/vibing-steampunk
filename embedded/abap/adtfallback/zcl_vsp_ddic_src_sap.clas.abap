"! <p class="shorttext synchronized">vsp ADT fallback: structure source, SAP's</p>
"! The standard's structure source (7.50: CL_SBD_STRUCTURE_PARSER,
"! CL_SBD_STRUCTURE_PERSIST), called only by name. Nothing here names
"! those classes as types, so the class compiles on a release without
"! them (7.40) and is simply not chosen there. The logger's methods are
"! called by name too: 7.40's CL_WB_CHECKLIST has no has_error_messages.
CLASS zcl_vsp_ddic_src_sap DEFINITION
  PUBLIC
  FINAL
  CREATE PUBLIC.

  PUBLIC SECTION.
    INTERFACES zif_vsp_ddic_source.

    CONSTANTS co_parser TYPE seoclsname VALUE 'CL_SBD_STRUCTURE_PARSER'.
    CONSTANTS co_persist TYPE seoclsname VALUE 'CL_SBD_STRUCTURE_PERSIST'.

    "! Whether this release has the standard's structure source.
    CLASS-METHODS is_available
      RETURNING VALUE(result) TYPE abap_bool.

  PROTECTED SECTION.
  PRIVATE SECTION.
ENDCLASS.



CLASS zcl_vsp_ddic_src_sap IMPLEMENTATION.

  METHOD is_available.
    cl_abap_typedescr=>describe_by_name(
      EXPORTING  p_name         = co_parser
      EXCEPTIONS type_not_found = 1
                 OTHERS         = 2 ).
    result = boolc( sy-subrc = 0 ).
  ENDMETHOD.


  METHOD zif_vsp_ddic_source~parse.
    DATA logger TYPE REF TO cl_wb_checklist.
    DATA parser TYPE REF TO object.
    DATA has_errors TYPE abap_bool.
    DATA fk_fields TYPE dd05mttyp.
    DATA message TYPE rsfb_source.
    DATA message_line TYPE LINE OF rsfb_source.
    DATA text TYPE string.
    DATA error_line TYPE i.

    CREATE OBJECT logger.
    CALL METHOD (co_parser)=>('CREATE')
      EXPORTING
        i_logger      = logger
        i_object_name = name
      RECEIVING
        r_result      = parser.
    CALL METHOD parser->('SCAN_SOURCE_TABLE')
      EXPORTING
        i_source_code = source.
    CALL METHOD logger->('HAS_ERROR_MESSAGES') RECEIVING result = has_errors.
    IF has_errors = abap_true.
      CALL METHOD logger->('GET_ERROR_MESSAGE')
        EXPORTING
          p_index        = 1
        IMPORTING
          p_message_text = message
          p_line         = error_line
        EXCEPTIONS
          OTHERS         = 1.
      LOOP AT message INTO message_line.
        text = text && message_line && ` `.
      ENDLOOP.
      IF error_line > 0.
        text = |Line { error_line }: { text }|.
      ENDIF.
      zcl_vsp_adt_txt_format=>raise_not_acceptable( text ).
    ENDIF.

    CALL METHOD parser->('GET_HEADER') RECEIVING r_result = result-header.
    CALL METHOD parser->('GET_FIELDS') RECEIVING r_result = result-fields.
    CALL METHOD parser->('GET_FOREIGN_KEYS') RECEIVING r_result = result-foreign_keys.
    CALL METHOD parser->('GET_FOREIGN_KEY_FIELDS') RECEIVING r_result = fk_fields.
    CALL METHOD parser->('GET_VALUE_HELPS') RECEIVING r_result = result-value_helps.
    CALL METHOD parser->('GET_VALUE_HELP_FIELDS') RECEIVING r_result = result-value_help_fields.
    " As the structure editor saves: the dictionary expects every key
    " field of the check table, generic ones included.
    CALL METHOD (co_persist)=>('ADJUST_FOREIGN_KEY_FIELDS')
      EXPORTING
        i_foreign_key_fields = fk_fields
        i_foreign_keys       = result-foreign_keys
      RECEIVING
        r_result             = result-foreign_key_fields.
  ENDMETHOD.


  METHOD zif_vsp_ddic_source~render.
    DATA persist TYPE REF TO object.
    DATA key TYPE seu_objkey.

    CREATE OBJECT persist TYPE (co_persist).
    key = name.
    CALL METHOD persist->('GET_SOURCE')
      EXPORTING
        i_object_key = key
        i_version    = version
      IMPORTING
        e_source     = result.
  ENDMETHOD.
ENDCLASS.
