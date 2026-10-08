"! <p class="shorttext synchronized">vsp ADT fallback: GUI statuses</p>
"! /sap/bc/adt/zvsp/cua/programs/{name}: the GUI statuses and titles of a
"! program (its CUA, what SE41 edits), as abapGit serializes them in a
"! .prog.xml, the CUA node: ADM and the tables STA, FUN, MEN, MTX, ACT,
"! BUT, PFK, SET, DOC, TIT and BIV, rows named after their RSMPE_*
"! structure, initial fields left out.
"! <ul>
"! <li>GET [?version=active|inactive]: the document; without a version the
"!     inactive one when it is in the caller's working area. A program
"!     without a CUA gives an empty CUA node;</li>
"! <li>POST ?_action=LOCK|UNLOCK: the lock SE41 takes (ESCUAPAINT);</li>
"! <li>PUT ?lockHandle=..&amp;corrNr=..: replaces the whole CUA with the
"!     document, as the inactive version, and answers with it.</li>
"! </ul>
"! The CUA is read and written in the program's master language, as
"! abapGit does; it is activated as object CUAD through
"! /sap/bc/adt/activation. RS_CUA_INTERNAL_WRITE is called as abapGit
"! calls it, as from SE41 (SAP Note 2159455).
CLASS zcl_vsp_adt_cua_resource DEFINITION
  PUBLIC
  INHERITING FROM cl_adt_rest_resource
  FINAL
  CREATE PUBLIC.

  PUBLIC SECTION.
    CONSTANTS co_content_type TYPE string
      VALUE 'application/vnd.sap.as+xml; charset=utf-8; dataname=com.sap.adt.zvsp.cua' ##NO_TEXT.

    TYPES:
      BEGIN OF ty_cua,
        adm TYPE rsmpe_adm,
        sta TYPE STANDARD TABLE OF rsmpe_stat WITH DEFAULT KEY,
        fun TYPE STANDARD TABLE OF rsmpe_funt WITH DEFAULT KEY,
        men TYPE STANDARD TABLE OF rsmpe_men WITH DEFAULT KEY,
        mtx TYPE STANDARD TABLE OF rsmpe_mnlt WITH DEFAULT KEY,
        act TYPE STANDARD TABLE OF rsmpe_act WITH DEFAULT KEY,
        but TYPE STANDARD TABLE OF rsmpe_but WITH DEFAULT KEY,
        pfk TYPE STANDARD TABLE OF rsmpe_pfk WITH DEFAULT KEY,
        set TYPE STANDARD TABLE OF rsmpe_staf WITH DEFAULT KEY,
        doc TYPE STANDARD TABLE OF rsmpe_atrt WITH DEFAULT KEY,
        tit TYPE STANDARD TABLE OF rsmpe_titt WITH DEFAULT KEY,
        biv TYPE STANDARD TABLE OF rsmpe_buts WITH DEFAULT KEY,
      END OF ty_cua.

    METHODS get REDEFINITION.
    METHODS post REDEFINITION.
    METHODS put REDEFINITION.

  PROTECTED SECTION.
  PRIVATE SECTION.
    DATA program TYPE trdir-name.
    DATA devclass TYPE devclass.
    DATA language TYPE sylangu.

    METHODS parse_uri
      IMPORTING request TYPE REF TO if_adt_rest_request
      RAISING   cx_adt_rest.

    METHODS lock
      IMPORTING response TYPE REF TO if_adt_rest_response
      RAISING   cx_adt_rest.

    METHODS unlock.

    METHODS check_handle
      IMPORTING request TYPE REF TO if_adt_rest_request
      RAISING   cx_adt_rest.

    METHODS resolve_version
      IMPORTING request       TYPE REF TO if_adt_rest_request
      RETURNING VALUE(result) TYPE r3state
      RAISING   cx_adt_rest.

    METHODS read
      IMPORTING state         TYPE r3state
      RETURNING VALUE(result) TYPE ty_cua
      RAISING   cx_adt_rest.

    METHODS parse_body
      IMPORTING request       TYPE REF TO if_adt_rest_request
      RETURNING VALUE(result) TYPE ty_cua
      RAISING   cx_adt_rest.

    METHODS record_in_transport
      IMPORTING request TYPE REF TO if_adt_rest_request
      RAISING   cx_adt_rest.

    METHODS save
      IMPORTING cua TYPE ty_cua
      RAISING   cx_adt_rest.

    METHODS respond
      IMPORTING response TYPE REF TO if_adt_rest_response
                cua      TYPE ty_cua.

    METHODS fail
      IMPORTING text TYPE csequence OPTIONAL
      RAISING   cx_adt_rest.
