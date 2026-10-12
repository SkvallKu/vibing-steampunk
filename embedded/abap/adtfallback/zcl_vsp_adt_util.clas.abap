"! <p class="shorttext synchronized">vsp ADT fallback: shared helpers</p>
"! What the resources of more than one feature need: an ADT lock handle
"! made from the enqueue entries of an object, a plain body in the
"! response, a lock result, and the ADT exception for a failed standard
"! call.
CLASS zcl_vsp_adt_util DEFINITION
  PUBLIC
  FINAL
  CREATE PUBLIC.

  PUBLIC SECTION.
    "! A hash over the caller's enqueue entries whose argument holds the
    "! name; empty when there is none. It changes when the lock is lost
    "! and taken again, which is what a lock handle must tell.
    CLASS-METHODS lock_handle
      IMPORTING name          TYPE csequence
      RETURNING VALUE(result) TYPE string.

    CLASS-METHODS respond
      IMPORTING response     TYPE REF TO if_adt_rest_response
                content_type TYPE string
                body         TYPE string.

    "! The answer to _action=LOCK: the lock result as asXML, the way the
    "! standard LOCK gives it. Written here and not through
    "! CL_ADT_REST_COMP_CNT_HANDLER, whose negotiation on 7.40 SP06 wants
    "! the Accept parameters to equal the handler's type, which has none.
    "! vsp asks with charset and dataname, so the lock was taken and the
    "! answer to it was a 406 — the lock stayed in the session.
    CLASS-METHODS respond_lock
      IMPORTING response TYPE REF TO if_adt_rest_response
                result   TYPE sadt_object_lock_result.

    "! Raises CX_ADT_RES_NO_ACCESS with the message in SY-MSG*, or with
    "! the text when there is none.
    CLASS-METHODS raise_failure
      IMPORTING resource_type TYPE csequence
                resource_id   TYPE csequence
                text          TYPE csequence OPTIONAL
      RAISING   cx_adt_rest.

    "! Refuses to create an object in a package where the user may not
    "! create it: S_DEVELOP with activity 01, as SE11 checks. The transport
    "! and dictionary calls a create makes after it check nothing of the
    "! kind, so without this a developer kept out of a package by DEVCLASS
    "! could create there through vsp.
    CLASS-METHODS check_create
      IMPORTING object_type TYPE trobjtype
                name        TYPE csequence
                devclass    TYPE devclass
      RAISING   cx_adt_rest.

    "! The body of a request as text lines.
    CLASS-METHODS request_lines
      IMPORTING request       TYPE REF TO if_adt_rest_request
      RETURNING VALUE(result) TYPE string_table.

  PROTECTED SECTION.
  PRIVATE SECTION.
ENDCLASS.



CLASS zcl_vsp_adt_util IMPLEMENTATION.

  METHOD lock_handle.
    DATA entries TYPE STANDARD TABLE OF seqg3.
    DATA entry TYPE seqg3.
    DATA text TYPE string.

    CALL FUNCTION 'ENQUEUE_READ'
      EXPORTING
        gname  = space
        guname = sy-uname
      TABLES
        enq    = entries
      EXCEPTIONS
        OTHERS = 1.
    IF sy-subrc <> 0.
      RETURN.
    ENDIF.
    SORT entries BY gname garg.
    LOOP AT entries INTO entry WHERE garg CS name.
      text = |{ text }{ entry-gname }{ entry-garg }{ entry-gtdate }{ entry-gttime }| &&
             |{ entry-gtusec }{ entry-gtwp }{ entry-gtsysnr }{ entry-gthost }|.
    ENDLOOP.
    IF text IS NOT INITIAL.
      result = cl_adt_lock_hasher=>get( text ).
    ENDIF.
  ENDMETHOD.


  METHOD respond.
    DATA entity TYPE REF TO if_rest_entity.

    entity = response->get_inner_rest_response( )->create_entity( ).
    entity->set_content_type( iv_media_type = content_type ).
    entity->set_string_data( body ).
  ENDMETHOD.


  METHOD respond_lock.
    DATA xml TYPE xstring.
    DATA entity TYPE REF TO if_rest_entity.

    CALL TRANSFORMATION id SOURCE data = result RESULT XML xml.
    entity = response->get_inner_rest_response( )->create_entity( ).
    entity->set_content_type(
      iv_media_type = 'application/vnd.sap.as+xml;charset=UTF-8;dataname=com.sap.adt.lock.result' ).
    entity->set_binary_data( xml ).
  ENDMETHOD.


  METHOD check_create.
    DATA objname TYPE sobj_name.

    objname = name.
    AUTHORITY-CHECK OBJECT 'S_DEVELOP'
      ID 'DEVCLASS' FIELD devclass
      ID 'OBJTYPE'  FIELD object_type
      ID 'OBJNAME'  FIELD objname
      ID 'P_GROUP'  DUMMY
      ID 'ACTVT'    FIELD '01'.
    IF sy-subrc <> 0.
      zcl_vsp_adt_txt_format=>set_message(
        |No authorization to create { object_type } { objname } in package { devclass } (S_DEVELOP, activity 01)| ).
      RAISE EXCEPTION TYPE cx_adt_res_no_authorization
        EXPORTING
          textid        = cx_adt_rest=>create_textid_from_msg_params( )
          resource_type = CONV #( object_type )
          resource_id   = CONV #( objname ).
    ENDIF.
  ENDMETHOD.


  METHOD raise_failure.
    DATA message TYPE string.

    IF text IS NOT INITIAL.
      message = text.
    ELSEIF sy-msgid IS NOT INITIAL.
      MESSAGE ID sy-msgid TYPE 'S' NUMBER sy-msgno
        WITH sy-msgv1 sy-msgv2 sy-msgv3 sy-msgv4 INTO message.
    ELSE.
      message = |{ resource_type } { resource_id }: failed with sy-subrc { sy-subrc }|.
    ENDIF.
    zcl_vsp_adt_txt_format=>set_message( message ).
    RAISE EXCEPTION TYPE cx_adt_res_no_access
      EXPORTING
        textid        = cx_adt_rest=>create_textid_from_msg_params( )
        resource_type = CONV #( resource_type )
        resource_id   = CONV #( resource_id ).
  ENDMETHOD.


  METHOD request_lines.
    DATA text TYPE string.

    text = request->get_inner_rest_request( )->get_entity( )->get_string_data( ).
    REPLACE ALL OCCURRENCES OF cl_abap_char_utilities=>cr_lf IN text
      WITH cl_abap_char_utilities=>newline.
    REPLACE ALL OCCURRENCES OF cl_abap_char_utilities=>cr_lf(1) IN text
      WITH cl_abap_char_utilities=>newline.
    SPLIT text AT cl_abap_char_utilities=>newline INTO TABLE result.
  ENDMETHOD.

ENDCLASS.
