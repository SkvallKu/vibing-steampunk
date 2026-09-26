REPORT zvsp_adt_fallback_enho.
* vsp ADT fallback points: the BAdI implementations (ENHO).
* vsp runs this through RFC_ABAP_INSTALL_AND_RUN, which saves it as
* Z$$$XRFC, runs it and deletes it; it can be run by hand in SE38 too.
* vsp sets the mode and the package: INSTALL, REMOVE or CHECK for the
* enhancements, PACKAGE or DROPPACKAGE for the local package (ADT
* before 7.5x cannot create or delete a package).
* Every line of output is: STATUS ENHO [IMPL] [text].
* Lines stay within 72 characters, the width RFC_ABAP_INSTALL_AND_RUN
* takes a program in.

TYPES: BEGIN OF ty_flt,
         name TYPE badi_filter_name,
         cmp  TYPE badi_comparators,
         val  TYPE string,
       END OF ty_flt,
       tt_flt TYPE STANDARD TABLE OF ty_flt WITH DEFAULT KEY,
       BEGIN OF ty_imp,
         enh   TYPE enhname,
         spot  TYPE enhspotname,
         impl  TYPE enh_badi_impl,
         badi  TYPE enhbadiid,
         class TYPE seoclsname,
         text  TYPE string,
         flts  TYPE tt_flt,
       END OF ty_imp,
       tt_imp TYPE STANDARD TABLE OF ty_imp WITH DEFAULT KEY.

CONSTANTS gc_mode TYPE string VALUE '{{MODE}}'.
CONSTANTS gc_pkg TYPE devclass VALUE '{{PACKAGE}}'.

DATA gt_imp TYPE tt_imp.
DATA gs_imp TYPE ty_imp.
DATA gt_enh TYPE STANDARD TABLE OF enhname WITH DEFAULT KEY.
DATA gv_enh TYPE enhname.

START-OF-SELECTION.
  CASE gc_mode.
    WHEN 'PACKAGE'.
      PERFORM create_package.
      RETURN.
    WHEN 'DROPPACKAGE'.
      PERFORM drop_package.
      RETURN.
  ENDCASE.
  PERFORM define.
  LOOP AT gt_imp INTO gs_imp.
    COLLECT gs_imp-enh INTO gt_enh.
  ENDLOOP.
  LOOP AT gt_enh INTO gv_enh.
    CASE gc_mode.
      WHEN 'INSTALL'.
        PERFORM install USING gv_enh.
      WHEN 'REMOVE'.
        PERFORM remove USING gv_enh.
      WHEN OTHERS.
        PERFORM check USING gv_enh.
    ENDCASE.
  ENDLOOP.

*----------------------------------------------------------------------
* The implementations. The paths never change: a new feature is a new
* class behind the same /sap/bc/adt/zvsp prefix.
FORM define.
  CLEAR gs_imp.
  gs_imp-enh = 'ZVSP_ADT_FALLBACK_APP'.
  gs_imp-spot = 'SADT_REST_RFC_APPLICATION'.
  gs_imp-impl = 'ZVSP_ADT_FALLBACK'.
  gs_imp-badi = 'BADI_ADT_REST_RFC_APPLICATION'.
  gs_imp-class = 'ZCL_VSP_ADT_APP'.
  gs_imp-text = 'vsp ADT fallback points'.
  PERFORM flt USING 'STATIC_URI_PATH' '=' '/sap/bc/adt/zvsp'.
  PERFORM flt USING 'STATIC_URI_PATH' 'CP' '/sap/bc/adt/zvsp/*'.
  APPEND gs_imp TO gt_imp.

  CLEAR gs_imp.
  gs_imp-enh = 'ZVSP_ADT_FALLBACK_APP'.
  gs_imp-spot = 'SADT_REST_RFC_APPLICATION'.
  gs_imp-impl = 'ZVSP_ADT_FALLBACK_DISC'.
  gs_imp-badi = 'BADI_ADT_DISCOVERY_PROVIDER'.
  gs_imp-class = 'ZCL_VSP_ADT_APP'.
  gs_imp-text = 'vsp ADT fallback points: discovery'.
  PERFORM flt USING 'URI' '=' '/sap/bc/adt/discovery'.
  APPEND gs_imp TO gt_imp.

  CLEAR gs_imp.
  gs_imp-enh = 'ZVSP_ADT_FALLBACK_URI_MAP'.
  gs_imp-spot = 'SADT_URI_MAPPING'.
  gs_imp-impl = 'ZVSP_ADT_FALLBACK_MAP'.
  gs_imp-badi = 'BADI_ADT_MAP_FROM_OBJREF'.
  gs_imp-class = 'ZCL_VSP_ADT_URI_MAPPER'.
  gs_imp-text = 'vsp ADT fallback points: URI mapping'.
  PERFORM flt USING 'URI_PATTERN' 'CP' '/sap/bc/adt/zvsp/*'.
  APPEND gs_imp TO gt_imp.
