"! <p class="shorttext synchronized">vsp ADT fallback: text pool lock</p>
"! Access to the text pool of one program: the SHOW check, the lock, and
"! the ADT lock handle proving on a later PUT that the lock is still ours.
"! RS_ACCESS_PERMISSION locks a text pool with ENQUEUE_EABAPTEXTE (table
"! S010T, the program name after a masked language) and, before 7.5x, does
"! not fill an ADT lock handle for it. Nor is there a SEU_LOCKING_ADT_BADI
"! implementation for TEXT, and adding one would collide with SAP's own
"! after an upgrade. So the handle is made here the way CL_ADT_LOCK_HANDLE
"! makes it: a hash over the enqueue entry, which changes when the lock is
"! lost and taken again.
CLASS zcl_vsp_adt_txt_lock DEFINITION
  PUBLIC
  FINAL
  CREATE PUBLIC.

  PUBLIC SECTION.
    METHODS constructor
      IMPORTING rept_name TYPE syrepid.

    METHODS check_show
      RAISING cx_adt_rest.

    METHODS lock
      RETURNING VALUE(result) TYPE sadt_object_lock_result
      RAISING   cx_adt_rest.

    METHODS unlock.

    "! Raises CX_ADT_RES_INVALID_LOCK_HANDLE unless the handle is the one
    "! the lock was taken with.
    METHODS check_handle
      IMPORTING lock_handle TYPE csequence
      RAISING   cx_adt_rest.

  PROTECTED SECTION.
  PRIVATE SECTION.
    DATA rept_name TYPE syrepid.

    METHODS get_handle
      RETURNING VALUE(result) TYPE string.

    METHODS raise_no_access
      IMPORTING text TYPE csequence OPTIONAL
      RAISING   cx_adt_rest.
ENDCLASS.



CLASS zcl_vsp_adt_txt_lock IMPLEMENTATION.

  METHOD constructor.
    me->rept_name = rept_name.
  ENDMETHOD.


  METHOD check_show.
    CALL FUNCTION 'RS_ACCESS_PERMISSION'
      EXPORTING
        authority_check          = abap_true
        global_lock              = space
        mode                     = 'SHOW'
        object                   = rept_name
        object_class             = 'TEXT'
        suppress_corr_check      = abap_true
        suppress_language_check  = abap_true
        suppress_extend_dialog   = abap_true
      EXCEPTIONS
        canceled_in_corr         = 1
        enqueued_by_user         = 2
        enqueue_system_failure   = 3
        illegal_parameter_values = 4
        locked_by_author         = 5
        no_modify_permission     = 6
        no_show_permission       = 7
        permission_failure       = 8
        OTHERS                   = 9.
    IF sy-subrc = 7.
      RAISE EXCEPTION TYPE cx_adt_res_no_authorization
        EXPORTING
          textid        = cx_adt_rest=>create_textid_from_msg_params( )
          resource_type = 'REPT'
          resource_id   = CONV #( rept_name ).
    ENDIF.
  ENDMETHOD.


  METHOD lock.
    DATA check_result TYPE cts_check_result.
    DATA messages TYPE cts_messages.
    DATA message TYPE cts_message.
    DATA transport TYPE sadt_object_lock_result.

    CALL FUNCTION 'RS_ACCESS_PERMISSION'
      EXPORTING
        authority_check                = abap_true
        global_lock                    = space
        mode                           = 'MODIFY'
        object                         = rept_name
        object_class                   = 'TEXT'
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
            resource_type = 'REPT'
            resource_id   = CONV #( rept_name ).
      WHEN OTHERS.
        raise_no_access( ).
    ENDCASE.

    result-lock_handle = get_handle( ).
    IF result-lock_handle IS INITIAL.
      unlock( ).
      raise_no_access( |No lock entry for the text pool of { rept_name } after locking it| ).
    ENDIF.

    " A system or software component closed for changes says so here.
    cl_adt_cts_management=>check_objects_in_request(
      EXPORTING
        pgmid        = 'LIMU'
        object       = 'REPT'
        obj_name     = CONV #( rept_name )
      IMPORTING
        check_result = check_result
        messages     = messages ).
    IF check_result-result <> 'S' AND messages IS NOT INITIAL.
      unlock( ).
      READ TABLE messages INTO message WITH KEY severity = 'E'.
      IF sy-subrc <> 0.
        READ TABLE messages INTO message INDEX 1.
      ENDIF.
      raise_no_access( message-text ).
    ENDIF.

    transport = cl_adt_cts_management=>get_transport_info(
      pgmid    = 'LIMU'
      obj_type = 'REPT'
      obj_name = CONV #( rept_name ) ).
    result-corrnr = transport-corrnr.
    result-corruser = transport-corruser.
    result-corrtext = transport-corrtext.
    result-is_link_up = transport-is_link_up.
    result-is_local = cl_adt_cts_management=>is_transport_disabled_for_obj(
      pgmid    = 'LIMU'
      object   = 'REPT'
      obj_name = CONV #( rept_name ) ).
  ENDMETHOD.


  METHOD unlock.
    CALL FUNCTION 'RS_ACCESS_PERMISSION'
      EXPORTING
        authority_check = space
        global_lock     = space
        mode            = 'FREE'
        object          = rept_name
        object_class    = 'TEXT'
      EXCEPTIONS
        OTHERS          = 1.
  ENDMETHOD.


  METHOD check_handle.
    DATA current TYPE string.

    current = get_handle( ).
    IF lock_handle IS INITIAL OR current <> lock_handle.
      RAISE EXCEPTION TYPE cx_adt_res_invalid_lock_handle
        EXPORTING
          resource_type = 'REPT'
          resource_id   = CONV #( rept_name )
          lock_handle   = lock_handle.
    ENDIF.
  ENDMETHOD.


  METHOD get_handle.
    DATA entries TYPE STANDARD TABLE OF seqg3.
    DATA entry TYPE seqg3.
    DATA text TYPE string.

    CALL FUNCTION 'ENQUEUE_READ'
      EXPORTING
        gname                 = 'S010T'
        guname                = sy-uname
      TABLES
        enq                   = entries
      EXCEPTIONS
        communication_failure = 1
        system_failure        = 2
        OTHERS                = 3.
    IF sy-subrc <> 0.
      RETURN.
    ENDIF.

    " The argument is S010T: a language (masked) and the program.
    LOOP AT entries INTO entry.
      IF entry-garg+1(40) = rept_name.
        text = |{ entry-gname }{ entry-garg }{ entry-gtdate }{ entry-gttime }| &&
               |{ entry-gtusec }{ entry-gtwp }{ entry-gtsysnr }{ entry-gthost }|.
        result = cl_adt_lock_hasher=>get( text ).
        RETURN.
      ENDIF.
    ENDLOOP.
  ENDMETHOD.


  METHOD raise_no_access.
    IF text IS NOT INITIAL.
      zcl_vsp_adt_txt_format=>set_message( text ).
    ENDIF.
    RAISE EXCEPTION TYPE cx_adt_res_no_access
      EXPORTING
        textid        = cx_adt_rest=>create_textid_from_msg_params( )
        resource_type = 'REPT'
        resource_id   = CONV #( rept_name ).
  ENDMETHOD.

ENDCLASS.
