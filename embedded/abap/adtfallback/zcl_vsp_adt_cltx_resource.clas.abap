"! <p class="shorttext synchronized">vsp ADT fallback: component texts</p>
"! /sap/bc/adt/zvsp/oo/componenttexts/{name}, for a class or an interface:
"! <ul>
"! <li>GET: the short texts in the master language, one per line,
"!     component TAB subcomponent TAB text; the subcomponent (a parameter
"!     or an exception) is empty for the component's own text;</li>
"! <li>POST ?_action=LOCK|UNLOCK: the lock SE24 takes on the class;</li>
"! <li>PUT ?lockHandle=..&amp;corrNr=..: the same lines; each sets one text,
"!     an empty text deletes it, texts not named stay. Every line is
"!     checked before anything is written. The answer is the GET one.</li>
"! </ul>
"! The texts are written the way the Class Builder writes them, straight
"! into SEOCOMPOTX and SEOSUBCOTX: they have no versions, so nothing is
"! activated. The class definition (LIMU CLSD) or the interface is
"! recorded in the transport.
CLASS zcl_vsp_adt_cltx_resource DEFINITION
  PUBLIC
  INHERITING FROM cl_adt_rest_resource
  FINAL
  CREATE PUBLIC.

  PUBLIC SECTION.
    METHODS get REDEFINITION.
    METHODS post REDEFINITION.
    METHODS put REDEFINITION.

  PROTECTED SECTION.
  PRIVATE SECTION.
    TYPES:
      BEGIN OF ty_text,
        cmpname  TYPE seocmpname,
        sconame  TYPE seosconame,
        descript TYPE string,
      END OF ty_text,
      tt_texts TYPE STANDARD TABLE OF ty_text WITH DEFAULT KEY.

    CONSTANTS co_content_type TYPE string
      VALUE 'text/plain; charset=utf-8' ##NO_TEXT.
    CONSTANTS co_max_length TYPE i VALUE 60.

    DATA name TYPE seoclsname.
    DATA object_type TYPE trobjtype.
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

    METHODS parse_body
      IMPORTING request       TYPE REF TO if_adt_rest_request
      RETURNING VALUE(result) TYPE tt_texts
      RAISING   cx_adt_rest.

    METHODS check_texts
      IMPORTING texts TYPE tt_texts
      RAISING   cx_adt_rest.

    METHODS record_in_transport
      IMPORTING request TYPE REF TO if_adt_rest_request
      RAISING   cx_adt_rest.

    METHODS save
      IMPORTING texts TYPE tt_texts.

    METHODS respond
      IMPORTING response TYPE REF TO if_adt_rest_response.

    METHODS fail
      IMPORTING text TYPE csequence OPTIONAL
      RAISING   cx_adt_rest.
ENDCLASS.