ENDFORM.

FORM flt USING iv_name TYPE csequence iv_cmp TYPE csequence
               iv_val TYPE csequence.
  DATA ls_flt TYPE ty_flt.
  ls_flt-name = iv_name.
  ls_flt-cmp = iv_cmp.
  ls_flt-val = iv_val.
  APPEND ls_flt TO gs_imp-flts.
ENDFORM.

*----------------------------------------------------------------------
FORM out USING iv_status TYPE csequence iv_enh TYPE csequence
               iv_impl TYPE csequence iv_text TYPE csequence.
  DATA lv_line TYPE string.
  lv_line = |{ iv_status } { iv_enh } { iv_impl } { iv_text }|.
  CONDENSE lv_line.
  WRITE / lv_line.
ENDFORM.

FORM exists USING iv_enh TYPE enhname CHANGING cv_yes TYPE abap_bool.
  DATA lv_name TYPE enhname.
  cv_yes = abap_false.
  SELECT SINGLE enhname FROM enhheader INTO lv_name
    WHERE enhname = iv_enh.
  IF sy-subrc = 0.
    cv_yes = abap_true.
  ENDIF.
ENDFORM.

* The wanted implementation data: filter metadata (type, check type)
* is copied from another implementation of the same BAdI, so that it
* is what this release expects.
FORM build USING is_imp TYPE ty_imp
                 io_tool TYPE REF TO cl_enh_tool_badi_impl
           CHANGING cs_data TYPE enh_badi_impl_data.
  DATA ls_flt TYPE ty_flt.
  DATA ls_val TYPE enh_badiimpl_filter_value.
  DATA ls_tpl TYPE enh_badiimpl_filter_value.
  DATA lt_tpl TYPE enh_badiimpl_filter_value_it.
  DATA ls_root TYPE enh_badiimpl_filter_root.
  DATA ls_id TYPE enh_badiimpl_filter_id.
  DATA lt_tree TYPE enh_badiimpl_filter_tree_it.
  DATA lv_id TYPE i.

  PERFORM template USING is_imp CHANGING lt_tpl.

  CLEAR cs_data.
  cs_data-impl_name = is_imp-impl.
  cs_data-badi_name = is_imp-badi.
  cs_data-spot_name = is_imp-spot.
  cs_data-impl_class = is_imp-class.
  cs_data-impl_shorttext = is_imp-text.
  cs_data-active = abap_true.

  LOOP AT is_imp-flts INTO ls_flt.
    lv_id = sy-tabix.
    CLEAR: ls_val, ls_tpl.
    READ TABLE lt_tpl INTO ls_tpl WITH KEY filter_name = ls_flt-name.
    ls_val-filter_type = ls_tpl-filter_type.
    ls_val-check_type = ls_tpl-check_type.
    ls_val-check_type_is_dom = ls_tpl-check_type_is_dom.
    ls_val-check_class = ls_tpl-check_class.
    ls_val-check_length = ls_tpl-check_length.
    ls_val-check_decs = ls_tpl-check_decs.
    ls_val-id = lv_id.
    ls_val-filter_name = ls_flt-name.
    ls_val-compare = ls_flt-cmp.
    ls_val-filter_string_value1 = ls_flt-val.
    ls_val-filter_general_value1 = ls_flt-val.
    ls_val-filter_char_value1 = ls_flt-val.
    APPEND ls_val TO cs_data-filter_values.

*   One root per condition: the roots are ORed.
    CLEAR: ls_root, ls_id.
    ls_root-root = lv_id.
    ls_id-id = lv_id.
    ls_id-filter_name = ls_flt-name.
    APPEND ls_id TO ls_root-filters.
    APPEND ls_root TO cs_data-filter_root.
  ENDLOOP.

  cs_data-filters = io_tool->calculate_filter_condition(
    filter_root   = cs_data-filter_root
    filter_tree   = lt_tree
    filter_values = cs_data-filter_values ).
