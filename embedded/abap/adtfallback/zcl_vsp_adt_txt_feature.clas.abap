"! <p class="shorttext synchronized">vsp ADT fallback: text elements</p>
"! Stands in for /sap/bc/adt/textelements (programs and classes) on
"! releases without it. Function groups are not served yet.
CLASS zcl_vsp_adt_txt_feature DEFINITION
  PUBLIC
  FINAL
  CREATE PUBLIC.

  PUBLIC SECTION.
    INTERFACES zif_vsp_adt_feature.

    CONSTANTS co_path TYPE string VALUE '/textelements'.

  PROTECTED SECTION.
  PRIVATE SECTION.
    CONSTANTS co_handler TYPE string VALUE 'ZCL_VSP_ADT_TXT_RESOURCE'.
ENDCLASS.



CLASS zcl_vsp_adt_txt_feature IMPLEMENTATION.

  METHOD zif_vsp_adt_feature~get_path.
    result = co_path.
  ENDMETHOD.


  METHOD zif_vsp_adt_feature~register_resources.
    DATA collections TYPE string_table.
    DATA collection TYPE string.
    DATA base TYPE string.

    APPEND 'programs' TO collections.
    APPEND 'classes' TO collections.

    LOOP AT collections INTO collection.
      base = |{ co_path }/{ collection }|.
      registry->register_discoverable_resource(
        url             = base
        handler_class   = co_handler
        description     = |Text elements ({ collection })|
        category_scheme = zcl_vsp_adt_app=>co_scheme
        category_term   = |textelements/{ collection }| ) ##NO_TEXT.
      registry->register_resource(
        template      = base && '/{name}'
        handler_class = co_handler ).
      registry->register_resource(
        template      = base && '/{name}/source/{sourcename}'
        handler_class = co_handler ).
    ENDLOOP.
  ENDMETHOD.


  METHOD zif_vsp_adt_feature~map_to_wb_object.
    DATA path TYPE string.
    DATA rest TYPE string.
    DATA collection TYPE string.
    DATA name TYPE string.
    DATA kind TYPE string.
    DATA object_type TYPE wbobjtype.
    DATA object_key TYPE seu_objkey.
    DATA class_name TYPE seoclsname.

    path = uri.
    SPLIT path AT '?' INTO path rest.
    SPLIT path AT '#' INTO path rest.

    FIND REGEX
      '^/sap/bc/adt/zvsp/textelements/(programs|classes)/([^/]+)(/source/(symbols|selections|headings))?$'
      IN path IGNORING CASE
      SUBMATCHES collection name rest kind.
    IF sy-subrc <> 0.
      RAISE EXCEPTION TYPE cx_adt_uri_mapping
        EXPORTING
          textid = cx_adt_uri_mapping=>invalid_uri
          uri    = path.
    ENDIF.

    TRANSLATE collection TO LOWER CASE.
    TRANSLATE kind TO LOWER CASE.
    name = cl_http_utility=>unescape_url( name ).
    TRANSLATE name TO UPPER CASE.

    " What is activated is REPT of the program that owns the text pool.
    " For a class that is its class pool, named as a program: before 7.5x
    " activating CLAS/OCX (class text elements) does nothing, while PROG/PX
    " of the class pool activates the same REPT.
    object_type-objtype_tr = 'PROG'.
    object_type-subtype_wb = swbm_c_type_prg_textelement.
    IF collection = 'classes'.
      class_name = name.
      name = cl_oo_classname_service=>get_classpool_name( class_name ).
    ENDIF.

    " The key is the name, and from offset 40 the part of the text pool.
    object_key = name.
    IF kind IS NOT INITIAL.
      object_key+40 = |source/{ kind }|.
    ENDIF.

    cl_wb_object=>create_from_global_type(
      EXPORTING
        p_object_type = object_type
        p_object_key  = object_key
      RECEIVING
        p_wb_object   = result
      EXCEPTIONS
        OTHERS        = 1 ).
    IF sy-subrc <> 0.
      RAISE EXCEPTION TYPE cx_adt_uri_mapping
        EXPORTING
          textid = cx_adt_uri_mapping=>invalid_uri
          uri    = path.
    ENDIF.
  ENDMETHOD.

ENDCLASS.