ENDCLASS.



CLASS zcl_vsp_adt_cua_resource IMPLEMENTATION.

  METHOD get.
    DATA state TYPE r3state.

    parse_uri( request ).
    state = resolve_version( request ).
    respond( response = response cua = read( state ) ).
  ENDMETHOD.


  METHOD post.
    DATA action TYPE string.

    parse_uri( request ).
    request->get_uri_query_parameter(
      EXPORTING name      = '_action'
                mandatory = abap_true
      IMPORTING value     = action ).
    TRANSLATE action TO UPPER CASE.

    CASE action.
      WHEN 'LOCK'.
        lock( response ).
      WHEN 'UNLOCK'.
        unlock( ).
      WHEN OTHERS.
        RAISE EXCEPTION TYPE cx_adt_res_meth_not_supported.
    ENDCASE.
  ENDMETHOD.


  METHOD put.
    DATA cua TYPE ty_cua.

    parse_uri( request ).
    check_handle( request ).
    cua = parse_body( request ).
    IF cua-adm IS INITIAL AND cua-sta IS INITIAL AND cua-tit IS INITIAL.
      fail( 'The document has no CUA: neither ADM, nor statuses, nor titles. Deleting a CUA is not served' ).
    ENDIF.
    record_in_transport( request ).
    save( cua ).
    respond( response = response cua = read( 'I' ) ).
  ENDMETHOD.


  METHOD parse_uri.
    DATA value TYPE string.
    DATA subc TYPE trdir-subc.
    DATA masterlang TYPE masterlang.

    request->get_uri_attribute(
      EXPORTING name      = 'name'
                mandatory = abap_true
      IMPORTING value     = value ).
    value = cl_http_utility=>unescape_url( value ).
    TRANSLATE value TO UPPER CASE.
    IF strlen( value ) > 40.
      RAISE EXCEPTION TYPE cx_adt_res_not_found.
    ENDIF.
    program = value.

    SELECT SINGLE subc FROM trdir INTO subc WHERE name = program.
    IF sy-subrc <> 0.
      RAISE EXCEPTION TYPE cx_adt_res_not_found
        EXPORTING
          resource_type = 'PROG'
          resource_id   = CONV #( program ).
    ENDIF.
    " Executable programs and module pools have a CUA of their own; the
    " CUA of a function group belongs to the group, served elsewhere.
    IF subc <> '1' AND subc <> 'M'.
      fail( |{ program } is a program of type { subc }: only executable programs (1) and module pools (M) are served| ).
    ENDIF.

    SELECT SINGLE devclass masterlang FROM tadir INTO (devclass, masterlang)
      WHERE pgmid    = 'R3TR'
        AND object   = 'PROG'
        AND obj_name = program.
    language = masterlang.
    IF language IS INITIAL.
      language = sy-langu.
    ENDIF.
  ENDMETHOD.


  METHOD lock.
    DATA result TYPE sadt_object_lock_result.
    DATA transport TYPE sadt_object_lock_result.
    DATA check_result TYPE cts_check_result.
    DATA messages TYPE cts_messages.
    DATA message TYPE cts_message.

    CALL FUNCTION 'RS_ACCESS_PERMISSION'
      EXPORTING
        authority_check                = abap_true
        global_lock                    = space
        mode                           = 'MODIFY'
        object                         = program
        object_class                   = 'CUAD'
        suppress_corr_check            = abap_true
        suppress_corr_check_altogether = abap_true
        suppress_language_check        = abap_true
        suppress_editor_lock_check     = abap_false
        suppress_extend_dialog         = abap_true
      EXCEPTIONS
        canceled_in_corr               = 1
        enqueued_by_user               = 2
        enqueue_system_failure         = 3
        illegal_parameter_values       = 4
        locked_by_author               = 5
        no_modify_permission           = 6
        no_show_permission             = 7
        permission_failure             = 8
        OTHERS                         = 9.
    CASE sy-subrc.
      WHEN 0.
      WHEN 6 OR 7 OR 8.
        RAISE EXCEPTION TYPE cx_adt_res_no_authorization
          EXPORTING
            textid        = cx_adt_rest=>create_textid_from_msg_params( )
            resource_type = 'CUAD'
            resource_id   = CONV #( program ).
      WHEN OTHERS.
        fail( ).
    ENDCASE.

    result-lock_handle = zcl_vsp_adt_util=>lock_handle( program ).
    IF result-lock_handle IS INITIAL.
      unlock( ).
      fail( |No lock entry for the GUI statuses of { program } after locking them| ).
    ENDIF.

    " A system or software component closed for changes says so here.
    cl_adt_cts_management=>check_objects_in_request(
      EXPORTING
        pgmid        = 'LIMU'
        object       = 'CUAD'
        obj_name     = CONV #( program )
      IMPORTING
        check_result = check_result
        messages     = messages ).
    IF check_result-result <> 'S' AND messages IS NOT INITIAL.
      unlock( ).
      READ TABLE messages INTO message WITH KEY severity = 'E'.
      IF sy-subrc <> 0.
        READ TABLE messages INTO message INDEX 1.
      ENDIF.
      fail( message-text ).
    ENDIF.

    transport = cl_adt_cts_management=>get_transport_info(
      pgmid    = 'LIMU'
      obj_type = 'CUAD'
      obj_name = CONV #( program ) ).
    result-corrnr = transport-corrnr.
    result-corruser = transport-corruser.
    result-corrtext = transport-corrtext.
    result-is_link_up = transport-is_link_up.
    result-is_local = cl_adt_cts_management=>is_transport_disabled_for_obj(
      pgmid    = 'LIMU'
      object   = 'CUAD'
      obj_name = CONV #( program ) ).
    zcl_vsp_adt_util=>respond_lock( response = response result = result ).
  ENDMETHOD.


  METHOD unlock.
    CALL FUNCTION 'RS_ACCESS_PERMISSION'
      EXPORTING
        authority_check = space
        global_lock     = space
        mode            = 'FREE'
        object          = program
        object_class    = 'CUAD'
      EXCEPTIONS
        OTHERS          = 1.
  ENDMETHOD.


  METHOD check_handle.
    DATA handle TYPE string.

    request->get_uri_query_parameter(
      EXPORTING name      = 'lockHandle'
                mandatory = abap_true
      IMPORTING value     = handle ).
    IF handle IS INITIAL OR handle <> zcl_vsp_adt_util=>lock_handle( program ).
      RAISE EXCEPTION TYPE cx_adt_res_invalid_lock_handle
        EXPORTING
          resource_type = 'CUAD'
          resource_id   = CONV #( program )
          lock_handle   = handle.
    ENDIF.
  ENDMETHOD.


  METHOD resolve_version.
    DATA version TYPE string.
    DATA name TYPE eudb-name.

    request->get_uri_query_parameter(
      EXPORTING name      = 'version'
                mandatory = abap_false
      IMPORTING value     = version ).
    TRANSLATE version TO LOWER CASE.
    CASE version.
      WHEN 'active'.
        result = 'A'.
      WHEN 'inactive'.
        result = 'I'.
      WHEN OTHERS.
        IF cl_wb_activation_db_api=>is_own_item(
             objtype_tr = 'CUAD'
             objname_tr = CONV #( program ) ) = abap_true.
          result = 'I'.
        ELSE.
          result = 'A'.
        ENDIF.
    ENDCASE.

    " No inactive version: the active one is the newest.
    IF result = 'I'.
      SELECT SINGLE name FROM eudb INTO name
        WHERE relid = 'IA' AND name = program AND sprsl = 'D' AND srtf2 = 0.
      IF sy-subrc <> 0.
        result = 'A'.
      ENDIF.
    ENDIF.
  ENDMETHOD.


  METHOD read.
    " NOT_FOUND is a program without a CUA: an empty document. 7.50 gives
    " sy-subrc 0 and empty tables instead; both read the same.
    CALL FUNCTION 'RS_CUA_INTERNAL_FETCH'
      EXPORTING
        program         = program
        language        = language
        state           = state
      IMPORTING
        adm             = result-adm
      TABLES
        sta             = result-sta
        fun             = result-fun
        men             = result-men
        mtx             = result-mtx
        act             = result-act
        but             = result-but
        pfk             = result-pfk
        set             = result-set
        doc             = result-doc
        tit             = result-tit
        biv             = result-biv
      EXCEPTIONS
        not_found       = 1
        unknown_version = 2
        OTHERS          = 3.
    IF sy-subrc > 1.
      fail( ).
    ENDIF.
  ENDMETHOD.


  METHOD parse_body.
    DATA xml TYPE xstring.
    DATA error TYPE REF TO cx_transformation_error.

    xml = request->get_inner_rest_request( )->get_entity( )->get_binary_data( ).
    TRY.
        CALL TRANSFORMATION id SOURCE XML xml RESULT cua = result.
      CATCH cx_transformation_error INTO error.
        fail( |The document is not a CUA (asXML, root CUA): { error->get_text( ) }| ).
    ENDTRY.
  ENDMETHOD.


  METHOD record_in_transport.
    DATA manager TYPE REF TO if_adt_lifecycle_manager.
    DATA corrnr TYPE trkorr.
    DATA masterlang TYPE masterlang.
    DATA immediately TYPE c LENGTH 1.

    IF cl_adt_cts_management=>is_transport_disabled_for_obj(
         pgmid    = 'LIMU'
         object   = 'CUAD'
         obj_name = CONV #( program ) ) = abap_true.
      RETURN.
    ENDIF.

    request->get_uri_query_parameter(
      EXPORTING name      = 'corrNr'
                mandatory = abap_false
      IMPORTING value     = corrnr ).
    IF corrnr IS NOT INITIAL.
      immediately = abap_true.
    ENDIF.
    masterlang = language.

    manager = cl_adt_corr_insert_dark=>get_instance( ).
    manager->corrnr = corrnr.
    manager->devclass = devclass.
    manager->corr_insert(
      EXPORTING
        pgmid                    = 'LIMU'
        obj_name                 = program
        obj_type                 = 'CUAD'
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
    DATA tr_key TYPE trkey.
    DATA copy TYPE ty_cua.
    DATA tcode TYPE sytcode.

    copy = cua.
    tr_key-devclass = devclass.
    tr_key-obj_type = 'PROG'.
    tr_key-obj_name = program.
    tr_key-sub_type = 'CUAD'.
    tr_key-sub_name = program.

    " Without SE41 the function checks for a dialog call that does not
    " apply here (SAP Note 2159455); abapGit does the same.
    tcode = sy-tcode.
    sy-tcode = 'SE41' ##WRITE_OK.
    CALL FUNCTION 'RS_CUA_INTERNAL_WRITE'
      EXPORTING
        program      = program
        language     = language
        tr_key       = tr_key
        adm          = copy-adm
        state        = 'I'
      TABLES
        sta          = copy-sta
        fun          = copy-fun
        men          = copy-men
        mtx          = copy-mtx
        act          = copy-act
        but          = copy-but
        pfk          = copy-pfk
        set          = copy-set
        doc          = copy-doc
        tit          = copy-tit
        biv          = copy-biv
      EXCEPTIONS
        not_found    = 1
        invalid_data = 2
        OTHERS       = 3.
    sy-tcode = tcode ##WRITE_OK.
    IF sy-subrc <> 0.
      fail( ).
    ENDIF.
  ENDMETHOD.


  METHOD respond.
    DATA xml TYPE xstring.
    DATA entity TYPE REF TO if_rest_entity.

    CALL TRANSFORMATION id
      OPTIONS initial_components = 'suppress'
      SOURCE cua = cua
      RESULT XML xml.
    entity = response->get_inner_rest_response( )->create_entity( ).
    entity->set_content_type( iv_media_type = co_content_type ).
    entity->set_binary_data( xml ).
  ENDMETHOD.


  METHOD fail.
    zcl_vsp_adt_util=>raise_failure( resource_type = 'CUAD'
                                     resource_id   = program
                                     text          = text ).
  ENDMETHOD.

ENDCLASS.