ENDFORM.

FORM template USING is_imp TYPE ty_imp
              CHANGING ct_tpl TYPE enh_badiimpl_filter_value_it.
  DATA lv_enh TYPE enhname.
  DATA lv_impl TYPE enh_badi_impl.
  DATA lo_tool TYPE REF TO cl_enh_tool_badi_impl.
  DATA ls_data TYPE enh_badi_impl_data.

  CLEAR ct_tpl.
  SELECT enhname badi_impl FROM badi_impl INTO (lv_enh, lv_impl)
    UP TO 1 ROWS
    WHERE badi_name = is_imp-badi
      AND enhname <> is_imp-enh.
  ENDSELECT.
  IF sy-subrc <> 0.
    RETURN.
  ENDIF.
  TRY.
      lo_tool ?= cl_enh_factory=>get_enhancement(
        enhancement_id = lv_enh
        run_dark       = abap_true ).
      ls_data = lo_tool->get_implementation( lv_impl ).
      ct_tpl = ls_data-filter_values.
    CATCH cx_root ##CATCH_ALL ##NO_HANDLER.
  ENDTRY.
ENDFORM.

* Same class, same conditions, active: nothing to do.
FORM same USING is_imp TYPE ty_imp is_old TYPE enh_badi_impl_data
          CHANGING cv_same TYPE abap_bool.
  DATA ls_flt TYPE ty_flt.
  DATA ls_val TYPE enh_badiimpl_filter_value.
  DATA lv_val TYPE string.

  cv_same = abap_false.
  IF is_old-impl_class <> is_imp-class
      OR is_old-active <> abap_true
      OR lines( is_old-filter_values ) <> lines( is_imp-flts ).
    RETURN.
  ENDIF.
  LOOP AT is_imp-flts INTO ls_flt.
    READ TABLE is_old-filter_values INTO ls_val
      WITH KEY filter_name = ls_flt-name compare = ls_flt-cmp.
    IF sy-subrc <> 0.
      RETURN.
    ENDIF.
    lv_val = ls_val-filter_string_value1.
    IF lv_val IS INITIAL.
      lv_val = ls_val-filter_char_value1.
    ENDIF.
    IF lv_val <> ls_flt-val.
      RETURN.
    ENDIF.
  ENDLOOP.
  cv_same = abap_true.
ENDFORM.

*----------------------------------------------------------------------
FORM install USING iv_enh TYPE enhname.
  DATA lv_yes TYPE abap_bool.
  DATA lv_same TYPE abap_bool.
  DATA lv_changed TYPE abap_bool.
  DATA lv_pkg TYPE devclass.
  DATA lv_tr TYPE trkorr.
  DATA lo_enh TYPE REF TO if_enh_tool.
  DATA lo_tool TYPE REF TO cl_enh_tool_badi_impl.
  DATA ls_imp TYPE ty_imp.
  DATA ls_old TYPE enh_badi_impl_data.
  DATA ls_new TYPE enh_badi_impl_data.
  DATA lx TYPE REF TO cx_root.
  DATA lv_text TYPE string.

  lv_pkg = gc_pkg.
  PERFORM exists USING iv_enh CHANGING lv_yes.
  TRY.
      IF lv_yes = abap_true.
        lo_enh = cl_enh_factory=>get_enhancement(
          enhancement_id = iv_enh
          lock           = abap_true
          run_dark       = abap_true ).
      ELSE.
        cl_enh_factory=>create_enhancement(
          EXPORTING
            enhname     = iv_enh
            enhtype     = cl_abstract_enh_tool_redef=>credefinition
            enhtooltype = cl_enh_tool_badi_impl=>tooltype
            dark        = abap_true
          IMPORTING
            enhancement = lo_enh
          CHANGING
            trkorr      = lv_tr
            devclass    = lv_pkg ).
        lv_changed = abap_true.
      ENDIF.
      lo_tool ?= lo_enh.

      LOOP AT gt_imp INTO ls_imp WHERE enh = iv_enh.
        IF lv_yes = abap_false.
          lo_tool->set_spot_name( ls_imp-spot ).
        ENDIF.
        CLEAR: ls_old, lv_same.
        TRY.
            ls_old = lo_tool->get_implementation( ls_imp-impl ).
            PERFORM same USING ls_imp ls_old CHANGING lv_same.
            IF lv_same = abap_false.
              lo_tool->delete_implementation( ls_imp-impl ).
            ENDIF.
          CATCH cx_enh_badi_impl_not_valid.
            CLEAR ls_old.
        ENDTRY.
        IF lv_same = abap_true.
          PERFORM out USING 'UNCHANGED' iv_enh ls_imp-impl ''.
          CONTINUE.
        ENDIF.
        PERFORM build USING ls_imp lo_tool CHANGING ls_new.
        lo_tool->add_implementation( ls_new ).
        lv_changed = abap_true.
        PERFORM out USING 'OK' iv_enh ls_imp-impl ls_imp-class.
      ENDLOOP.

      IF lv_changed = abap_true.
        lo_enh->if_enh_object~save(
          EXPORTING run_dark = abap_true
          CHANGING  devclass = lv_pkg
                    trkorr   = lv_tr ).
        lo_enh->if_enh_object~activate(
          EXPORTING run_dark = abap_true
          CHANGING  devclass = lv_pkg
                    trkorr   = lv_tr ).
      ENDIF.
