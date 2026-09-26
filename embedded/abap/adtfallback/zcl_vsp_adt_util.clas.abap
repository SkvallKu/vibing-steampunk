"! <p class="shorttext synchronized">vsp ADT fallback: shared helpers</p>
"! What the resources of more than one feature need: an ADT lock handle
"! made from the enqueue entries of an object, a plain body in the
"! response, and the ADT exception for a failed standard call.
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

    "! Raises CX_ADT_RES_NO_ACCESS with the message in SY-MSG*, or with
    "! the text when there is none.
    CLASS-METHODS raise_failure
      IMPORTING resource_type TYPE csequence
                resource_id   TYPE csequence
                text          TYPE csequence OPTIONAL
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
