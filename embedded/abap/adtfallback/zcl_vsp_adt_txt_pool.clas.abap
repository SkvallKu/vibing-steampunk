"! <p class="shorttext synchronized">vsp ADT fallback: one text pool</p>
"! The text pool of a program or class pool in its master language, as
"! the standard resource does (the RFC tunnel carries no request language
"! either, so the logon language would be a guess): which
"! version to read, the documents of each kind, their etags, and the write.
"! A write goes to the inactive version and into the working area, as SE38
"! does it, and is activated as object REPT through /sap/bc/adt/activation.
"! RPY_TEXTELEMENTS_INSERT(_AIE) is not used: before 7.5x it writes the
"! active version and frees the lock at the end, which would break the
"! next PUT made under the same lock.
CLASS zcl_vsp_adt_txt_pool DEFINITION
  PUBLIC
  FINAL
  CREATE PUBLIC.

  PUBLIC SECTION.
    CONSTANTS:
      BEGIN OF co_id,
        symbols       TYPE textpool-id VALUE 'I',
        selections    TYPE textpool-id VALUE 'S',
        list_header   TYPE textpool-id VALUE 'T',
        column_header TYPE textpool-id VALUE 'H',
      END OF co_id.

    "! OBJECT_TYPE is PROG or CLAS, OBJECT_NAME the program or class,
    "! REPT_NAME the program or class pool that owns the text pool.
    METHODS constructor
      IMPORTING object_type TYPE trobjtype
                object_name TYPE csequence
                rept_name   TYPE syrepid.

    METHODS exists
      RETURNING VALUE(result) TYPE abap_bool.

    "! The version a read without ?version= means: the inactive one if it
    "! is in the caller's working area and exists, the active one otherwise.
    METHODS resolve_version
      IMPORTING requested     TYPE r3state OPTIONAL
      RETURNING VALUE(result) TYPE r3state.

    METHODS read
      IMPORTING version TYPE r3state.

    METHODS get_document
      IMPORTING kind          TYPE string
      RETURNING VALUE(result) TYPE zcl_vsp_adt_txt_format=>ty_document.

    METHODS get_etag
      IMPORTING kind          TYPE string
      RETURNING VALUE(result) TYPE string.

    "! Records the text pool in a transport request (none for a local
    "! object), as the standard resource does before it writes.
    METHODS record_in_transport
      CHANGING corrnr TYPE trkorr
      RAISING  cx_adt_rest.

    "! Replaces the entries of one kind with those of the document and
    "! writes the result as the inactive version.
    METHODS save
      IMPORTING kind     TYPE string
                document TYPE zcl_vsp_adt_txt_format=>ty_document
      RAISING   cx_adt_rest.

  PROTECTED SECTION.
  PRIVATE SECTION.
    TYPES tt_textpool TYPE STANDARD TABLE OF textpool WITH DEFAULT KEY.

    DATA object_type TYPE trobjtype.
    DATA object_name TYPE sobj_name.
    DATA rept_name TYPE syrepid.
    DATA language TYPE sylangu.
    DATA version TYPE r3state.
    DATA tpool TYPE tt_textpool.

    METHODS merge
      IMPORTING id      TYPE textpool-id
                entries TYPE zcl_vsp_adt_txt_format=>tt_entries.

    METHODS set_heading
      IMPORTING id   TYPE textpool-id
                key  TYPE textpool-key
                text TYPE string.
ENDCLASS.