*     Activation leaves the BAdI registry (BADI_IMPL, the filter
*     tables) to the workbench; generate writes it. Always, so that a
*     run also repairs an enhancement activated without it.
      lo_enh->if_enh_object~generate( ).
      lo_enh->if_enh_object~unlock( ).
      COMMIT WORK.
    CATCH cx_root INTO lx ##CATCH_ALL.
      lv_text = lx->get_text( ).
      PERFORM out USING 'ERROR' iv_enh '' lv_text.
      IF lo_enh IS BOUND.
        TRY.
            lo_enh->if_enh_object~unlock( ).
          CATCH cx_root ##CATCH_ALL ##NO_HANDLER.
        ENDTRY.
      ENDIF.
      ROLLBACK WORK.
  ENDTRY.
ENDFORM.

FORM remove USING iv_enh TYPE enhname.
  DATA lv_yes TYPE abap_bool.
  DATA lv_pkg TYPE devclass.
  DATA lv_tr TYPE trkorr.
  DATA lo_enh TYPE REF TO if_enh_tool.
  DATA lx TYPE REF TO cx_root.
  DATA lv_text TYPE string.

  PERFORM exists USING iv_enh CHANGING lv_yes.
  IF lv_yes = abap_false.
    PERFORM out USING 'ABSENT' iv_enh '' ''.
    RETURN.
  ENDIF.
  TRY.
      lo_enh = cl_enh_factory=>get_enhancement(
        enhancement_id = iv_enh
        lock           = abap_true
        run_dark       = abap_true ).
      lo_enh->if_enh_object~delete(
        EXPORTING nevertheless_delete = abap_true
                  run_dark            = abap_true
        CHANGING  devclass            = lv_pkg
                  trkorr              = lv_tr ).
      COMMIT WORK.
      PERFORM out USING 'DELETED' iv_enh '' ''.
    CATCH cx_root INTO lx ##CATCH_ALL.
      lv_text = lx->get_text( ).
      PERFORM out USING 'ERROR' iv_enh '' lv_text.
      ROLLBACK WORK.
  ENDTRY.
ENDFORM.

FORM check USING iv_enh TYPE enhname.
  DATA lv_yes TYPE abap_bool.
  DATA lv_same TYPE abap_bool.
  DATA lo_tool TYPE REF TO cl_enh_tool_badi_impl.
  DATA ls_imp TYPE ty_imp.
  DATA ls_old TYPE enh_badi_impl_data.
  DATA lx TYPE REF TO cx_root.
  DATA lv_text TYPE string.

  PERFORM exists USING iv_enh CHANGING lv_yes.
  IF lv_yes = abap_false.
    PERFORM out USING 'ABSENT' iv_enh '' ''.
    RETURN.
  ENDIF.
  TRY.
      lo_tool ?= cl_enh_factory=>get_enhancement(
        enhancement_id = iv_enh
        run_dark       = abap_true ).
      LOOP AT gt_imp INTO ls_imp WHERE enh = iv_enh.
        TRY.
            ls_old = lo_tool->get_implementation( ls_imp-impl ).
            PERFORM same USING ls_imp ls_old CHANGING lv_same.
            IF lv_same = abap_true.
              PERFORM out USING 'PRESENT' iv_enh ls_imp-impl ''.
            ELSE.
              PERFORM out USING 'DIFFERENT' iv_enh ls_imp-impl ''.
            ENDIF.
          CATCH cx_enh_badi_impl_not_valid.
            PERFORM out USING 'MISSING' iv_enh ls_imp-impl ''.
        ENDTRY.
      ENDLOOP.
    CATCH cx_root INTO lx ##CATCH_ALL.
      lv_text = lx->get_text( ).
      PERFORM out USING 'ERROR' iv_enh '' lv_text.
  ENDTRY.