CLASS zcl_vsp_adt_cltx_resource IMPLEMENTATION.

  METHOD get.
    parse_uri( request ).
    respond( response ).
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
    DATA texts TYPE tt_texts.

    parse_uri( request ).
    check_handle( request ).
    texts = parse_body( request ).
    check_texts( texts ).
    record_in_transport( request ).
    save( texts ).
    respond( response ).
  ENDMETHOD.


  METHOD parse_uri.
    DATA value TYPE string.
    DATA clstype TYPE seoclstype.

    request->get_uri_attribute(
      EXPORTING name      = 'name'
                mandatory = abap_true
      IMPORTING value     = value ).
    value = cl_http_utility=>unescape_url( value ).
    TRANSLATE value TO UPPER CASE.
    IF strlen( value ) > 30.
      RAISE EXCEPTION TYPE cx_adt_res_not_found.
    ENDIF.
    name = value.

    SELECT SINGLE clstype FROM seoclass INTO clstype WHERE clsname = name.
    IF sy-subrc <> 0.
      RAISE EXCEPTION TYPE cx_adt_res_not_found
        EXPORTING
          resource_type = 'CLAS'
          resource_id   = CONV #( name ).
    ENDIF.
    IF clstype = seoc_clstype_interface.
      object_type = 'INTF'.
    ELSE.
      object_type = 'CLAS'.
    ENDIF.

    SELECT SINGLE masterlang FROM tadir INTO language
      WHERE pgmid    = 'R3TR'
        AND object   = object_type
        AND obj_name = name.
    IF language IS INITIAL.
      language = sy-langu.
    ENDIF.
  ENDMETHOD.


  METHOD lock.
    DATA result TYPE sadt_object_lock_result.
    DATA transport TYPE sadt_object_lock_result.
    DATA cifkey TYPE seoclskey.

    cifkey-clsname = name.
    CALL FUNCTION 'SEO_CLIF_ACCESS_PERMISSION'
      EXPORTING
        cifkey                 = cifkey
        mode                   = seok_access_modify
        " The texts are written in the master language, whatever the
        " logon language is: the check would refuse a session in another.
        suppress_langu_check   = seox_true
        suppress_corr_check    = seox_true
        suppress_dialog        = seox_true
        suppress_extend_dialog = seox_true
        suppress_language_dialog = seox_true
      EXCEPTIONS
        no_access              = 1
        OTHERS                 = 2.
    IF sy-subrc <> 0.
      fail( ).
    ENDIF.

    result-lock_handle = zcl_vsp_adt_util=>lock_handle( name ).
    IF result-lock_handle IS INITIAL.
      unlock( ).
      fail( |No lock entry for { name } after locking it| ).
    ENDIF.
    transport = cl_adt_cts_management=>get_transport_info(
      pgmid    = 'R3TR'
      obj_type = object_type
      obj_name = CONV #( name ) ).
    result-corrnr = transport-corrnr.
    result-corruser = transport-corruser.
    result-corrtext = transport-corrtext.
    result-is_link_up = transport-is_link_up.
    result-is_local = cl_adt_cts_management=>is_transport_disabled_for_obj(
      object   = object_type
      obj_name = CONV #( name ) ).
    zcl_vsp_adt_util=>respond_lock( response = response result = result ).
  ENDMETHOD.


  METHOD unlock.
    DATA cifkey TYPE seoclskey.

    cifkey-clsname = name.
    CALL FUNCTION 'SEO_CLIF_ACCESS_PERMISSION'
      EXPORTING
        cifkey          = cifkey
        mode            = seok_access_free
        authority_check = seox_false
      EXCEPTIONS
        OTHERS          = 1.
  ENDMETHOD.


  METHOD check_handle.
    DATA handle TYPE string.

    request->get_uri_query_parameter(
      EXPORTING name      = 'lockHandle'
                mandatory = abap_true
      IMPORTING value     = handle ).
    IF handle IS INITIAL OR handle <> zcl_vsp_adt_util=>lock_handle( name ).
      RAISE EXCEPTION TYPE cx_adt_res_invalid_lock_handle
        EXPORTING
          resource_type = CONV #( object_type )
          resource_id   = CONV #( name )
          lock_handle   = handle.
    ENDIF.
  ENDMETHOD.


  METHOD parse_body.
    DATA lines TYPE string_table.
    DATA line TYPE string.
    DATA fields TYPE string_table.
    DATA text TYPE ty_text.
    DATA number TYPE i.

    lines = zcl_vsp_adt_util=>request_lines( request ).
    LOOP AT lines INTO line.
      number = sy-tabix.
      IF line IS INITIAL.
        CONTINUE.
      ENDIF.
      SPLIT line AT cl_abap_char_utilities=>horizontal_tab INTO TABLE fields.
      IF lines( fields ) < 2 OR lines( fields ) > 3.
        fail( |Line { number }: want component TAB subcomponent TAB text| ).
      ENDIF.
      CLEAR text.
      READ TABLE fields INDEX 1 INTO line.
      line = to_upper( condense( line ) ).
      IF line IS INITIAL OR strlen( line ) > 30.
        fail( |Line { number }: "{ line }" is not a component name| ).
      ENDIF.
      text-cmpname = line.
      READ TABLE fields INDEX 2 INTO line.
      line = to_upper( condense( line ) ).
      IF strlen( line ) > 30.
        fail( |Line { number }: "{ line }" is not a parameter or exception name| ).
      ENDIF.
      text-sconame = line.
      READ TABLE fields INDEX 3 INTO text-descript.
      text-descript = condense( text-descript ).
      APPEND text TO result.
    ENDLOOP.
  ENDMETHOD.


  METHOD check_texts.
    DATA text TYPE ty_text.
    DATA problems TYPE string.
    DATA cmpname TYPE seocmpname.
    DATA sconame TYPE seosconame.

    LOOP AT texts INTO text.
      IF strlen( text-descript ) > co_max_length.
        problems = |{ problems }{ text-cmpname } { text-sconame }: more than { co_max_length } characters; |.
        CONTINUE.
      ENDIF.
      IF text-sconame IS INITIAL.
        SELECT SINGLE cmpname FROM seocompo INTO cmpname
          WHERE clsname = name
            AND cmpname = text-cmpname.
      ELSE.
        SELECT SINGLE sconame FROM seosubco INTO sconame
          WHERE clsname = name
            AND cmpname = text-cmpname
            AND sconame = text-sconame.
      ENDIF.
      IF sy-subrc <> 0.
        problems = |{ problems }{ text-cmpname } { text-sconame }: no such component of { name }; |.
      ENDIF.
    ENDLOOP.
    IF problems IS NOT INITIAL.
      fail( |Nothing written: { problems }| ).
    ENDIF.
  ENDMETHOD.


  METHOD record_in_transport.
    DATA corrnr TYPE trkorr.
    DATA devclass TYPE devclass.
    DATA manager TYPE REF TO if_adt_lifecycle_manager.
    DATA cifkey TYPE seoclskey.
    DATA limu TYPE trobjtype.

    request->get_uri_query_parameter(
      EXPORTING name      = 'corrNr'
                mandatory = abap_false
      IMPORTING value     = corrnr ).
    IF cl_adt_cts_management=>is_transport_disabled_for_obj(
         object   = object_type
         obj_name = CONV #( name ) ) = abap_true.
      RETURN.
    ENDIF.

    " As SE24: the class definition, which holds the component metadata;
    " an interface goes whole.
    IF object_type = 'INTF'.
      limu = 'INTF'.
    ELSE.
      limu = 'CLSD'.
    ENDIF.
    cifkey-clsname = name.
    manager = cl_adt_corr_insert_dark=>get_instance( ).
    manager->corrnr = corrnr.
    CALL FUNCTION 'SEO_CLIF_CORR_INSERT'
      EXPORTING
        cifkey            = cifkey
        limu              = limu
        suppress_dialog   = seox_true
        lifecycle_manager = manager
      CHANGING
        corrnr            = corrnr
        devclass          = devclass
      EXCEPTIONS
        OTHERS            = 1.
    IF sy-subrc <> 0.
      RAISE EXCEPTION TYPE cx_adt_res_save_failure
        EXPORTING
          textid = cx_adt_rest=>create_textid_from_msg_params( ).
    ENDIF.
  ENDMETHOD.


  METHOD save.
    DATA text TYPE ty_text.
    DATA compotx TYPE seocompotx.
    DATA subcotx TYPE seosubcotx.

    LOOP AT texts INTO text.
      IF text-sconame IS INITIAL.
        IF text-descript IS INITIAL.
          DELETE FROM seocompotx WHERE clsname = name
                                   AND cmpname = text-cmpname
                                   AND langu   = language.
        ELSE.
          compotx-clsname = name.
          compotx-cmpname = text-cmpname.
          compotx-langu = language.
          compotx-descript = text-descript.
          MODIFY seocompotx FROM compotx.
        ENDIF.
      ELSE.
        IF text-descript IS INITIAL.
          DELETE FROM seosubcotx WHERE clsname = name
                                   AND cmpname = text-cmpname
                                   AND sconame = text-sconame
                                   AND langu   = language.
        ELSE.
          subcotx-clsname = name.
          subcotx-cmpname = text-cmpname.
          subcotx-sconame = text-sconame.
          subcotx-langu = language.
          subcotx-descript = text-descript.
          MODIFY seosubcotx FROM subcotx.
        ENDIF.
      ENDIF.
    ENDLOOP.
  ENDMETHOD.


  METHOD respond.
    DATA compotx TYPE STANDARD TABLE OF seocompotx.
    DATA subcotx TYPE STANDARD TABLE OF seosubcotx.
    DATA texts TYPE tt_texts.
    DATA text TYPE ty_text.
    DATA line_c TYPE seocompotx.
    DATA line_s TYPE seosubcotx.
    DATA body TYPE string.
    DATA tab TYPE c LENGTH 1.

    tab = cl_abap_char_utilities=>horizontal_tab.
    SELECT * FROM seocompotx INTO TABLE compotx
      WHERE clsname = name AND langu = language.
    SELECT * FROM seosubcotx INTO TABLE subcotx
      WHERE clsname = name AND langu = language.
    LOOP AT compotx INTO line_c WHERE descript IS NOT INITIAL.
      CLEAR text.
      text-cmpname = line_c-cmpname.
      text-descript = line_c-descript.
      APPEND text TO texts.
    ENDLOOP.
    LOOP AT subcotx INTO line_s WHERE descript IS NOT INITIAL.
      text-cmpname = line_s-cmpname.
      text-sconame = line_s-sconame.
      text-descript = line_s-descript.
      APPEND text TO texts.
    ENDLOOP.
    SORT texts BY cmpname sconame.

    LOOP AT texts INTO text.
      body = |{ body }{ text-cmpname }{ tab }{ text-sconame }{ tab }{ text-descript }\n|.
    ENDLOOP.
    zcl_vsp_adt_util=>respond( response     = response
                               content_type = co_content_type
                               body         = body ).
  ENDMETHOD.


  METHOD fail.
    zcl_vsp_adt_util=>raise_failure( resource_type = object_type
                                     resource_id   = name
                                     text          = text ).
  ENDMETHOD.

ENDCLASS.