CLASS zcl_vsp_adt_txt_pool IMPLEMENTATION.

  METHOD constructor.
    me->object_type = object_type.
    me->object_name = object_name.
    me->rept_name = rept_name.

    SELECT SINGLE masterlang FROM tadir INTO language
      WHERE pgmid    = 'R3TR'
        AND object   = object_type
        AND obj_name = object_name.
    IF sy-subrc <> 0 OR language IS INITIAL.
      language = sy-langu.
    ENDIF.
  ENDMETHOD.


  METHOD exists.
    DATA progname TYPE progname.

    SELECT SINGLE progname FROM reposrc INTO progname
      WHERE progname = rept_name
        AND r3state  = 'A'.
    IF sy-subrc = 0.
      result = abap_true.
    ENDIF.
  ENDMETHOD.


  METHOD resolve_version.
    DATA inactive TYPE tt_textpool.
    DATA in_db TYPE r3state.

    READ TEXTPOOL rept_name INTO inactive LANGUAGE language STATE 'I'.
    IF sy-subrc = 0.
      in_db = 'I'.
    ELSE.
      in_db = 'A'.
    ENDIF.

    result = requested.
    IF result IS INITIAL.
      IF cl_wb_activation_db_api=>is_own_item(
           objtype_tr = 'REPT'
           objname_tr = CONV #( rept_name ) ) = abap_true.
        result = 'I'.
      ELSE.
        result = 'A'.
      ENDIF.
    ENDIF.
    IF result = 'I' AND in_db = 'A'.
      result = 'A'.
    ENDIF.
  ENDMETHOD.


  METHOD read.
    me->version = version.
    CLEAR tpool.
    READ TEXTPOOL rept_name INTO tpool LANGUAGE language STATE version.
    SORT tpool BY id key.
  ENDMETHOD.


  METHOD get_document.
    DATA line TYPE textpool.
    DATA entry TYPE zcl_vsp_adt_txt_format=>ty_entry.
    DATA params TYPE STANDARD TABLE OF rsel_paras WITH DEFAULT KEY.
    DATA param TYPE rsel_paras.
    DATA ddic TYPE tt_textpool.
    FIELD-SYMBOLS <entry> TYPE zcl_vsp_adt_txt_format=>ty_entry.

    LOOP AT tpool INTO line.
      CLEAR entry.
      entry-key = line-key.
      entry-entry = line-entry.
      entry-length = line-length.
      CASE line-id.
        WHEN co_id-symbols.
          APPEND entry TO result-symbols.
        WHEN co_id-selections.
          APPEND entry TO result-selections.
        WHEN co_id-list_header.
          result-list_header = line-entry.
        WHEN co_id-column_header.
          CASE line-key.
            WHEN '001'.
              result-column_header_1 = line-entry.
            WHEN '002'.
              result-column_header_2 = line-entry.
            WHEN '003'.
              result-column_header_3 = line-entry.
            WHEN '004'.
              result-column_header_4 = line-entry.
          ENDCASE.
      ENDCASE.
    ENDLOOP.

    " A selection text taken from the dictionary is stored as a
    " placeholder; RS_TEXTPOOL_READ puts the dictionary text in, as the
    " standard resource shows it. The version does not matter for it.
    IF kind = zcl_vsp_adt_txt_format=>co_kind-selections.
      CALL FUNCTION 'RS_TEXTPOOL_READ'
        EXPORTING
          objectname      = rept_name
          action          = 'SHOW'
          authority_check = space
          language        = language
        TABLES
          tpool           = ddic
        EXCEPTIONS
          OTHERS          = 1.
      IF sy-subrc = 0.
        LOOP AT result-selections ASSIGNING <entry>
             WHERE entry(8) = zcl_vsp_adt_txt_format=>co_entry_key_ddic.
          READ TABLE ddic INTO line WITH KEY id = co_id-selections key = <entry>-key.
          IF sy-subrc = 0 AND line-entry(8) = zcl_vsp_adt_txt_format=>co_entry_key_ddic.
            <entry>-entry = line-entry.
          ENDIF.
        ENDLOOP.
      ENDIF.
    ENDIF.

    " A parameter or select-option without a text yet is offered as ?...
    IF kind = zcl_vsp_adt_txt_format=>co_kind-selections AND object_type = 'PROG'.
      CALL FUNCTION 'SELOPTS_AND_PARAMS'
        EXPORTING
          program = rept_name
        TABLES
          selpars = params
        EXCEPTIONS
          OTHERS  = 1.
      LOOP AT params INTO param.
        READ TABLE result-selections TRANSPORTING NO FIELDS
          WITH KEY key = param-name.
        IF sy-subrc <> 0.
          CLEAR entry.
          entry-key = param-name.
          CONCATENATE zcl_vsp_adt_txt_format=>co_entry_key_non_ddic '?...'
            INTO entry-entry RESPECTING BLANKS.
          APPEND entry TO result-selections.
        ENDIF.
      ENDLOOP.
    ENDIF.
  ENDMETHOD.


  METHOD get_etag.
    DATA line TYPE textpool.
    DATA text TYPE string.

    LOOP AT tpool INTO line.
      CASE kind.
        WHEN zcl_vsp_adt_txt_format=>co_kind-symbols.
          CHECK line-id = co_id-symbols.
        WHEN zcl_vsp_adt_txt_format=>co_kind-selections.
          CHECK line-id = co_id-selections.
        WHEN zcl_vsp_adt_txt_format=>co_kind-headings.
          CHECK line-id = co_id-list_header OR line-id = co_id-column_header.
      ENDCASE.
      text = |{ text }{ line-key }{ line-length }{ line-entry }|.
    ENDLOOP.

    TRY.
        cl_abap_message_digest=>calculate_hash_for_char(
          EXPORTING if_data       = text
          IMPORTING ef_hashstring = result ).
      CATCH cx_abap_message_digest ##NO_HANDLER.
    ENDTRY.
    result = result && version.
  ENDMETHOD.


  METHOD record_in_transport.
    DATA manager TYPE REF TO if_adt_lifecycle_manager.
    DATA devclass TYPE devclass.
    DATA masterlang TYPE masterlang.
    DATA immediately TYPE c LENGTH 1.

    IF cl_adt_cts_management=>is_transport_disabled_for_obj(
         pgmid    = 'LIMU'
         object   = 'REPT'
         obj_name = CONV #( rept_name ) ) = abap_true.
      CLEAR corrnr.
      RETURN.
    ENDIF.

    SELECT SINGLE devclass masterlang FROM tadir INTO (devclass, masterlang)
      WHERE pgmid    = 'R3TR'
        AND object   = object_type
        AND obj_name = object_name.

    IF corrnr IS NOT INITIAL.
      immediately = abap_true.
    ENDIF.

    manager = cl_adt_corr_insert_dark=>get_instance( ).
    manager->corrnr = corrnr.
    manager->devclass = devclass.
    manager->corr_insert(
      EXPORTING
        pgmid                    = 'LIMU'
        obj_name                 = rept_name
        obj_type                 = 'REPT'
        mode                     = 'MODIFY'
        global_lock              = abap_true
        masterlang               = masterlang
        suppress_dialog          = abap_true
        use_korrnum_immediatedly = immediately
      CHANGING
        devclass                 = devclass
        korrnum                  = corrnr
      EXCEPTIONS
        OTHERS                   = 1 ).
    IF sy-subrc <> 0.
      RAISE EXCEPTION TYPE cx_adt_res_save_failure
        EXPORTING
          textid = cx_adt_rest=>create_textid_from_msg_params( ).
    ENDIF.
  ENDMETHOD.


  METHOD save.
    DATA state TYPE r3state.
    DATA dummy TYPE tt_textpool.

    " Start from what a reader in the working area sees.
    READ TEXTPOOL rept_name INTO dummy LANGUAGE language STATE 'I'.
    IF sy-subrc = 0.
      state = 'I'.
    ELSE.
      state = 'A'.
    ENDIF.
    read( state ).

    CASE kind.
      WHEN zcl_vsp_adt_txt_format=>co_kind-symbols.
        merge( id = co_id-symbols entries = document-symbols ).
      WHEN zcl_vsp_adt_txt_format=>co_kind-selections.
        merge( id = co_id-selections entries = document-selections ).
      WHEN zcl_vsp_adt_txt_format=>co_kind-headings.
        set_heading( id = co_id-list_header key = space text = document-list_header ).
        set_heading( id = co_id-column_header key = '001' text = document-column_header_1 ).
        set_heading( id = co_id-column_header key = '002' text = document-column_header_2 ).
        set_heading( id = co_id-column_header key = '003' text = document-column_header_3 ).
        set_heading( id = co_id-column_header key = '004' text = document-column_header_4 ).
    ENDCASE.

    SORT tpool BY id key.
    INSERT TEXTPOOL rept_name FROM tpool LANGUAGE language STATE 'I'.
    IF sy-subrc <> 0.
      zcl_vsp_adt_txt_format=>set_message( |Writing the text pool of { rept_name } failed| ).
      RAISE EXCEPTION TYPE cx_adt_res_save_failure
        EXPORTING
          textid = cx_adt_rest=>create_textid_from_msg_params( ).
    ENDIF.

    CALL FUNCTION 'RS_INSERT_INTO_WORKING_AREA'
      EXPORTING
        object            = 'REPT'
        obj_name          = CONV trobj_name( rept_name )
      EXCEPTIONS
        wrong_object_name = 1
        OTHERS            = 2.

    COMMIT WORK.
    read( 'I' ).
  ENDMETHOD.


  METHOD merge.
    DATA entry TYPE zcl_vsp_adt_txt_format=>ty_entry.
    DATA line TYPE textpool.
    FIELD-SYMBOLS <line> TYPE textpool.

    DELETE tpool WHERE id = id.
    LOOP AT entries INTO entry.
      CLEAR line.
      line-id = id.
      line-key = entry-key.
      line-entry = entry-entry.
      line-length = entry-length.
      IF line-length = 0.
        line-length = strlen( line-entry ).
      ENDIF.
      READ TABLE tpool ASSIGNING <line> WITH KEY id = id key = line-key.
      IF sy-subrc = 0.
        <line> = line.
      ELSE.
        APPEND line TO tpool.
      ENDIF.
    ENDLOOP.
  ENDMETHOD.


  METHOD set_heading.
    FIELD-SYMBOLS <line> TYPE textpool.
    DATA line TYPE textpool.

    READ TABLE tpool ASSIGNING <line> WITH KEY id = id key = key.
    IF sy-subrc = 0.
      IF text IS INITIAL.
        DELETE tpool INDEX sy-tabix.
      ELSE.
        <line>-entry = text.
        <line>-length = strlen( text ).
      ENDIF.
    ELSEIF text IS NOT INITIAL.
      line-id = id.
      line-key = key.
      line-entry = text.
      line-length = strlen( text ).
      APPEND line TO tpool.
    ENDIF.
  ENDMETHOD.

ENDCLASS.