ENDFORM.

*----------------------------------------------------------------------
FORM create_package.
  DATA ls_data TYPE scompkdtln.
  DATA lo_pkg TYPE REF TO if_package.
  DATA lv_text TYPE string.

  cl_package_factory=>load_package(
    EXPORTING  i_package_name = gc_pkg
    IMPORTING  e_package      = lo_pkg
    EXCEPTIONS OTHERS         = 1 ).
  IF sy-subrc = 0.
    PERFORM out USING 'EXISTS' gc_pkg '' ''.
    RETURN.
  ENDIF.

  ls_data-devclass = gc_pkg.
  ls_data-ctext = 'vsp ADT fallback points'.
  ls_data-as4user = sy-uname.
  ls_data-dlvunit = 'LOCAL'.
  cl_package_factory=>create_new_package(
    EXPORTING  i_suppress_dialog = abap_true
    IMPORTING  e_package         = lo_pkg
    CHANGING   c_package_data    = ls_data
    EXCEPTIONS OTHERS            = 1 ).
  IF sy-subrc <> 0.
    PERFORM msg_text CHANGING lv_text.
    PERFORM out USING 'ERROR' gc_pkg '' lv_text.
    RETURN.
  ENDIF.
  lo_pkg->save(
    EXPORTING  i_suppress_dialog = abap_true
    EXCEPTIONS OTHERS            = 1 ).
  IF sy-subrc <> 0.
    PERFORM msg_text CHANGING lv_text.
    PERFORM out USING 'ERROR' gc_pkg '' lv_text.
    ROLLBACK WORK.
    RETURN.
  ENDIF.
  lo_pkg->set_changeable(
    EXPORTING  i_changeable      = abap_false
               i_suppress_dialog = abap_true
    EXCEPTIONS OTHERS            = 1 ).
  COMMIT WORK.
  PERFORM out USING 'CREATED' gc_pkg '' ''.
ENDFORM.

FORM drop_package.
  DATA lo_pkg TYPE REF TO if_package.
  DATA lv_text TYPE string.

  cl_package_factory=>load_package(
    EXPORTING  i_package_name = gc_pkg
    IMPORTING  e_package      = lo_pkg
    EXCEPTIONS OTHERS         = 1 ).
  IF sy-subrc <> 0.
    PERFORM out USING 'ABSENT' gc_pkg '' ''.
    RETURN.
  ENDIF.
  lo_pkg->set_changeable(
    EXPORTING  i_changeable      = abap_true
               i_suppress_dialog = abap_true
    EXCEPTIONS OTHERS            = 1 ).
  lo_pkg->delete(
    EXPORTING  i_suppress_dialog = abap_true
    EXCEPTIONS OTHERS            = 1 ).
  IF sy-subrc = 0.
    lo_pkg->save(
      EXPORTING  i_suppress_dialog = abap_true
      EXCEPTIONS OTHERS            = 1 ).
  ENDIF.
  IF sy-subrc <> 0.
    PERFORM msg_text CHANGING lv_text.
    PERFORM out USING 'ERROR' gc_pkg '' lv_text.
    ROLLBACK WORK.
    RETURN.
  ENDIF.
  COMMIT WORK.
  PERFORM out USING 'DELETED' gc_pkg '' ''.
ENDFORM.

FORM msg_text CHANGING cv_text TYPE string.
  IF sy-msgid IS INITIAL.
    cv_text = |failed with sy-subrc { sy-subrc }|.
    RETURN.
  ENDIF.
  MESSAGE ID sy-msgid TYPE 'S' NUMBER sy-msgno
    WITH sy-msgv1 sy-msgv2 sy-msgv3 sy-msgv4 INTO cv_text.
ENDFORM.
