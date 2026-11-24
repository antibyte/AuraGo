var __defProp = Object.defineProperty;
var __defNormalProp = (obj, key, value) => key in obj ? __defProp(obj, key, { enumerable: true, configurable: true, writable: true, value }) : obj[key] = value;
var __publicField = (obj, key, value) => __defNormalProp(obj, typeof key !== "symbol" ? key + "" : key, value);
const NA = new URL("./ironrdp_web_bg.wasm", import.meta.url);
class s {
  static __wrap(A) {
    const g = Object.create(s.prototype);
    return g.__wbg_ptr = A, n.register(g, g.__wbg_ptr, g), g;
  }
  __destroy_into_raw() {
    const A = this.__wbg_ptr;
    return this.__wbg_ptr = 0, n.unregister(this), A;
  }
  free() {
    const A = this.__destroy_into_raw();
    C.__wbg_clipboarddata_free(A, 0);
  }
  /**
   * @param {string} mime_type
   * @param {Uint8Array} binary
   */
  addBinary(A, g) {
    const I = Y(A, C.__wbindgen_malloc, C.__wbindgen_realloc), B = N, E = SA(g, C.__wbindgen_malloc), D = N;
    C.clipboarddata_addBinary(this.__wbg_ptr, I, B, E, D);
  }
  /**
   * @param {string} mime_type
   * @param {string} text
   */
  addText(A, g) {
    const I = Y(A, C.__wbindgen_malloc, C.__wbindgen_realloc), B = N, E = Y(g, C.__wbindgen_malloc, C.__wbindgen_realloc), D = N;
    C.clipboarddata_addText(this.__wbg_ptr, I, B, E, D);
  }
  constructor() {
    const A = C.clipboarddata_create();
    return this.__wbg_ptr = A, n.register(this, this.__wbg_ptr, this), this;
  }
  /**
   * @returns {boolean}
   */
  isEmpty() {
    return C.clipboarddata_isEmpty(this.__wbg_ptr) !== 0;
  }
  /**
   * @returns {ClipboardItem[]}
   */
  items() {
    const A = C.clipboarddata_items(this.__wbg_ptr);
    var g = hA(A[0], A[1]);
    return C.__wbindgen_free(A[0], A[1] * 4, 4), g;
  }
}
Symbol.dispose && (s.prototype[Symbol.dispose] = s.prototype.free);
class V {
  static __wrap(A) {
    const g = Object.create(V.prototype);
    return g.__wbg_ptr = A, $.register(g, g.__wbg_ptr, g), g;
  }
  __destroy_into_raw() {
    const A = this.__wbg_ptr;
    return this.__wbg_ptr = 0, $.unregister(this), A;
  }
  free() {
    const A = this.__destroy_into_raw();
    C.__wbg_clipboarditem_free(A, 0);
  }
  /**
   * @returns {string}
   */
  mimeType() {
    let A, g;
    try {
      const I = C.clipboarditem_mimeType(this.__wbg_ptr);
      return A = I[0], g = I[1], k(I[0], I[1]);
    } finally {
      C.__wbindgen_free(A, g, 1);
    }
  }
  /**
   * @returns {any}
   */
  value() {
    return C.clipboarditem_value(this.__wbg_ptr);
  }
}
Symbol.dispose && (V.prototype[Symbol.dispose] = V.prototype.free);
class S {
  static __wrap(A) {
    const g = Object.create(S.prototype);
    return g.__wbg_ptr = A, f.register(g, g.__wbg_ptr, g), g;
  }
  __destroy_into_raw() {
    const A = this.__wbg_ptr;
    return this.__wbg_ptr = 0, f.unregister(this), A;
  }
  free() {
    const A = this.__destroy_into_raw();
    C.__wbg_desktopsize_free(A, 0);
  }
  /**
   * @param {number} width
   * @param {number} height
   */
  constructor(A, g) {
    const I = C.desktopsize_create(A, g);
    return this.__wbg_ptr = I, f.register(this, this.__wbg_ptr, this), this;
  }
  /**
   * @returns {number}
   */
  get height() {
    return C.__wbg_get_desktopsize_height(this.__wbg_ptr);
  }
  /**
   * @returns {number}
   */
  get width() {
    return C.__wbg_get_desktopsize_width(this.__wbg_ptr);
  }
  /**
   * @param {number} arg0
   */
  set height(A) {
    C.__wbg_set_desktopsize_height(this.__wbg_ptr, A);
  }
  /**
   * @param {number} arg0
   */
  set width(A) {
    C.__wbg_set_desktopsize_width(this.__wbg_ptr, A);
  }
}
Symbol.dispose && (S.prototype[Symbol.dispose] = S.prototype.free);
class J {
  static __wrap(A) {
    const g = Object.create(J.prototype);
    return g.__wbg_ptr = A, AA.register(g, g.__wbg_ptr, g), g;
  }
  __destroy_into_raw() {
    const A = this.__wbg_ptr;
    return this.__wbg_ptr = 0, AA.unregister(this), A;
  }
  free() {
    const A = this.__destroy_into_raw();
    C.__wbg_deviceevent_free(A, 0);
  }
  /**
   * @param {number} scancode
   * @returns {DeviceEvent}
   */
  static keyPressed(A) {
    const g = C.deviceevent_keyPressed(A);
    return J.__wrap(g);
  }
  /**
   * @param {number} scancode
   * @returns {DeviceEvent}
   */
  static keyReleased(A) {
    const g = C.deviceevent_keyReleased(A);
    return J.__wrap(g);
  }
  /**
   * @param {number} button
   * @returns {DeviceEvent}
   */
  static mouseButtonPressed(A) {
    const g = C.deviceevent_mouseButtonPressed(A);
    return J.__wrap(g);
  }
  /**
   * @param {number} button
   * @returns {DeviceEvent}
   */
  static mouseButtonReleased(A) {
    const g = C.deviceevent_mouseButtonReleased(A);
    return J.__wrap(g);
  }
  /**
   * @param {number} x
   * @param {number} y
   * @returns {DeviceEvent}
   */
  static mouseMove(A, g) {
    const I = C.deviceevent_mouseMove(A, g);
    return J.__wrap(I);
  }
  /**
   * @param {string} unicode
   * @returns {DeviceEvent}
   */
  static unicodePressed(A) {
    const g = A.codePointAt(0);
    GA(g);
    const I = C.deviceevent_unicodePressed(g);
    return J.__wrap(I);
  }
  /**
   * @param {string} unicode
   * @returns {DeviceEvent}
   */
  static unicodeReleased(A) {
    const g = A.codePointAt(0);
    GA(g);
    const I = C.deviceevent_unicodeReleased(g);
    return J.__wrap(I);
  }
  /**
   * @param {boolean} vertical
   * @param {number} rotation_amount
   * @param {RotationUnit} rotation_unit
   * @returns {DeviceEvent}
   */
  static wheelRotations(A, g, I) {
    const B = C.deviceevent_wheelRotations(A, g, I);
    return J.__wrap(B);
  }
}
Symbol.dispose && (J.prototype[Symbol.dispose] = J.prototype.free);
class w {
  __destroy_into_raw() {
    const A = this.__wbg_ptr;
    return this.__wbg_ptr = 0, gA.unregister(this), A;
  }
  free() {
    const A = this.__destroy_into_raw();
    C.__wbg_extension_free(A, 0);
  }
  /**
   * @param {string} ident
   * @param {any} value
   */
  constructor(A, g) {
    const I = Y(A, C.__wbindgen_malloc, C.__wbindgen_realloc), B = N, E = C.extension_create(I, B, g);
    return this.__wbg_ptr = E, gA.register(this, this.__wbg_ptr, this), this;
  }
}
Symbol.dispose && (w.prototype[Symbol.dispose] = w.prototype.free);
class z {
  __destroy_into_raw() {
    const A = this.__wbg_ptr;
    return this.__wbg_ptr = 0, IA.unregister(this), A;
  }
  free() {
    const A = this.__destroy_into_raw();
    C.__wbg_inputtransaction_free(A, 0);
  }
  /**
   * @param {DeviceEvent} event
   */
  addEvent(A) {
    Z(A, J);
    var g = A.__destroy_into_raw();
    C.inputtransaction_addEvent(this.__wbg_ptr, g);
  }
  constructor() {
    const A = C.inputtransaction_create();
    return this.__wbg_ptr = A, IA.register(this, this.__wbg_ptr, this), this;
  }
}
Symbol.dispose && (z.prototype[Symbol.dispose] = z.prototype.free);
class T {
  static __wrap(A) {
    const g = Object.create(T.prototype);
    return g.__wbg_ptr = A, BA.register(g, g.__wbg_ptr, g), g;
  }
  __destroy_into_raw() {
    const A = this.__wbg_ptr;
    return this.__wbg_ptr = 0, BA.unregister(this), A;
  }
  free() {
    const A = this.__destroy_into_raw();
    C.__wbg_ironerror_free(A, 0);
  }
  /**
   * @returns {string}
   */
  backtrace() {
    let A, g;
    try {
      const I = C.ironerror_backtrace(this.__wbg_ptr);
      return A = I[0], g = I[1], k(I[0], I[1]);
    } finally {
      C.__wbindgen_free(A, g, 1);
    }
  }
  /**
   * @returns {IronErrorKind}
   */
  kind() {
    return C.ironerror_kind(this.__wbg_ptr);
  }
  /**
   * @returns {RDCleanPathDetails | undefined}
   */
  rdcleanpathDetails() {
    const A = C.ironerror_rdcleanpathDetails(this.__wbg_ptr);
    return A === 0 ? void 0 : t.__wrap(A);
  }
}
Symbol.dispose && (T.prototype[Symbol.dispose] = T.prototype.free);
class t {
  static __wrap(A) {
    const g = Object.create(t.prototype);
    return g.__wbg_ptr = A, QA.register(g, g.__wbg_ptr, g), g;
  }
  __destroy_into_raw() {
    const A = this.__wbg_ptr;
    return this.__wbg_ptr = 0, QA.unregister(this), A;
  }
  free() {
    const A = this.__destroy_into_raw();
    C.__wbg_rdcleanpathdetails_free(A, 0);
  }
  /**
   * RDCleanPath error code sent by the proxy.
   *
   * Common values:
   * - 1: General error
   * - 100: Credential expired
   * - 103: Target unreachable
   *
   * Codes 16384 to 32767 are implementation-specific: their meaning depends on the proxy.
   * @returns {number | undefined}
   */
  get errorCode() {
    const A = C.rdcleanpathdetails_errorCode(this.__wbg_ptr);
    return A === 16777215 ? void 0 : A;
  }
  /**
   * HTTP status code if the error originated from an HTTP response.
   *
   * Common values:
   * - 403: Forbidden (e.g., deleted VNET, insufficient permissions)
   * - 404: Not Found
   * - 500: Internal Server Error
   * - 502: Bad Gateway
   * - 503: Service Unavailable
   * @returns {number | undefined}
   */
  get httpStatusCode() {
    const A = C.rdcleanpathdetails_httpStatusCode(this.__wbg_ptr);
    return A === 16777215 ? void 0 : A;
  }
  /**
   * TLS alert code if the error occurred during TLS handshake.
   *
   * Common values:
   * - 40: Handshake failure
   * - 42: Bad certificate
   * - 45: Certificate expired
   * - 48: Unknown CA
   * - 112: Unrecognized name
   * @returns {number | undefined}
   */
  get tlsAlertCode() {
    const A = C.rdcleanpathdetails_tlsAlertCode(this.__wbg_ptr);
    return A === 16777215 ? void 0 : A;
  }
  /**
   * Windows Socket API (WSA) error code.
   *
   * Common values:
   * - 10013: Permission denied (WSAEACCES) - often indicates deleted/invalid VNET
   * - 10060: Connection timed out (WSAETIMEDOUT)
   * - 10061: Connection refused (WSAECONNREFUSED)
   * - 10051: Network is unreachable (WSAENETUNREACH)
   * - 10065: No route to host (WSAEHOSTUNREACH)
   * @returns {number | undefined}
   */
  get wsaErrorCode() {
    const A = C.rdcleanpathdetails_wsaErrorCode(this.__wbg_ptr);
    return A === 16777215 ? void 0 : A;
  }
}
Symbol.dispose && (t.prototype[Symbol.dispose] = t.prototype.free);
class _ {
  __destroy_into_raw() {
    const A = this.__wbg_ptr;
    return this.__wbg_ptr = 0, CA.unregister(this), A;
  }
  free() {
    const A = this.__destroy_into_raw();
    C.__wbg_rdpfile_free(A, 0);
  }
  constructor() {
    const A = C.rdpfile_create();
    return this.__wbg_ptr = A, CA.register(this, this.__wbg_ptr, this), this;
  }
  /**
   * @param {string} key
   * @returns {number | undefined}
   */
  getInt(A) {
    const g = Y(A, C.__wbindgen_malloc, C.__wbindgen_realloc), I = N, B = C.rdpfile_getInt(this.__wbg_ptr, g, I);
    return B === Number.MAX_SAFE_INTEGER ? void 0 : B;
  }
  /**
   * @param {string} key
   * @returns {string | undefined}
   */
  getStr(A) {
    const g = Y(A, C.__wbindgen_malloc, C.__wbindgen_realloc), I = N, B = C.rdpfile_getStr(this.__wbg_ptr, g, I);
    let E;
    return B[0] !== 0 && (E = k(B[0], B[1]), C.__wbindgen_free(B[0], B[1] * 1, 1)), E;
  }
  /**
   * @param {string} key
   * @param {number} value
   */
  insertInt(A, g) {
    const I = Y(A, C.__wbindgen_malloc, C.__wbindgen_realloc), B = N;
    C.rdpfile_insertInt(this.__wbg_ptr, I, B, g);
  }
  /**
   * @param {string} key
   * @param {string} value
   */
  insertStr(A, g) {
    const I = Y(A, C.__wbindgen_malloc, C.__wbindgen_realloc), B = N, E = Y(g, C.__wbindgen_malloc, C.__wbindgen_realloc), D = N;
    C.rdpfile_insertStr(this.__wbg_ptr, I, B, E, D);
  }
  /**
   * @param {string} config
   */
  parse(A) {
    const g = Y(A, C.__wbindgen_malloc, C.__wbindgen_realloc), I = N;
    C.rdpfile_parse(this.__wbg_ptr, g, I);
  }
  /**
   * @returns {string}
   */
  write() {
    let A, g;
    try {
      const I = C.rdpfile_write(this.__wbg_ptr);
      return A = I[0], g = I[1], k(I[0], I[1]);
    } finally {
      C.__wbindgen_free(A, g, 1);
    }
  }
}
Symbol.dispose && (_.prototype[Symbol.dispose] = _.prototype.free);
class P {
  static __wrap(A) {
    const g = Object.create(P.prototype);
    return g.__wbg_ptr = A, EA.register(g, g.__wbg_ptr, g), g;
  }
  __destroy_into_raw() {
    const A = this.__wbg_ptr;
    return this.__wbg_ptr = 0, EA.unregister(this), A;
  }
  free() {
    const A = this.__destroy_into_raw();
    C.__wbg_session_free(A, 0);
  }
  /**
   * @param {InputTransaction} transaction
   */
  applyInputs(A) {
    Z(A, z);
    var g = A.__destroy_into_raw();
    const I = C.session_applyInputs(this.__wbg_ptr, g);
    if (I[1])
      throw K(I[0]);
  }
  /**
   * @returns {DesktopSize}
   */
  desktopSize() {
    const A = C.session_desktopSize(this.__wbg_ptr);
    return S.__wrap(A);
  }
  /**
   * @param {Extension} ext
   * @returns {any}
   */
  invokeExtension(A) {
    Z(A, w);
    var g = A.__destroy_into_raw();
    const I = C.session_invokeExtension(this.__wbg_ptr, g);
    if (I[2])
      throw K(I[1]);
    return K(I[0]);
  }
  /**
   * @param {ClipboardData} content
   * @returns {Promise<void>}
   */
  onClipboardPaste(A) {
    return Z(A, s), C.session_onClipboardPaste(this.__wbg_ptr, A.__wbg_ptr);
  }
  releaseAllInputs() {
    const A = C.session_releaseAllInputs(this.__wbg_ptr);
    if (A[1])
      throw K(A[0]);
  }
  /**
   * @param {number} width
   * @param {number} height
   * @param {number | null} [scale_factor]
   * @param {number | null} [physical_width]
   * @param {number | null} [physical_height]
   */
  resize(A, g, I, B, E) {
    C.session_resize(this.__wbg_ptr, A, g, a(I) ? Number.MAX_SAFE_INTEGER : I >>> 0, a(B) ? Number.MAX_SAFE_INTEGER : B >>> 0, a(E) ? Number.MAX_SAFE_INTEGER : E >>> 0);
  }
  /**
   * @returns {Promise<SessionTerminationInfo>}
   */
  run() {
    return C.session_run(this.__wbg_ptr);
  }
  shutdown() {
    const A = C.session_shutdown(this.__wbg_ptr);
    if (A[1])
      throw K(A[0]);
  }
  /**
   * @returns {boolean}
   */
  supportsUnicodeKeyboardShortcuts() {
    return C.session_supportsUnicodeKeyboardShortcuts(this.__wbg_ptr) !== 0;
  }
  /**
   * @param {boolean} scroll_lock
   * @param {boolean} num_lock
   * @param {boolean} caps_lock
   * @param {boolean} kana_lock
   */
  synchronizeLockKeys(A, g, I, B) {
    const E = C.session_synchronizeLockKeys(this.__wbg_ptr, A, g, I, B);
    if (E[1])
      throw K(E[0]);
  }
}
Symbol.dispose && (P.prototype[Symbol.dispose] = P.prototype.free);
class M {
  static __wrap(A) {
    const g = Object.create(M.prototype);
    return g.__wbg_ptr = A, v.register(g, g.__wbg_ptr, g), g;
  }
  __destroy_into_raw() {
    const A = this.__wbg_ptr;
    return this.__wbg_ptr = 0, v.unregister(this), A;
  }
  free() {
    const A = this.__destroy_into_raw();
    C.__wbg_sessionbuilder_free(A, 0);
  }
  /**
   * @param {string} token
   * @returns {SessionBuilder}
   */
  authToken(A) {
    const g = Y(A, C.__wbindgen_malloc, C.__wbindgen_realloc), I = N, B = C.sessionbuilder_authToken(this.__wbg_ptr, g, I);
    return M.__wrap(B);
  }
  /**
   * @param {Function} callback
   * @returns {SessionBuilder}
   */
  canvasResizedCallback(A) {
    const g = C.sessionbuilder_canvasResizedCallback(this.__wbg_ptr, A);
    return M.__wrap(g);
  }
  /**
   * @returns {Promise<Session>}
   */
  connect() {
    return C.sessionbuilder_connect(this.__wbg_ptr);
  }
  constructor() {
    const A = C.sessionbuilder_create();
    return this.__wbg_ptr = A, v.register(this, this.__wbg_ptr, this), this;
  }
  /**
   * @param {DesktopSize} desktop_size
   * @returns {SessionBuilder}
   */
  desktopSize(A) {
    Z(A, S);
    var g = A.__destroy_into_raw();
    const I = C.sessionbuilder_desktopSize(this.__wbg_ptr, g);
    return M.__wrap(I);
  }
  /**
   * @param {string} destination
   * @returns {SessionBuilder}
   */
  destination(A) {
    const g = Y(A, C.__wbindgen_malloc, C.__wbindgen_realloc), I = N, B = C.sessionbuilder_destination(this.__wbg_ptr, g, I);
    return M.__wrap(B);
  }
  /**
   * @param {Extension} ext
   * @returns {SessionBuilder}
   */
  extension(A) {
    Z(A, w);
    var g = A.__destroy_into_raw();
    const I = C.sessionbuilder_extension(this.__wbg_ptr, g);
    return M.__wrap(I);
  }
  /**
   * @param {Function} callback
   * @returns {SessionBuilder}
   */
  forceClipboardUpdateCallback(A) {
    const g = C.sessionbuilder_forceClipboardUpdateCallback(this.__wbg_ptr, A);
    return M.__wrap(g);
  }
  /**
   * @param {string} password
   * @returns {SessionBuilder}
   */
  password(A) {
    const g = Y(A, C.__wbindgen_malloc, C.__wbindgen_realloc), I = N, B = C.sessionbuilder_password(this.__wbg_ptr, g, I);
    return M.__wrap(B);
  }
  /**
   * @param {string} address
   * @returns {SessionBuilder}
   */
  proxyAddress(A) {
    const g = Y(A, C.__wbindgen_malloc, C.__wbindgen_realloc), I = N, B = C.sessionbuilder_proxyAddress(this.__wbg_ptr, g, I);
    return M.__wrap(B);
  }
  /**
   * @param {Function} callback
   * @returns {SessionBuilder}
   */
  remoteClipboardChangedCallback(A) {
    const g = C.sessionbuilder_remoteClipboardChangedCallback(this.__wbg_ptr, A);
    return M.__wrap(g);
  }
  /**
   * @param {HTMLCanvasElement} canvas
   * @returns {SessionBuilder}
   */
  renderCanvas(A) {
    const g = C.sessionbuilder_renderCanvas(this.__wbg_ptr, A);
    return M.__wrap(g);
  }
  /**
   * @param {string} server_domain
   * @returns {SessionBuilder}
   */
  serverDomain(A) {
    const g = Y(A, C.__wbindgen_malloc, C.__wbindgen_realloc), I = N, B = C.sessionbuilder_serverDomain(this.__wbg_ptr, g, I);
    return M.__wrap(B);
  }
  /**
   * @param {any} context
   * @returns {SessionBuilder}
   */
  setCursorStyleCallbackContext(A) {
    const g = C.sessionbuilder_setCursorStyleCallbackContext(this.__wbg_ptr, A);
    return M.__wrap(g);
  }
  /**
   * @param {Function} callback
   * @returns {SessionBuilder}
   */
  setCursorStyleCallback(A) {
    const g = C.sessionbuilder_setCursorStyleCallback(this.__wbg_ptr, A);
    return M.__wrap(g);
  }
  /**
   * @param {string} username
   * @returns {SessionBuilder}
   */
  username(A) {
    const g = Y(A, C.__wbindgen_malloc, C.__wbindgen_realloc), I = N, B = C.sessionbuilder_username(this.__wbg_ptr, g, I);
    return M.__wrap(B);
  }
  /**
   * @param {Array<any>} protocols
   * @returns {SessionBuilder}
   */
  webSocketProtocols(A) {
    const g = C.sessionbuilder_webSocketProtocols(this.__wbg_ptr, A);
    return M.__wrap(g);
  }
}
Symbol.dispose && (M.prototype[Symbol.dispose] = M.prototype.free);
class O {
  static __wrap(A) {
    const g = Object.create(O.prototype);
    return g.__wbg_ptr = A, DA.register(g, g.__wbg_ptr, g), g;
  }
  __destroy_into_raw() {
    const A = this.__wbg_ptr;
    return this.__wbg_ptr = 0, DA.unregister(this), A;
  }
  free() {
    const A = this.__destroy_into_raw();
    C.__wbg_sessionterminationinfo_free(A, 0);
  }
  /**
   * @returns {string}
   */
  reason() {
    let A, g;
    try {
      const I = C.sessionterminationinfo_reason(this.__wbg_ptr);
      return A = I[0], g = I[1], k(I[0], I[1]);
    } finally {
      C.__wbindgen_free(A, g, 1);
    }
  }
}
Symbol.dispose && (O.prototype[Symbol.dispose] = O.prototype.free);
function YA(Q) {
  const A = Y(Q, C.__wbindgen_malloc, C.__wbindgen_realloc), g = N;
  C.setup(A, g);
}
function kA() {
  return {
    __proto__: null,
    "./ironrdp_web_bg.js": {
      __proto__: null,
      __wbg___wbindgen_boolean_get_5b446f51afd21013: function(A) {
        const g = A, I = typeof g == "boolean" ? g : void 0;
        return a(I) ? 16777215 : I ? 1 : 0;
      },
      __wbg___wbindgen_debug_string_4687d8d8c2017d52: function(A, g) {
        const I = e(g), B = Y(I, C.__wbindgen_malloc, C.__wbindgen_realloc), E = N;
        y().setInt32(A + 4, E, true), y().setInt32(A + 0, B, true);
      },
      __wbg___wbindgen_is_function_1f9d30630b8b1d3d: function(A) {
        return typeof A == "function";
      },
      __wbg___wbindgen_is_null_e343b7d08827ba72: function(A) {
        return A === null;
      },
      __wbg___wbindgen_is_object_3c45d4f2dde4e749: function(A) {
        const g = A;
        return typeof g == "object" && g !== null;
      },
      __wbg___wbindgen_is_string_90b56bc79aad6f6c: function(A) {
        return typeof A == "string";
      },
      __wbg___wbindgen_is_undefined_8865fb403f8fe9d8: function(A) {
        return A === void 0;
      },
      __wbg___wbindgen_number_get_2e0e7dee9f701a71: function(A, g) {
        const I = g, B = typeof I == "number" ? I : void 0;
        y().setFloat64(A + 8, a(B) ? 0 : B, true), y().setInt32(A + 0, !a(B), true);
      },
      __wbg___wbindgen_string_get_0380ccaa2f57f0d9: function(A, g) {
        const I = g, B = typeof I == "string" ? I : void 0;
        var E = a(B) ? 0 : Y(B, C.__wbindgen_malloc, C.__wbindgen_realloc), D = N;
        y().setInt32(A + 4, D, true), y().setInt32(A + 0, E, true);
      },
      __wbg___wbindgen_throw_41e9ee4f547fc59a: function(A, g) {
        throw new Error(k(A, g));
      },
      __wbg__wbg_cb_unref_dcc1a90847f04c41: function(A) {
        A._wbg_cb_unref();
      },
      __wbg_addEventListener_4d0db17c671ea324: function() {
        return o(function(A, g, I, B) {
          A.addEventListener(k(g, I), B);
        }, arguments);
      },
      __wbg_addEventListener_8d3235978b9c69a8: function() {
        return o(function(A, g, I, B, E) {
          A.addEventListener(k(g, I), B, E);
        }, arguments);
      },
      __wbg_apply_a910804df6e1e433: function() {
        return o(function(A, g, I) {
          return A.apply(g, I);
        }, arguments);
      },
      __wbg_arrayBuffer_f07be74819944ea2: function() {
        return o(function(A) {
          return A.arrayBuffer();
        }, arguments);
      },
      __wbg_call_1875a20c43a36133: function() {
        return o(function(A, g, I, B) {
          return A.call(g, I, B);
        }, arguments);
      },
      __wbg_call_187d372bd5fdd4aa: function() {
        return o(function(A, g, I) {
          return A.call(g, I);
        }, arguments);
      },
      __wbg_call_6137034ef55c9d0f: function() {
        return o(function(A, g) {
          return A.call(g);
        }, arguments);
      },
      __wbg_clearInterval_26ba580547547579: function(A) {
        return clearInterval(A);
      },
      __wbg_clearTimeout_3629d6209dfcc46e: function(A) {
        return clearTimeout(A);
      },
      __wbg_clipboarddata_new: function(A) {
        return s.__wrap(A);
      },
      __wbg_clipboarditem_new: function(A) {
        return V.__wrap(A);
      },
      __wbg_close_d3ed56b5763be5ae: function() {
        return o(function(A) {
          A.close();
        }, arguments);
      },
      __wbg_code_4cb6dbcfceec1eac: function(A) {
        return A.code;
      },
      __wbg_data_522f7abc70721269: function(A) {
        return A.data;
      },
      __wbg_debug_05be7ad0a1e623f8: function(A, g, I, B) {
        console.debug(A, g, I, B);
      },
      __wbg_debug_52bda05ddf50b736: function(A) {
        console.debug(A);
      },
      __wbg_dispatchEvent_57e1a8af186ff471: function() {
        return o(function(A, g) {
          return A.dispatchEvent(g);
        }, arguments);
      },
      __wbg_done_b41a1d26cdb37fb6: function(A) {
        return A.done;
      },
      __wbg_error_6614f5677eeead43: function(A, g, I, B) {
        console.error(A, g, I, B);
      },
      __wbg_error_757e9472f8410341: function(A, g) {
        let I, B;
        try {
          I = A, B = g, console.error(k(A, g));
        } finally {
          C.__wbindgen_free(I, B, 1);
        }
      },
      __wbg_error_c9cf3fc2064683a9: function(A) {
        console.error(A);
      },
      __wbg_fetch_9b478faef8cda538: function(A) {
        return fetch(A);
      },
      __wbg_from_296ca31f8d0f1c52: function(A) {
        return Array.from(A);
      },
      __wbg_getContext_635e36719cad2623: function() {
        return o(function(A, g, I) {
          const B = A.getContext(k(g, I));
          return a(B) ? 0 : H(B);
        }, arguments);
      },
      __wbg_getRandomValues_436a51d0629d84e1: function() {
        return o(function(A, g) {
          globalThis.crypto.getRandomValues(l(A, g));
        }, arguments);
      },
      __wbg_getRandomValues_869eb80731672410: function() {
        return o(function(A, g) {
          globalThis.crypto.getRandomValues(l(A, g));
        }, arguments);
      },
      __wbg_getRandomValues_a608c4436c19407a: function() {
        return o(function(A, g) {
          globalThis.crypto.getRandomValues(l(A, g));
        }, arguments);
      },
      __wbg_getTime_f5a55efff2585d5d: function(A) {
        return A.getTime();
      },
      __wbg_get_31af05bd4842a84f: function() {
        return o(function(A, g) {
          return Reflect.get(A, g);
        }, arguments);
      },
      __wbg_get_658f6698067d9515: function() {
        return o(function(A, g) {
          return Reflect.get(A, g);
        }, arguments);
      },
      __wbg_get_6c896e0571ddae51: function(A, g) {
        return A[g >>> 0];
      },
      __wbg_get_unchecked_288889d017702237: function(A, g) {
        return A[g >>> 0];
      },
      __wbg_info_26925f73a2eee895: function(A) {
        console.info(A);
      },
      __wbg_info_b68ad4a35d1670c2: function(A, g, I, B) {
        console.info(A, g, I, B);
      },
      __wbg_instanceof_ArrayBuffer_a99f175873e5d9b8: function(A) {
        let g;
        try {
          g = A instanceof ArrayBuffer;
        } catch {
          g = false;
        }
        return g;
      },
      __wbg_instanceof_CanvasRenderingContext2d_769208c72dcbf5e6: function(A) {
        let g;
        try {
          g = A instanceof CanvasRenderingContext2D;
        } catch {
          g = false;
        }
        return g;
      },
      __wbg_instanceof_Error_80a725f81f2e102d: function(A) {
        let g;
        try {
          g = A instanceof Error;
        } catch {
          g = false;
        }
        return g;
      },
      __wbg_instanceof_Object_67a83cdc00c5d141: function(A) {
        let g;
        try {
          g = A instanceof Object;
        } catch {
          g = false;
        }
        return g;
      },
      __wbg_instanceof_Response_b8758567269c30b2: function(A) {
        let g;
        try {
          g = A instanceof Response;
        } catch {
          g = false;
        }
        return g;
      },
      __wbg_instanceof_Uint8Array_828cef2aaacafc31: function(A) {
        let g;
        try {
          g = A instanceof Uint8Array;
        } catch {
          g = false;
        }
        return g;
      },
      __wbg_instanceof_Window_82d71df4eddf88bc: function(A) {
        let g;
        try {
          g = A instanceof Window;
        } catch {
          g = false;
        }
        return g;
      },
      __wbg_ironerror_new: function(A) {
        return T.__wrap(A);
      },
      __wbg_isArray_e15a2ff68ffdbef2: function(A) {
        return Array.isArray(A);
      },
      __wbg_iterator_e3c31c892080e444: function() {
        return Symbol.iterator;
      },
      __wbg_length_7f3c00c40364105e: function(A) {
        return A.length;
      },
      __wbg_length_d4bdea10311bd9cf: function(A) {
        return A.length;
      },
      __wbg_message_5f8387f0c32b90a7: function(A) {
        return A.message;
      },
      __wbg_name_e2eac7cdfa054f65: function(A) {
        return A.name;
      },
      __wbg_navigator_2156486643462a87: function(A) {
        return A.navigator;
      },
      __wbg_new_0_72d020f0c63443d4: function() {
        return /* @__PURE__ */ new Date();
      },
      __wbg_new_1dbf7428bba60a42: function(A) {
        return new Uint8Array(A);
      },
      __wbg_new_227d7c05414eb861: function() {
        return new Error();
      },
      __wbg_new_4059fb0406225e04: function() {
        return o(function(A, g) {
          return new WebSocket(k(A, g));
        }, arguments);
      },
      __wbg_new_617a8cdb8bb1130e: function() {
        return new Object();
      },
      __wbg_new_820eb53f620ccf82: function() {
        return o(function(A, g) {
          return new URL(k(A, g));
        }, arguments);
      },
      __wbg_new_83ca405eae412a62: function() {
        return o(function() {
          return new Headers();
        }, arguments);
      },
      __wbg_new_d92488e3f585ed02: function() {
        return o(function() {
          return new URLSearchParams();
        }, arguments);
      },
      __wbg_new_ee2291f50781bf1d: function() {
        return new Array();
      },
      __wbg_new_from_slice_37a3c3a194266cb8: function(A, g) {
        return new Uint32Array(HA(A, g));
      },
      __wbg_new_from_slice_9a868026ffa4208a: function(A, g) {
        return new Uint8Array(l(A, g));
      },
      __wbg_new_typed_b01cb72a8af741a3: function(A, g) {
        try {
          var I = { a: A, b: g }, B = (D, G) => {
            const i = I.a;
            I.a = 0;
            try {
              return UA(i, I.b, D, G);
            } finally {
              I.a = i;
            }
          };
          return new Promise(B);
        } finally {
          I.a = 0;
        }
      },
      __wbg_new_with_event_init_dict_92dd01bedbb7ac30: function() {
        return o(function(A, g, I) {
          return new CloseEvent(k(A, g), I);
        }, arguments);
      },
      __wbg_new_with_str_2d1292ac88699448: function() {
        return o(function(A, g) {
          return new Request(k(A, g));
        }, arguments);
      },
      __wbg_new_with_str_and_init_4618dee4e950224f: function() {
        return o(function(A, g, I) {
          return new Request(k(A, g), I);
        }, arguments);
      },
      __wbg_new_with_str_sequence_f79e461d58fdeacc: function() {
        return o(function(A, g, I) {
          return new WebSocket(k(A, g), I);
        }, arguments);
      },
      __wbg_new_with_u8_array_sequence_f380e3e783ae5617: function() {
        return o(function(A) {
          return new Blob(A);
        }, arguments);
      },
      __wbg_new_with_u8_clamped_array_and_sh_af3f0dee40638e18: function() {
        return o(function(A, g, I, B) {
          return new ImageData(LA(A, g), I >>> 0, B >>> 0);
        }, arguments);
      },
      __wbg_next_33784799010f1bbe: function(A) {
        return A.next;
      },
      __wbg_next_f4aac29c42af995c: function() {
        return o(function(A) {
          return A.next();
        }, arguments);
      },
      __wbg_now_aa4ccb83129e9e55: function() {
        return Date.now();
      },
      __wbg_now_d0b7f4bea9f38490: function(A) {
        return A.now();
      },
      __wbg_now_e7c6795a7f81e10f: function(A) {
        return A.now();
      },
      __wbg_of_d5dbecdc59e02e34: function(A) {
        return Array.of(A);
      },
      __wbg_ok_125878a7db256ecb: function(A) {
        return A.ok;
      },
      __wbg_performance_3fcf6e32a7e1ed0a: function(A) {
        return A.performance;
      },
      __wbg_performance_d0d7b03fc649b694: function(A) {
        const g = A.performance;
        return a(g) ? 0 : H(g);
      },
      __wbg_prototypesetcall_bc27214492979395: function(A, g, I) {
        Uint8Array.prototype.set.call(l(A, g), I);
      },
      __wbg_push_2baf45db356cf468: function(A, g) {
        return A.push(g);
      },
      __wbg_putImageData_11570c06086c9e05: function() {
        return o(function(A, g, I, B) {
          A.putImageData(g, I, B);
        }, arguments);
      },
      __wbg_queueMicrotask_9833f9a49df95a49: function(A) {
        return A.queueMicrotask;
      },
      __wbg_queueMicrotask_a72f977e97f23c5f: function(A) {
        queueMicrotask(A);
      },
      __wbg_readyState_d8514376867e415b: function(A) {
        return A.readyState;
      },
      __wbg_reason_832acaf5f4083b7b: function(A, g) {
        const I = g.reason, B = Y(I, C.__wbindgen_malloc, C.__wbindgen_realloc), E = N;
        y().setInt32(A + 4, E, true), y().setInt32(A + 0, B, true);
      },
      __wbg_removeEventListener_6e68185345978771: function() {
        return o(function(A, g, I, B) {
          A.removeEventListener(k(g, I), B);
        }, arguments);
      },
      __wbg_resolve_0076e10020304ede: function(A) {
        return Promise.resolve(A);
      },
      __wbg_search_c401c3a62c560ce7: function(A, g) {
        const I = g.search, B = Y(I, C.__wbindgen_malloc, C.__wbindgen_realloc), E = N;
        y().setInt32(A + 4, E, true), y().setInt32(A + 0, B, true);
      },
      __wbg_send_5420ae0bcef8c110: function() {
        return o(function(A, g) {
          A.send(g);
        }, arguments);
      },
      __wbg_send_b5bdd806efe4baf6: function() {
        return o(function(A, g, I) {
          A.send(k(g, I));
        }, arguments);
      },
      __wbg_session_new: function(A) {
        return P.__wrap(A);
      },
      __wbg_sessionterminationinfo_new: function(A) {
        return O.__wrap(A);
      },
      __wbg_setInterval_cbf1c35c6a692d37: function() {
        return o(function(A, g) {
          return setInterval(A, g);
        }, arguments);
      },
      __wbg_setTimeout_56bcdccbad22fd44: function() {
        return o(function(A, g) {
          return setTimeout(A, g);
        }, arguments);
      },
      __wbg_set_145a351398b48c65: function() {
        return o(function(A, g, I) {
          return Reflect.set(A, g, I);
        }, arguments);
      },
      __wbg_set_af5932e3f78b5648: function() {
        return o(function(A, g, I, B, E) {
          A.set(k(g, I), k(B, E));
        }, arguments);
      },
      __wbg_set_binaryType_21835bf0df8f70aa: function(A, g) {
        A.binaryType = cA[g];
      },
      __wbg_set_body_1fb0f1008bfc7df6: function(A, g) {
        A.body = g;
      },
      __wbg_set_code_f385a2b721072cea: function(A, g) {
        A.code = g;
      },
      __wbg_set_headers_40ad33dcf016613f: function(A, g) {
        A.headers = g;
      },
      __wbg_set_height_c9789c1c77eaedff: function(A, g) {
        A.height = g >>> 0;
      },
      __wbg_set_method_dcb32343247ec427: function(A, g, I) {
        A.method = k(g, I);
      },
      __wbg_set_once_8c45bbbfb71d1b34: function(A, g) {
        A.once = g !== 0;
      },
      __wbg_set_reason_b18f2b84e16d9b48: function(A, g, I) {
        A.reason = k(g, I);
      },
      __wbg_set_search_de4e8694f3b52c24: function(A, g, I) {
        A.search = k(g, I);
      },
      __wbg_set_width_b0e1267db4b196b5: function(A, g) {
        A.width = g >>> 0;
      },
      __wbg_stack_3b0d974bbf31e44f: function(A, g) {
        const I = g.stack, B = Y(I, C.__wbindgen_malloc, C.__wbindgen_realloc), E = N;
        y().setInt32(A + 4, E, true), y().setInt32(A + 0, B, true);
      },
      __wbg_static_accessor_GLOBAL_266715b9d96ba635: function() {
        const A = typeof global > "u" ? null : global;
        return a(A) ? 0 : H(A);
      },
      __wbg_static_accessor_GLOBAL_THIS_10fb7dc1ae063179: function() {
        const A = typeof globalThis > "u" ? null : globalThis;
        return a(A) ? 0 : H(A);
      },
      __wbg_static_accessor_SELF_0b583911f537483a: function() {
        const A = typeof self > "u" ? null : self;
        return a(A) ? 0 : H(A);
      },
      __wbg_static_accessor_WINDOW_d7f903d1508cbdc4: function() {
        const A = typeof window > "u" ? null : window;
        return a(A) ? 0 : H(A);
      },
      __wbg_statusText_bceed558320dd590: function(A, g) {
        const I = g.statusText, B = Y(I, C.__wbindgen_malloc, C.__wbindgen_realloc), E = N;
        y().setInt32(A + 4, E, true), y().setInt32(A + 0, B, true);
      },
      __wbg_status_ce0a98d3c57125f3: function(A) {
        return A.status;
      },
      __wbg_then_c949d5a25a4e78f8: function(A, g, I) {
        return A.then(g, I);
      },
      __wbg_then_e71170d78fcf8954: function(A, g) {
        return A.then(g);
      },
      __wbg_toString_0a6162345e699dea: function(A) {
        return A.toString();
      },
      __wbg_toString_83fab3d7c865ecd3: function(A) {
        return A.toString();
      },
      __wbg_url_7bfada5c2297bc70: function(A, g) {
        const I = g.url, B = Y(I, C.__wbindgen_malloc, C.__wbindgen_realloc), E = N;
        y().setInt32(A + 4, E, true), y().setInt32(A + 0, B, true);
      },
      __wbg_userAgent_cde4c65cc98db425: function() {
        return o(function(A, g) {
          const I = g.userAgent, B = Y(I, C.__wbindgen_malloc, C.__wbindgen_realloc), E = N;
          y().setInt32(A + 4, E, true), y().setInt32(A + 0, B, true);
        }, arguments);
      },
      __wbg_value_f3c585ee8f5ba40c: function(A) {
        return A.value;
      },
      __wbg_warn_13abc63e0d4b3527: function(A) {
        console.warn(A);
      },
      __wbg_warn_88929063e36ba285: function(A, g, I, B) {
        console.warn(A, g, I, B);
      },
      __wbg_wasClean_b246ecade802c6ff: function(A) {
        return A.wasClean;
      },
      __wbindgen_generic_0000000000000001: function(A, g) {
        return q(A, g, aA);
      },
      __wbindgen_generic_0000000000000002: function(A, g) {
        return q(A, g, RA);
      },
      __wbindgen_generic_0000000000000003: function(A, g) {
        return q(A, g, FA);
      },
      __wbindgen_generic_0000000000000004: function(A, g) {
        return q(A, g, JA);
      },
      __wbindgen_generic_0000000000000005: function(A, g) {
        return q(A, g, MA);
      },
      __wbindgen_generic_0000000000000006: function(A, g) {
        return q(A, g, yA);
      },
      __wbindgen_generic_0000000000000007: function(A) {
        return A;
      },
      __wbindgen_generic_0000000000000008: function(A, g) {
        return k(A, g);
      },
      __wbindgen_init_externref_table: function() {
        const A = C.__wbindgen_externrefs, g = A.grow(4);
        A.set(0, void 0), A.set(g + 0, void 0), A.set(g + 1, null), A.set(g + 2, true), A.set(g + 3, false);
      }
    }
  };
}
function MA(Q, A) {
  C.wasm_bindgen__convert__closures_____invoke__h43f4169ee688cf51(Q, A);
}
function yA(Q, A) {
  C.wasm_bindgen__convert__closures_____invoke__h4a921f6235159135(Q, A);
}
function RA(Q, A, g) {
  C.wasm_bindgen__convert__closures_____invoke__h151689d2a7caaa74(Q, A, g);
}
function FA(Q, A, g) {
  C.wasm_bindgen__convert__closures_____invoke__h151689d2a7caaa74_69(Q, A, g);
}
function JA(Q, A, g) {
  C.wasm_bindgen__convert__closures_____invoke__h151689d2a7caaa74_70(Q, A, g);
}
function aA(Q, A, g) {
  const I = C.wasm_bindgen__convert__closures_____invoke__h9b3b88103f98b444(Q, A, g);
  if (I[1])
    throw K(I[0]);
}
function UA(Q, A, g, I) {
  C.wasm_bindgen__convert__closures_____invoke__h7fd80c0d0f68485d(Q, A, g, I);
}
const cA = ["blob", "arraybuffer"], n = typeof FinalizationRegistry > "u" ? { register: () => {
}, unregister: () => {
} } : new FinalizationRegistry((Q) => C.__wbg_clipboarddata_free(Q, 1)), $ = typeof FinalizationRegistry > "u" ? { register: () => {
}, unregister: () => {
} } : new FinalizationRegistry((Q) => C.__wbg_clipboarditem_free(Q, 1)), f = typeof FinalizationRegistry > "u" ? { register: () => {
}, unregister: () => {
} } : new FinalizationRegistry((Q) => C.__wbg_desktopsize_free(Q, 1)), AA = typeof FinalizationRegistry > "u" ? { register: () => {
}, unregister: () => {
} } : new FinalizationRegistry((Q) => C.__wbg_deviceevent_free(Q, 1)), gA = typeof FinalizationRegistry > "u" ? { register: () => {
}, unregister: () => {
} } : new FinalizationRegistry((Q) => C.__wbg_extension_free(Q, 1)), IA = typeof FinalizationRegistry > "u" ? { register: () => {
}, unregister: () => {
} } : new FinalizationRegistry((Q) => C.__wbg_inputtransaction_free(Q, 1)), BA = typeof FinalizationRegistry > "u" ? { register: () => {
}, unregister: () => {
} } : new FinalizationRegistry((Q) => C.__wbg_ironerror_free(Q, 1)), QA = typeof FinalizationRegistry > "u" ? { register: () => {
}, unregister: () => {
} } : new FinalizationRegistry((Q) => C.__wbg_rdcleanpathdetails_free(Q, 1)), CA = typeof FinalizationRegistry > "u" ? { register: () => {
}, unregister: () => {
} } : new FinalizationRegistry((Q) => C.__wbg_rdpfile_free(Q, 1)), EA = typeof FinalizationRegistry > "u" ? { register: () => {
}, unregister: () => {
} } : new FinalizationRegistry((Q) => C.__wbg_session_free(Q, 1)), v = typeof FinalizationRegistry > "u" ? { register: () => {
}, unregister: () => {
} } : new FinalizationRegistry((Q) => C.__wbg_sessionbuilder_free(Q, 1)), DA = typeof FinalizationRegistry > "u" ? { register: () => {
}, unregister: () => {
} } : new FinalizationRegistry((Q) => C.__wbg_sessionterminationinfo_free(Q, 1));
function H(Q) {
  const A = C.__externref_table_alloc();
  return C.__wbindgen_externrefs.set(A, Q), A;
}
function GA(Q) {
  if (typeof Q == "number" && (Q >= 1114112 || Q >= 55296 && Q < 57344)) throw new Error(`expected a valid Unicode scalar value, found ${Q}`);
}
function Z(Q, A) {
  if (!(Q instanceof A))
    throw new Error(`expected instance of ${A.name}`);
}
const iA = typeof FinalizationRegistry > "u" ? { register: () => {
}, unregister: () => {
} } : new FinalizationRegistry((Q) => C.__wbindgen_destroy_closure(Q.a, Q.b));
function e(Q) {
  const A = typeof Q;
  if (A == "number" || A == "boolean" || Q == null)
    return `${Q}`;
  if (A == "string")
    return `"${Q}"`;
  if (A == "symbol") {
    const B = Q.description;
    return B == null ? "Symbol" : `Symbol(${B})`;
  }
  if (A == "function") {
    const B = Q.name;
    return typeof B == "string" && B.length > 0 ? `Function(${B})` : "Function";
  }
  if (Array.isArray(Q)) {
    const B = Q.length;
    let E = "[";
    B > 0 && (E += e(Q[0]));
    for (let D = 1; D < B; D++)
      E += ", " + e(Q[D]);
    return E += "]", E;
  }
  const g = /\[object ([^\]]+)\]/.exec(toString.call(Q));
  let I;
  if (g && g.length > 1)
    I = g[1];
  else
    return toString.call(Q);
  if (I == "Object")
    try {
      return "Object(" + JSON.stringify(Q) + ")";
    } catch {
      return "Object";
    }
  return Q instanceof Error ? `${Q.name}: ${Q.message}
${Q.stack}` : I;
}
function hA(Q, A) {
  Q = Q >>> 0;
  const g = y(), I = [];
  for (let B = Q; B < Q + 4 * A; B += 4)
    I.push(C.__wbindgen_externrefs.get(g.getUint32(B, true)));
  return C.__externref_drop_slice(Q, A), I;
}
function HA(Q, A) {
  return Q = Q >>> 0, KA().subarray(Q / 4, Q / 4 + A);
}
function l(Q, A) {
  return Q = Q >>> 0, j().subarray(Q / 1, Q / 1 + A);
}
function LA(Q, A) {
  return Q = Q >>> 0, sA().subarray(Q / 1, Q / 1 + A);
}
let L = null;
function y() {
  return (L === null || L.buffer.detached === true || L.buffer.detached === void 0 && L.buffer !== C.memory.buffer) && (L = new DataView(C.memory.buffer)), L;
}
function k(Q, A) {
  return ZA(Q >>> 0, A);
}
let x = null;
function KA() {
  return (x === null || x.byteLength === 0) && (x = new Uint32Array(C.memory.buffer)), x;
}
let W = null;
function j() {
  return (W === null || W.byteLength === 0) && (W = new Uint8Array(C.memory.buffer)), W;
}
let m = null;
function sA() {
  return (m === null || m.byteLength === 0) && (m = new Uint8ClampedArray(C.memory.buffer)), m;
}
function o(Q, A) {
  try {
    return Q.apply(this, A);
  } catch (g) {
    const I = H(g);
    C.__wbindgen_exn_store(I);
  }
}
function a(Q) {
  return Q == null;
}
function q(Q, A, g) {
  const I = { a: Q, b: A, cnt: 1 }, B = (...E) => {
    I.cnt++;
    const D = I.a;
    I.a = 0;
    try {
      return g(D, I.b, ...E);
    } finally {
      I.a = D, B._wbg_cb_unref();
    }
  };
  return B._wbg_cb_unref = () => {
    --I.cnt === 0 && (C.__wbindgen_destroy_closure(I.a, I.b), I.a = 0, iA.unregister(I));
  }, iA.register(B, I, I), B;
}
function SA(Q, A) {
  const g = A(Q.length * 1, 1) >>> 0;
  return j().set(Q, g / 1), N = Q.length, g;
}
function Y(Q, A, g) {
  if (g === void 0) {
    const G = p.encode(Q), i = A(G.length, 1) >>> 0;
    return j().subarray(i, i + G.length).set(G), N = G.length, i;
  }
  let I = Q.length, B = A(I, 1) >>> 0;
  const E = j();
  let D = 0;
  for (; D < I; D++) {
    const G = Q.charCodeAt(D);
    if (G > 127) break;
    E[B + D] = G;
  }
  if (D !== I) {
    D !== 0 && (Q = Q.slice(D)), B = g(B, I, I = D + Q.length * 3, 1) >>> 0;
    const G = j().subarray(B + D, B + I), i = p.encodeInto(Q, G);
    D += i.written, B = g(B, I, D, 1) >>> 0;
  }
  return N = D, B;
}
function K(Q) {
  const A = C.__wbindgen_externrefs.get(Q);
  return C.__externref_table_dealloc(Q), A;
}
let X = new TextDecoder("utf-8", { ignoreBOM: true, fatal: true });
X.decode();
const qA = 2146435072;
let r = 0;
function ZA(Q, A) {
  return r += A, r >= qA && (X = new TextDecoder("utf-8", { ignoreBOM: true, fatal: true }), X.decode(), r = A), X.decode(j().subarray(Q, Q + A));
}
const p = new TextEncoder();
"encodeInto" in p || (p.encodeInto = function(Q, A) {
  const g = p.encode(Q);
  return A.set(g), {
    read: Q.length,
    written: g.length
  };
});
let N = 0, C;
function jA(Q, A) {
  return C = Q.exports, L = null, x = null, W = null, m = null, C.__wbindgen_start(), C;
}
async function dA(Q, A) {
  if (typeof Response == "function" && Q instanceof Response) {
    if (!Q.ok)
      throw new Error(`failed to fetch Wasm: ${Q.status} ${Q.statusText} fetching '${Q.url}'`);
    if (typeof WebAssembly.instantiateStreaming == "function")
      try {
        return await WebAssembly.instantiateStreaming(Q, A);
      } catch (B) {
        if (g(Q.type) && Q.headers.get("Content-Type") !== "application/wasm")
          console.warn("`WebAssembly.instantiateStreaming` failed because your server does not serve Wasm with `application/wasm` MIME type. Falling back to `WebAssembly.instantiate` which is slower. Original error:\n", B);
        else
          throw B;
      }
    const I = await Q.arrayBuffer();
    return await WebAssembly.instantiate(I, A);
  } else {
    const I = await WebAssembly.instantiate(Q, A);
    return I instanceof WebAssembly.Instance ? { instance: I, module: Q } : I;
  }
  function g(I) {
    switch (I) {
      case "basic":
      case "cors":
      case "default":
        return true;
    }
    return false;
  }
}
async function lA(Q) {
  if (C !== void 0) return C;
  Q !== void 0 && (Object.getPrototypeOf(Q) === Object.prototype ? { module_or_path: Q } = Q : console.warn("using deprecated parameters for the initialization function; pass a single object instead")), Q === void 0 && (Q = NA);
  const A = kA();
  (typeof Q == "string" || typeof Request == "function" && Q instanceof Request || typeof URL == "function" && Q instanceof URL) && (Q = fetch(Q));
  const { instance: g, module: I } = await dA(await Q, A);
  return jA(g);
}
const vA = {
  /** General error, without a more specific reason. */
  GENERAL: 1,
  /** The proxy credential has expired. */
  EXPIRED: 100,
  /** The proxy credential is not valid yet. */
  NOT_YET_VALID: 101,
  /** The proxy credential has been revoked. */
  REVOKED: 102,
  /** The proxy could not connect to the target. */
  TARGET_UNREACHABLE: 103,
  /** The proxy could not resolve the target host name. */
  DNS_RESOLUTION_FAILED: 104,
  /** The TLS handshake between the proxy and the target failed. */
  TLS_HANDSHAKE_FAILED: 105
}, b = {
  /**
   * Request file size (8-byte unsigned integer).
   * When set: position must be 0, size must be 8.
   */
  SIZE: 1,
  /**
   * Request byte range from file.
   * When set: position and size define the requested range.
   */
  RANGE: 2
};
function bA(Q) {
  return new w("files_available_callback", Q);
}
function xA(Q) {
  return new w("file_contents_request_callback", Q);
}
function WA(Q) {
  return new w("file_contents_response_callback", Q);
}
function mA(Q) {
  return new w("lock_callback", Q);
}
function pA(Q) {
  return new w("unlock_callback", Q);
}
function VA(Q) {
  return new w("locks_expired_callback", Q);
}
function TA(Q) {
  return new w("format_list_response_callback", Q);
}
const rA = {
  PostScript: "MS Publisher Imagesetter",
  MicrosoftPrintToPdf: "Microsoft Print to PDF"
};
function eA(Q) {
  return new w("print_job_stream_callbacks", Q);
}
function uA(Q) {
  return new w("printer_name", Q);
}
function _A(Q) {
  return new w("printer_device_id", Q);
}
function $A(Q) {
  return new w("printer_driver_name", Q);
}
function tA(Q) {
  return new w("request_file_contents", Q);
}
function PA(Q) {
  return new w("submit_file_contents", Q);
}
function OA(Q) {
  return new w("initiate_file_copy", Q);
}
class XA {
  constructor() {
    __publicField(this, "chunks", []);
    __publicField(this, "_bytesWritten", 0);
    __publicField(this, "finalized", false);
  }
  get bytesWritten() {
    return this._bytesWritten;
  }
  async write(A) {
    if (this.finalized)
      throw new Error("BlobWriteHandle: write after finalize/abort");
    this.chunks.push(A), this._bytesWritten += A.length;
  }
  async finalize() {
    if (this.finalized)
      throw new Error("BlobWriteHandle: already finalized or aborted");
    this.finalized = true;
    const A = new Blob(this.chunks);
    return this.chunks = [], A;
  }
  async abort() {
    this.finalized || (this.finalized = true, this.chunks = []);
  }
}
class wA {
  constructor() {
    __publicField(this, "name", "blob");
  }
  async createWriteHandle(A, g) {
    return new XA();
  }
  async dispose() {
  }
}
class zA {
  constructor(A, g, I, B) {
    __publicField(this, "writable");
    __publicField(this, "_bytesWritten", 0);
    __publicField(this, "finalized", false);
    this.fileHandle = A, this.sessionDir = g, this.entryName = I, this.writable = B;
  }
  get bytesWritten() {
    return this._bytesWritten;
  }
  async write(A) {
    if (this.finalized || !this.writable)
      throw new Error("OpfsWriteHandle: write after finalize/abort");
    await this.writable.write(A), this._bytesWritten += A.length;
  }
  async finalize() {
    if (this.finalized)
      throw new Error("OpfsWriteHandle: already finalized or aborted");
    return this.finalized = true, this.writable && (await this.writable.close(), this.writable = void 0), this.fileHandle.getFile();
  }
  async abort() {
    if (!this.finalized) {
      if (this.finalized = true, this.writable) {
        try {
          await this.writable.abort();
        } catch {
        }
        this.writable = void 0;
      }
      try {
        await this.sessionDir.removeEntry(this.entryName);
      } catch {
      }
    }
  }
}
const _d = class _d {
  constructor(A, g, I) {
    __publicField(this, "name", "opfs");
    /** Sequence counter for generating unique temp file names. */
    __publicField(this, "sequence", 0);
    this.opfsRoot = A, this.sessionDir = g, this.sessionId = I;
  }
  /**
   * Create an OPFS backend, including the per-session directory.
   *
   * Call {@link probe} first to verify OPFS is available before
   * constructing -- this factory assumes OPFS works.
   */
  static async create(A, g) {
    const I = g ?? `s-${Date.now()}-${Math.random().toString(36).slice(2, 8)}`, B = await A.getDirectoryHandle("ironrdp-transfers", { create: true }), E = await B.getDirectoryHandle(I, { create: true });
    return _d.cleanupStale(B, I), new _d(A, E, I);
  }
  // 24 hours
  /**
   * Remove session directories older than {@link STALE_SESSION_THRESHOLD_MS}.
   *
   * Session IDs generated by {@link create} embed a timestamp in the
   * format `s-{Date.now()}-{random}`.  This method parses that timestamp
   * to determine age.  Directories with unparsable names or those
   * belonging to the current session are skipped.
   */
  static async cleanupStale(A, g) {
    const I = Date.now();
    try {
      for await (const B of A.keys()) {
        if (B === g)
          continue;
        const E = /^s-(\d+)-/.exec(B);
        if (!E)
          continue;
        const D = Number(E[1]);
        if (I - D > _d.STALE_SESSION_THRESHOLD_MS)
          try {
            await A.removeEntry(B, { recursive: true });
          } catch {
          }
      }
    } catch {
    }
  }
  /**
   * Probe whether OPFS is usable in the current context.
   *
   * Performs a full round-trip: creates a temp file, opens a writable,
   * closes it, and deletes it.  This catches environments where the API
   * exists but throws at runtime (e.g., some private browsing modes).
   */
  static async probe(A) {
    try {
      return await (await (await A.getFileHandle(".ironrdp-opfs-probe", { create: true })).createWritable()).close(), await A.removeEntry(".ironrdp-opfs-probe"), true;
    } catch {
      return false;
    }
  }
  async createWriteHandle(A, g) {
    if (!this.sessionDir)
      throw new Error("OpfsStorageBackend: backend has been disposed");
    const B = `${this.sequence++}-${nA(A)}`, E = await this.sessionDir.getFileHandle(B, { create: true }), D = await E.createWritable();
    return new zA(E, this.sessionDir, B, D);
  }
  async dispose() {
    if (!this.sessionDir)
      return;
    const A = this.sessionDir;
    this.sessionDir = void 0;
    try {
      const g = await this.opfsRoot.getDirectoryHandle("ironrdp-transfers");
      await g.removeEntry(this.sessionId, { recursive: true });
      let I = false;
      for await (const B of g.values()) {
        I = true;
        break;
      }
      I || await this.opfsRoot.removeEntry("ironrdp-transfers");
    } catch (g) {
      console.debug("OPFS session directory removal failed, falling back to per-file cleanup:", g);
      try {
        for await (const I of A.keys())
          try {
            await A.removeEntry(I);
          } catch {
          }
      } catch {
      }
    }
  }
};
/** Maximum age (in milliseconds) before a session directory is considered stale. */
__publicField(_d, "STALE_SESSION_THRESHOLD_MS", 1440 * 60 * 1e3);
let d = _d;
function nA(Q) {
  let A = Q.replace(/[\u0000-\u001f]/g, "");
  A = A.replace(/[/\\]/g, "_"), A = A.replace(/^\.+/, ""), A.length === 0 && (A = "unnamed");
  const g = new TextEncoder();
  if (g.encode(A).byteLength > 200) {
    for (; g.encode(A).byteLength > 200; )
      A = A.slice(0, -1);
    A.length === 0 && (A = "unnamed");
  }
  return A;
}
async function fA(Q = "auto", A) {
  var _a, _b;
  if (Q === "blob")
    return new wA();
  if (typeof ((_b = (_a = globalThis.navigator) == null ? void 0 : _a.storage) == null ? void 0 : _b.getDirectory) == "function")
    try {
      const g = await navigator.storage.getDirectory();
      if (await d.probe(g))
        return d.create(g, A);
      console.debug("OPFS probe failed (createWritable not functional), falling back to blob storage");
    } catch (g) {
      console.debug("OPFS unavailable, falling back to blob storage:", g);
    }
  return new wA();
}
const _F = class _F {
  constructor(A) {
    __publicField(this, "session");
    __publicField(this, "chunkSize");
    __publicField(this, "onUploadStarted");
    __publicField(this, "onUploadFinished");
    __publicField(this, "storagePreference");
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    __publicField(this, "eventHandlers", /* @__PURE__ */ new Map());
    __publicField(this, "storageBackend");
    __publicField(this, "storageBackendReady");
    __publicField(this, "activeDownloads", /* @__PURE__ */ new Map());
    __publicField(this, "uploadState");
    // Upload paste-window watchdog. Armed when we advertise an upload, disarmed on the
    // first FileContentsRequest (the remote pulled the files). Being armed is the single
    // "advertised but not yet pulled" signal: on timeout it resumes monitoring and fails
    // the never-pulled upload, and handleFormatListResponse consults it to fail a refused
    // paste early (handleUnlock is a deliberate no-op: Unlock is not a reliable failure
    // signal). Upload completion/failure run only after that first request, so they need
    // not touch it.
    __publicField(this, "pasteAckTimeout");
    // Upload inactivity watchdog. Armed/reset on each FileContentsRequest once the remote
    // starts pulling; fires if pulls stop for UPLOAD_INACTIVITY_TIMEOUT_MS (a stalled or
    // clipboard-superseded paste) to release uploadState. Complements the paste-ack
    // watchdog above, which only covers the "advertised but never pulled" case.
    __publicField(this, "uploadInactivityTimeout");
    // True between onUploadStarted (suppress monitoring) and onUploadFinished
    // (resume), so resume fires exactly once even though it is now deferred until
    // the paste is pulled, times out, fails, or the provider is disposed.
    __publicField(this, "uploadMonitoringSuppressed", false);
    // DroppedFile metadata retained after upload completes so re-paste works
    // without re-dropping. Cleared when a new upload starts or the manager is disposed.
    __publicField(this, "retainedFiles");
    __publicField(this, "availableFiles", []);
    // Clipboard lock ID received with the most recent file list. The Rust layer
    // acquires this lock automatically when FileGroupDescriptorW is detected in
    // the FormatList. Downloads use it instead of explicit lock/unlock calls -
    // the lock lifecycle is managed entirely by the Rust cliprdr processor.
    __publicField(this, "clipDataId");
    __publicField(this, "nextStreamId", 1);
    __publicField(this, "disposed", false);
    this.chunkSize = (A == null ? void 0 : A.chunkSize) ?? 65536, this.onUploadStarted = A == null ? void 0 : A.onUploadStarted, this.onUploadFinished = A == null ? void 0 : A.onUploadFinished;
    const g = A == null ? void 0 : A.storageBackend;
    if (typeof g == "object" && g !== null) {
      if (typeof g.createWriteHandle != "function" || typeof g.dispose != "function")
        throw new Error(
          "storageBackend: expected 'auto', 'blob', or a FileStorageBackend with createWriteHandle() and dispose() methods"
        );
      this.storageBackend = g, this.storagePreference = "auto";
    } else {
      const I = g ?? "auto";
      if (I !== "auto" && I !== "blob")
        throw new Error(
          `storageBackend: invalid preference '${I}', expected 'auto', 'blob', or a FileStorageBackend instance`
        );
      this.storagePreference = I;
    }
  }
  /**
   * Set the session instance after connection is established.
   * Called by the web component's connect() flow via the FileTransferProvider interface.
   */
  setSession(A) {
    this.session = A;
  }
  ensureSession() {
    if (this.session === void 0)
      throw new Error("RdpFileTransferProvider: Session not available. Ensure connect() has been called.");
    return this.session;
  }
  /**
   * Lazily initialize and return the storage backend.
   *
   * Detection is performed once and cached.  Concurrent callers share
   * the same initialization promise so the probe only runs once.
   * If detection fails the cached promise is cleared so subsequent
   * downloads can retry.
   */
  async ensureStorageBackend() {
    return this.storageBackend ? this.storageBackend : (this.storageBackendReady || (this.storageBackendReady = fA(this.storagePreference).then((A) => (this.storageBackend = A, console.debug(`File transfer storage: ${A.name}`), A)).catch((A) => {
      throw this.storageBackendReady = void 0, A;
    })), this.storageBackendReady);
  }
  // --- Extension-based session method wrappers ---
  // These replace direct session.requestFileContents() etc. calls
  // with invokeExtension() to keep the Session interface protocol-agnostic.
  sendRequestFileContents(A, g, I, B, E, D) {
    this.ensureSession().invokeExtension(
      tA({
        stream_id: A,
        file_index: g,
        flags: I,
        position: B,
        size: E,
        clip_data_id: D
      })
    );
  }
  sendSubmitFileContents(A, g, I) {
    var _a;
    (_a = this.session) == null ? void 0 : _a.invokeExtension(PA({ stream_id: A, is_error: g, data: I }));
  }
  sendInitiateFileCopy(A) {
    this.ensureSession().invokeExtension(OA(A));
  }
  /**
   * Returns the extension objects to register on the SessionBuilder before connect().
   * Implements the FileTransferProvider interface.
   */
  getBuilderExtensions() {
    return [
      bA(
        (A, g) => this.handleFilesAvailable(A, g)
      ),
      xA((A) => this.handleFileContentsRequest(A)),
      WA((A) => this.handleFileContentsResponse(A)),
      mA((A) => this.handleLock(A)),
      pA((A) => this.handleUnlock(A)),
      VA((A) => this.handleLocksExpired(A)),
      TA((A) => this.handleFormatListResponse(A))
    ];
  }
  /**
   * Register an event handler.
   *
   * @param event - Event name
   * @param handler - Event handler function
   *
   * @example
   * ```typescript
   * manager.on('download-progress', (progress) => {
   *   console.log(`${progress.fileName}: ${progress.percentage}%`);
   * });
   * ```
   */
  on(A, g) {
    this.eventHandlers.has(A) || this.eventHandlers.set(A, /* @__PURE__ */ new Set()), this.eventHandlers.get(A).add(g);
  }
  /**
   * Unregister an event handler.
   *
   * @param event - Event name
   * @param handler - Event handler function to remove
   */
  off(A, g) {
    const I = this.eventHandlers.get(A);
    I && I.delete(g);
  }
  /**
   * Emit an event to all registered handlers.
   */
  emit(A, ...g) {
    const I = this.eventHandlers.get(A);
    if (I)
      for (const B of I)
        try {
          B(...g);
        } catch (E) {
          console.error(`Error in ${A} handler:`, E);
        }
  }
  /**
   * Download a single file from the remote.
   *
   * Returns a {@link DownloadHandle} with a `transferId` available synchronously
   * (for immediate UI association) and a `completion` promise that resolves with
   * the downloaded blob.
   *
   * @param fileInfo - File metadata from 'files-available' event
   * @param fileIndex - Index of the file in the original file list
   * @returns Handle with synchronous transferId and async completion
   *
   * @example
   * ```typescript
   * const { transferId, completion } = manager.downloadFile(fileInfo, 0);
   * // transferId is available immediately for UI binding
   * const blob = await completion;
   * saveAs(blob, fileInfo.name);
   * ```
   */
  downloadFile(A, g) {
    const I = this.generateStreamId(), B = this.executeDownload(A, g, I);
    return { transferId: I, completion: B };
  }
  /**
   * Internal: execute the async download workflow for a single file.
   */
  async executeDownload(A, g, I) {
    const B = this.clipDataId, E = new Promise((D, G) => {
      const i = {
        fileInfo: A,
        fileIndex: g,
        streamId: I,
        clipDataId: B,
        bytesReceived: 0,
        resolve: D,
        reject: G
      };
      this.activeDownloads.set(I, i);
    });
    try {
      this.sendRequestFileContents(I, g, b.SIZE, 0, 8, B);
    } catch (D) {
      this.activeDownloads.delete(I);
      const G = {
        message: "Failed to request file size",
        transferId: I,
        fileIndex: g,
        fileName: A.name,
        direction: "download",
        cause: D
      };
      throw this.emit("error", G), new Error(G.message, { cause: D });
    }
    return E;
  }
  /**
   * Download multiple files sequentially.
   *
   * @param files - Array of FileInfo from 'files-available' event
   * @returns AsyncGenerator yielding file/blob pairs as they complete
   *
   * @example
   * ```typescript
   * for await (const { file, blob } of manager.downloadFiles(files)) {
   *   saveAs(blob, file.name);
   * }
   * ```
   */
  async *downloadFiles(A) {
    for (let g = 0; g < A.length; g++) {
      const I = A[g], { transferId: B, completion: E } = this.downloadFile(I, g), D = await E;
      yield { file: I, blob: D, transferId: B };
    }
  }
  /**
   * Download multiple files concurrently with configurable parallelism.
   *
   * This method initiates multiple file downloads in parallel, improving
   * performance for multi-file transfers. Each file uses an independent
   * clipboard lock and stream ID.
   *
   * @param files - Array of FileInfo from 'files-available' event
   * @param options - Download options
   * @param options.maxConcurrent - Maximum concurrent downloads (default: 3)
   * @returns Promise resolving to map of fileIndex to Blob
   *
   * @example
   * ```typescript
   * const blobs = await manager.downloadFilesConcurrent(files, { maxConcurrent: 5 });
   * files.forEach((file, i) => saveAs(blobs.get(i)!, file.name));
   * ```
   */
  async downloadFilesConcurrent(A, g = {}) {
    const I = g.maxConcurrent ?? 3, B = /* @__PURE__ */ new Map(), E = [], D = A.map((i, R) => async () => {
      try {
        const { completion: h } = this.downloadFile(i, R), U = await h;
        B.set(R, U);
      } catch (h) {
        E.push({ index: R, error: h });
      }
    }), G = [];
    for (const i of D) {
      const R = i().finally(() => {
        G.splice(G.indexOf(R), 1);
      });
      G.push(R), G.length >= I && await Promise.race(G);
    }
    if (await Promise.all(G), E.length > 0)
      throw new Error(
        `Failed to download ${E.length} file(s): ` + E.map((i) => `file ${i.index}: ${i.error}`).join(", ")
      );
    return B;
  }
  /**
   * Upload files to the remote.
   *
   * Returns an {@link UploadHandle} with per-file `transferIds` available
   * synchronously (for immediate UI association) and a `completion` promise
   * that resolves when all files have been uploaded.
   *
   * @param files - Array of File objects to upload
   * @returns Handle with synchronous transferIds and async completion
   *
   * @example
   * ```typescript
   * const files = await manager.showFilePicker({ multiple: true });
   * const { transferIds, completion } = manager.uploadFiles(files);
   * // transferIds available immediately for UI binding
   * await completion;
   * ```
   */
  uploadFiles(A) {
    this.uploadState !== void 0 && this.supersedeUpload(), this.retainedFiles = void 0;
    const g = _F.normalizeToDroppedFiles(A), I = /* @__PURE__ */ new Map();
    for (let i = 0; i < g.length; i++)
      I.set(i, this.generateStreamId());
    const B = g.map((i) => ({
      name: i.name,
      size: i.size,
      lastModified: i.lastModified,
      path: i.path,
      isDirectory: i.isDirectory
    })), E = g.filter((i) => i.isDirectory !== true).length, D = g.map((i) => i.file), G = new Promise((i, R) => {
      this.uploadState = {
        files: D,
        droppedFiles: g,
        failedFiles: /* @__PURE__ */ new Set(),
        expectedFileCount: E,
        completedFiles: /* @__PURE__ */ new Set(),
        bytesServed: /* @__PURE__ */ new Map(),
        activeReaders: /* @__PURE__ */ new Map(),
        readerTimeouts: /* @__PURE__ */ new Map(),
        transferIds: I,
        resolve: i,
        reject: R
      }, this.suppressUploadMonitoring();
      try {
        this.sendInitiateFileCopy(B), this.emit("upload-batch-started", I, g);
      } catch (h) {
        this.uploadState = void 0, this.resumeUploadMonitoring();
        const U = {
          message: "Failed to initiate file upload",
          direction: "upload",
          cause: h
        };
        this.emit("error", U), R(new Error(U.message, { cause: h }));
        return;
      }
      this.armPasteAckWatchdog();
    });
    return { transferIds: I, completion: G };
  }
  /** Whether an upload batch is currently advertised or in flight (a fresh upload, or a re-paste
   *  rebuilt from retained files). */
  isUploadInProgress() {
    return this.uploadState !== void 0;
  }
  /** Suppress clipboard monitoring for an upload's paste window. Idempotent: a supersede keeps
   *  monitoring suppressed across the old->new upload, so this must not re-fire `onUploadStarted`. */
  suppressUploadMonitoring() {
    var _a;
    if (!this.uploadMonitoringSuppressed) {
      this.uploadMonitoringSuppressed = true;
      try {
        (_a = this.onUploadStarted) == null ? void 0 : _a.call(this);
      } catch (A) {
        console.error("Error in onUploadStarted callback:", A);
      }
    }
  }
  /** Resume clipboard monitoring (idempotent: fires onUploadFinished at most once
   *  per upload, since the resume point is now deferred past the wire send). */
  resumeUploadMonitoring() {
    var _a;
    if (this.uploadMonitoringSuppressed) {
      this.uploadMonitoringSuppressed = false;
      try {
        (_a = this.onUploadFinished) == null ? void 0 : _a.call(this);
      } catch (A) {
        console.error("Error in onUploadFinished callback:", A);
      }
    }
  }
  /** Arm the paste-acknowledgment watchdog for the current upload. */
  armPasteAckWatchdog() {
    this.clearPasteAckWatchdog(), this.clearUploadInactivityWatchdog(), this.pasteAckTimeout = setTimeout(() => {
      this.pasteAckTimeout = void 0;
      const A = this.uploadState;
      A !== void 0 && A.isRePaste !== true ? this.failPendingUpload("The remote did not request the files in time, so the paste was not completed") : this.resumeUploadMonitoring();
    }, _F.PASTE_ACK_TIMEOUT_MS);
  }
  /** Disarm the paste-acknowledgment watchdog, if armed. */
  clearPasteAckWatchdog() {
    this.pasteAckTimeout !== void 0 && (clearTimeout(this.pasteAckTimeout), this.pasteAckTimeout = void 0);
  }
  /**
   * (Re)arm the upload inactivity watchdog (see {@link UPLOAD_INACTIVITY_TIMEOUT_MS}).
   * Called on every FileContentsRequest, so continued pulls keep resetting it and a
   * slow-but-progressing transfer is never killed; if pulls stop for the window the
   * upload is failed (releasing uploadState). Skipped for re-pastes, matching the
   * paste-ack watchdog.
   */
  resetUploadInactivityWatchdog() {
    this.clearUploadInactivityWatchdog(), this.uploadInactivityTimeout = setTimeout(() => {
      this.uploadInactivityTimeout = void 0;
      const A = this.uploadState;
      A !== void 0 && A.isRePaste !== true && this.failPendingUpload("The remote stopped requesting the files, so the paste did not complete");
    }, _F.UPLOAD_INACTIVITY_TIMEOUT_MS);
  }
  /** Disarm the upload inactivity watchdog, if armed. */
  clearUploadInactivityWatchdog() {
    this.uploadInactivityTimeout !== void 0 && (clearTimeout(this.uploadInactivityTimeout), this.uploadInactivityTimeout = void 0);
  }
  /**
   * The remote acknowledged the paste by requesting file contents: the clobber
   * window is over, so disarm the paste-ack watchdog and resume clipboard monitoring.
   * Called on every FileContentsRequest; only the first disarms/resumes. Every request
   * also (re)arms the inactivity watchdog so a stall after pulling began is recovered.
   */
  acknowledgePaste() {
    this.pasteAckTimeout !== void 0 && (this.clearPasteAckWatchdog(), this.resumeUploadMonitoring());
    const A = this.uploadState;
    if (A !== void 0 && A.isRePaste !== true && A.expectedFileCount === 0) {
      this.finishUploadBatch(A);
      return;
    }
    this.resetUploadInactivityWatchdog();
  }
  /**
   * Abort any in-flight chunk reads for a batch and clear their timeouts. Without this, a read
   * still running when the batch is torn down keeps its FileReader and a 60s reader-timeout
   * alive. Shared by every upload teardown path (fail, supersede, dispose).
   */
  abortInFlightReads(A) {
    for (const g of A.readerTimeouts.values())
      clearTimeout(g);
    A.readerTimeouts.clear();
    for (const g of A.activeReaders.values())
      g.abort();
    A.activeReaders.clear();
  }
  /**
   * Fail the in-flight upload (reject its completion, emit an upload error, clear
   * uploadState) and resume monitoring. Used when the advertise is rejected or the
   * paste is never pulled, so `uploadState` is released instead of lingering and
   * throwing "Upload already in progress" on every later upload.
   */
  failPendingUpload(A) {
    this.clearPasteAckWatchdog(), this.clearUploadInactivityWatchdog(), this.resumeUploadMonitoring();
    const g = this.uploadState;
    if (g === void 0)
      return;
    this.abortInFlightReads(g);
    const I = { message: A, direction: "upload" };
    this.emit("error", I);
    const { reject: B } = g;
    this.uploadState = void 0, B(new Error(A));
  }
  /**
   * Tear down the current upload because a new paste is replacing it. Unlike
   * {@link failPendingUpload}, this emits no error and leaves monitoring suppressed (a new upload
   * is starting); it aborts in-flight reads, clears the state, and *resolves* the old completion
   * (a replacement, not a failure).
   */
  supersedeUpload() {
    this.clearPasteAckWatchdog(), this.clearUploadInactivityWatchdog();
    const A = this.uploadState;
    if (A === void 0)
      return;
    this.abortInFlightReads(A);
    const { resolve: g } = A;
    this.uploadState = void 0, g();
  }
  /**
   * Finalize a fully-accounted upload batch (every counted file either served in full
   * or permanently failed). Stops the inactivity watchdog so it cannot linger past
   * completion, retains the DroppedFile metadata so a re-paste from the remote can serve
   * the data again, clears `uploadState`, and resolves the completion promise.
   */
  finishUploadBatch(A) {
    this.clearUploadInactivityWatchdog(), this.retainedFiles = A.droppedFiles, this.uploadState = void 0, A.resolve();
  }
  /**
   * Show a file picker dialog and return selected files.
   *
   * Note: This must be called in response to a user gesture (e.g., button click)
   * due to browser security restrictions.
   *
   * @param options - File picker options
   * @returns Promise resolving to selected File objects
   *
   * @example
   * ```typescript
   * button.onclick = async () => {
   *   const files = await manager.showFilePicker({ multiple: true, accept: 'image/*' });
   *   await manager.uploadFiles(files);
   * };
   * ```
   */
  showFilePicker(A) {
    return new Promise((g) => {
      const I = document.createElement("input");
      I.type = "file", I.style.display = "none", (A == null ? void 0 : A.multiple) === true && (I.multiple = true), (A == null ? void 0 : A.accept) !== void 0 && A.accept.length > 0 && (I.accept = A.accept), I.addEventListener("change", () => {
        const G = Array.from(I.files || []);
        E(), window.removeEventListener("focus", D), g(G);
      });
      let B = false;
      const E = () => {
        B || (B = true, I.parentNode && document.body.removeChild(I));
      };
      I.addEventListener("cancel", () => {
        E(), window.removeEventListener("focus", D), g([]);
      });
      const D = () => {
        setTimeout(() => {
          B || (E(), g([])), window.removeEventListener("focus", D);
        }, 300);
      };
      window.addEventListener("focus", D), document.body.appendChild(I), I.click();
    });
  }
  /**
   * Extract files (and recursively traverse directories) from a drag-and-drop event.
   *
   * Uses the File and Directory Entries API (`webkitGetAsEntry`) which is
   * supported across all major browsers (Chrome, Firefox, Safari, Edge).
   * Falls back to `getAsFile()` when the Entries API is unavailable.
   *
   * Directory entries are included in the result with `isDirectory: true`
   * and a `null` file handle.  Files inside directories have their
   * {@link DroppedFile.path} set to the relative backslash-separated path.
   *
   * @param event - DragEvent from drop handler
   * @returns Promise resolving to an array of DroppedFile descriptors
   *
   * @example
   * ```typescript
   * dropZone.addEventListener('drop', async (e) => {
   *   const files = await manager.handleDrop(e);
   *   manager.uploadFiles(files);
   * });
   * ```
   */
  async handleDrop(A) {
    var _a, _b, _c;
    A.preventDefault();
    const g = [];
    if ((_a = A.dataTransfer) == null ? void 0 : _a.items) {
      const I = [], B = [];
      for (const E of A.dataTransfer.items) {
        if (E.kind !== "file") continue;
        const D = (_b = E.webkitGetAsEntry) == null ? void 0 : _b.call(E);
        if (D)
          I.push(D);
        else {
          const G = E.getAsFile();
          G && B.push(G);
        }
      }
      for (const E of I)
        await this.traverseEntry(E, void 0, g, 0);
      for (const E of B)
        g.push({
          file: E,
          name: E.name,
          size: E.size,
          lastModified: E.lastModified
        });
    } else if ((_c = A.dataTransfer) == null ? void 0 : _c.files)
      for (const I of Array.from(A.dataTransfer.files))
        g.push({
          file: I,
          name: I.name,
          size: I.size,
          lastModified: I.lastModified
        });
    return g;
  }
  /**
   * Normalize a plain `File[]` or `DroppedFile[]` into `DroppedFile[]`.
   * Allows `uploadFiles` to accept either type for backward compatibility.
   */
  static normalizeToDroppedFiles(A) {
    return A.length === 0 ? [] : "file" in A[0] ? A : A.map((I) => ({
      file: I,
      name: I.name,
      size: I.size,
      lastModified: I.lastModified
    }));
  }
  /**
   * Recursively traverse a FileSystemEntry, collecting files and directory
   * entries into `results`.
   */
  async traverseEntry(A, g, I, B) {
    if (!(I.length >= _F.MAX_DIRECTORY_ENTRIES)) {
      if (B > _F.MAX_DIRECTORY_DEPTH) {
        console.warn(
          `Skipping "${A.name}": directory depth exceeds ${_F.MAX_DIRECTORY_DEPTH}`
        );
        return;
      }
      if (A.isFile) {
        const E = await new Promise((D, G) => {
          A.file(D, G);
        });
        I.push({
          file: E,
          name: E.name,
          size: E.size,
          lastModified: E.lastModified,
          path: g
        });
      } else if (A.isDirectory) {
        const E = g !== void 0 ? `${g}\\${A.name}` : A.name;
        I.push({
          file: null,
          name: A.name,
          size: 0,
          lastModified: 0,
          path: g,
          isDirectory: true
        });
        const D = A.createReader(), G = await _F.readAllDirectoryEntries(D);
        for (const i of G)
          await this.traverseEntry(i, E, I, B + 1);
      }
    }
  }
  /**
   * Read all entries from a FileSystemDirectoryReader.  Chromium-based
   * browsers return at most 100 entries per `readEntries()` call, so we
   * must loop until an empty batch is returned.
   */
  static readAllDirectoryEntries(A) {
    return new Promise((g, I) => {
      const B = [], E = () => {
        A.readEntries((D) => {
          D.length === 0 ? g(B) : (B.push(...D), E());
        }, I);
      };
      E();
    });
  }
  /**
   * Prevent default drag-over behavior to enable drop target.
   *
   * This must be called in the dragover event handler for drag-and-drop to work.
   *
   * @param event - DragEvent from dragover handler
   *
   * @example
   * ```typescript
   * dropZone.addEventListener('dragover', (e) => manager.handleDragOver(e));
   * ```
   */
  handleDragOver(A) {
    A.preventDefault();
  }
  /**
   * Cleanup resources and unregister callbacks.
   *
   * Call this when the session is terminating or RdpFileTransferProvider is no longer needed.
   */
  dispose() {
    this.disposed = true, this.clearPasteAckWatchdog(), this.clearUploadInactivityWatchdog(), this.resumeUploadMonitoring();
    for (const A of this.activeDownloads.values())
      this.abortWriteHandle(A), A.reject(new Error("RdpFileTransferProvider disposed"));
    this.activeDownloads.clear(), this.uploadState !== void 0 && (this.abortInFlightReads(this.uploadState), this.uploadState.reject(new Error("RdpFileTransferProvider disposed"))), this.uploadState = void 0, this.retainedFiles = void 0, this.availableFiles = [], this.clipDataId = void 0, this.storageBackend && (this.storageBackend.dispose(), this.storageBackend = void 0, this.storageBackendReady = void 0), this.eventHandlers.clear();
  }
  // ==================== Callback Handlers ====================
  handleFilesAvailable(A, g) {
    this.failPendingUpload("Upload interrupted: the remote clipboard changed");
    const I = A.map((B) => ({
      ...B,
      name: _F.sanitizeFileName(B.name),
      path: B.path !== void 0 ? _F.sanitizePath(B.path) : void 0
    }));
    this.availableFiles = I, this.clipDataId = g, this.emit("files-available", I);
  }
  /**
   * Extract the basename from a file name, stripping any path traversal or
   * directory components. Returns "unnamed_file" if the name is empty or
   * consists entirely of path separators / traversal sequences.
   */
  /** @internal Visible for testing. */
  static sanitizeFileName(A) {
    const g = A.split(/[/\\]/);
    for (let I = g.length - 1; I >= 0; I--) {
      const B = g[I];
      if (B.length > 0 && B !== "." && B !== "..")
        return B;
    }
    return "unnamed_file";
  }
  /**
   * Sanitize a relative directory path by stripping traversal components
   * (`.` and `..`) and absolute path prefixes. Returns undefined if the
   * path is empty after sanitization.
   */
  /** @internal Visible for testing. */
  static sanitizePath(A) {
    const I = A.split(/[/\\]/).filter((B) => B.length > 0 && B !== "." && B !== "..");
    if (I.length > 0 && (I[0] === "?" || I[0] === ".") && (I.shift(), I.length > 0 && /^[A-Za-z]:$/.test(I[0]) && I.shift()), I.length > 0 && /^[A-Za-z]:$/.test(I[0]) && I.shift(), I.length !== 0)
      return I.join("\\");
  }
  handleFileContentsRequest(A) {
    if (this.acknowledgePaste(), !this.uploadState) {
      if (!this.retainedFiles) {
        console.warn("Received file contents request but no upload in progress"), this.sendSubmitFileContents(A.streamId, true, new Uint8Array());
        return;
      }
      this.rebuildUploadStateFromRetained();
    }
    const g = this.uploadState, { files: I, droppedFiles: B } = g;
    if (g.failedFiles.has(A.index)) {
      this.sendSubmitFileContents(A.streamId, true, new Uint8Array());
      return;
    }
    const E = I[A.index], D = B[A.index];
    if (D === void 0) {
      console.error(
        `File index ${A.index} out of range (stream ${A.streamId}, valid: 0..${B.length - 1})`
      ), this.sendSubmitFileContents(A.streamId, true, new Uint8Array());
      return;
    }
    if (E == null) {
      if ((A.flags & b.SIZE) !== 0) {
        const i = new Uint8Array(8);
        this.sendSubmitFileContents(A.streamId, false, i);
      } else
        this.sendSubmitFileContents(A.streamId, true, new Uint8Array());
      return;
    }
    const G = E;
    if ((A.flags & b.SIZE) !== 0) {
      const i = new Uint8Array(8);
      new DataView(i.buffer).setBigUint64(0, BigInt(G.size), true), this.sendSubmitFileContents(A.streamId, false, i), G.size === 0 && this.markUploadFileComplete(A.index, G, g.transferIds.get(A.index) ?? -1);
    } else if ((A.flags & b.RANGE) !== 0) {
      const i = G.slice(A.position, A.position + A.size), R = new FileReader();
      g.activeReaders.set(A.streamId, R);
      const h = setTimeout(() => {
        var _a;
        R.abort(), this.uploadState !== void 0 && (this.uploadState.activeReaders.delete(A.streamId), this.uploadState.readerTimeouts.delete(A.streamId)), this.sendSubmitFileContents(A.streamId, true, new Uint8Array());
        const U = {
          message: `File read timeout after ${_F.FILE_READER_TIMEOUT_MS / 1e3}s`,
          transferId: (_a = this.uploadState) == null ? void 0 : _a.transferIds.get(A.index),
          fileIndex: A.index,
          fileName: D.name,
          direction: "upload"
        };
        this.emit("error", U), this.uploadState !== void 0 && (this.uploadState.failedFiles.add(A.index), this.uploadState.completedFiles.add(A.index), this.uploadState.completedFiles.size >= this.uploadState.expectedFileCount && this.finishUploadBatch(this.uploadState));
      }, _F.FILE_READER_TIMEOUT_MS);
      g.readerTimeouts.set(A.streamId, h), R.onload = () => {
        if (this.uploadState !== void 0) {
          this.uploadState.activeReaders.delete(A.streamId);
          const c = this.uploadState.readerTimeouts.get(A.streamId);
          c !== void 0 && (clearTimeout(c), this.uploadState.readerTimeouts.delete(A.streamId));
        }
        const U = new Uint8Array(R.result);
        if (this.sendSubmitFileContents(A.streamId, false, U), this.uploadState !== void 0) {
          const c = (this.uploadState.bytesServed.get(A.index) ?? 0) + U.length;
          this.uploadState.bytesServed.set(A.index, c);
          const u = this.uploadState.transferIds.get(A.index) ?? -1, oA = {
            transferId: u,
            fileIndex: A.index,
            fileName: D.name,
            bytesTransferred: c,
            totalBytes: G.size,
            percentage: G.size === 0 ? 100 : Math.min(c / G.size * 100, 100)
          };
          this.emit("upload-progress", oA), c >= G.size && this.markUploadFileComplete(A.index, G, u);
        }
      }, R.onerror = () => {
        var _a, _b;
        if (this.uploadState !== void 0) {
          this.uploadState.activeReaders.delete(A.streamId);
          const c = this.uploadState.readerTimeouts.get(A.streamId);
          c !== void 0 && (clearTimeout(c), this.uploadState.readerTimeouts.delete(A.streamId));
        }
        if (((_a = this.uploadState) == null ? void 0 : _a.failedFiles.has(A.index)) === true)
          return;
        this.sendSubmitFileContents(A.streamId, true, new Uint8Array());
        const U = {
          message: "Failed to read file chunk",
          transferId: (_b = this.uploadState) == null ? void 0 : _b.transferIds.get(A.index),
          fileIndex: A.index,
          fileName: D.name,
          direction: "upload",
          cause: R.error
        };
        this.emit("error", U), this.uploadState !== void 0 && (this.uploadState.failedFiles.add(A.index), this.uploadState.completedFiles.add(A.index), this.uploadState.completedFiles.size >= this.uploadState.expectedFileCount && this.finishUploadBatch(this.uploadState));
      }, R.readAsArrayBuffer(i);
    }
  }
  /**
   * Mark a single upload file as fully served: record it, emit upload-complete once, and
   * finalize the batch once every counted file is accounted for. Shared by the RANGE onload
   * path (final chunk served) and the SIZE path for 0-byte files (which get no RANGE request).
   */
  markUploadFileComplete(A, g, I) {
    const B = this.uploadState;
    B === void 0 || B.completedFiles.has(A) || (B.completedFiles.add(A), this.emit("upload-complete", g, A, I), B.completedFiles.size === B.expectedFileCount && this.finishUploadBatch(B));
  }
  /**
   * Lazily rebuild uploadState from retainedFiles when the remote re-pastes
   * after the original upload completed. This lets the main code path in
   * handleFileContentsRequest handle progress and completion identically.
   * No external promise - resolve/reject are no-ops.
   */
  rebuildUploadStateFromRetained() {
    const A = this.retainedFiles, g = /* @__PURE__ */ new Map();
    for (let B = 0; B < A.length; B++)
      g.set(B, this.generateStreamId());
    const I = A.filter((B) => B.isDirectory !== true).length;
    this.uploadState = {
      files: A.map((B) => B.file),
      droppedFiles: A,
      failedFiles: /* @__PURE__ */ new Set(),
      expectedFileCount: I,
      completedFiles: /* @__PURE__ */ new Set(),
      bytesServed: /* @__PURE__ */ new Map(),
      activeReaders: /* @__PURE__ */ new Map(),
      readerTimeouts: /* @__PURE__ */ new Map(),
      transferIds: g,
      resolve: () => {
      },
      reject: () => {
      },
      isRePaste: true
    }, this.emit("upload-batch-started", g, A);
  }
  handleFileContentsResponse(A) {
    const g = this.activeDownloads.get(A.streamId);
    if (!g) {
      console.warn(`Received response for unknown stream ${A.streamId}`);
      return;
    }
    if (A.isError) {
      this.activeDownloads.delete(A.streamId), this.abortWriteHandle(g);
      const I = {
        message: "Remote failed to provide file contents",
        transferId: g.streamId,
        fileIndex: g.fileIndex,
        fileName: g.fileInfo.name,
        direction: "download"
      };
      this.emit("error", I), g.reject(new Error(I.message));
      return;
    }
    if (g.expectedSize === void 0) {
      if (A.data.length < 8) {
        this.activeDownloads.delete(A.streamId), this.abortWriteHandle(g);
        const E = {
          message: "Invalid SIZE response: expected 8 bytes for file size",
          transferId: g.streamId,
          fileIndex: g.fileIndex,
          fileName: g.fileInfo.name,
          direction: "download"
        };
        this.emit("error", E), g.reject(new Error(E.message));
        return;
      }
      const I = new DataView(A.data.buffer, A.data.byteOffset, A.data.byteLength), B = Number(I.getBigUint64(0, true));
      if (B > _F.MAX_FILE_SIZE) {
        this.activeDownloads.delete(A.streamId), this.abortWriteHandle(g);
        const E = {
          message: `File size ${(B / (1024 * 1024 * 1024)).toFixed(2)}GB exceeds maximum download limit of 2GB`,
          transferId: g.streamId,
          fileIndex: g.fileIndex,
          fileName: g.fileInfo.name,
          direction: "download"
        };
        this.emit("error", E), g.reject(new Error(E.message));
        return;
      }
      if (g.expectedSize = B, B === 0) {
        this.activeDownloads.delete(A.streamId), this.abortWriteHandle(g);
        const E = new Blob([]);
        this.emit("download-complete", g.fileInfo, E, g.fileIndex, g.streamId), g.resolve(E);
        return;
      }
      this.initWriteHandleAndRequestFirstChunk(g);
    } else
      this.handleDataChunk(g, A.data);
  }
  /**
   * Create a write handle for the given transfer and request the first
   * data chunk.  Runs asynchronously because backend initialization
   * (especially OPFS) is async.
   *
   * The init promise is stored on `state.writeHandleReady` so that
   * DATA responses arriving before the handle is ready can await it.
   */
  initWriteHandleAndRequestFirstChunk(A) {
    A.writeHandleReady = (async () => {
      try {
        const g = (async () => (await this.ensureStorageBackend()).createWriteHandle(A.fileInfo.name, A.expectedSize ?? 0))(), I = _F.WRITE_HANDLE_INIT_TIMEOUT_MS, B = await Promise.race([
          g,
          new Promise(
            (E, D) => setTimeout(() => D(new Error(`Storage init timed out after ${I / 1e3}s`)), I)
          )
        ]);
        if (this.disposed || !this.activeDownloads.has(A.streamId)) {
          try {
            await B.abort();
          } catch {
          }
          return;
        }
        A.writeHandle = B;
      } catch (g) {
        this.activeDownloads.delete(A.streamId);
        const I = {
          message: "Failed to initialize storage for download",
          transferId: A.streamId,
          fileIndex: A.fileIndex,
          fileName: A.fileInfo.name,
          direction: "download",
          cause: g
        };
        this.emit("error", I), A.reject(new Error(I.message, { cause: g }));
        return;
      }
      this.requestNextChunk(A);
    })();
  }
  /**
   * Write a data chunk to the storage backend and advance the download.
   *
   * If the write handle is not yet ready (DATA response arrived before
   * backend init completed), this method awaits `state.writeHandleReady`
   * to preserve chunk ordering -- each concurrent caller awaits the same
   * promise in sequence.
   */
  async handleDataChunk(A, g) {
    if (!A.writeHandle && (!A.writeHandleReady || this.disposed || !this.activeDownloads.has(A.streamId) || (await A.writeHandleReady, this.disposed || !this.activeDownloads.has(A.streamId))))
      return;
    const I = A.writeHandle;
    if (I) {
      try {
        await I.write(g);
      } catch (B) {
        this.activeDownloads.delete(A.streamId), this.abortWriteHandle(A);
        const G = {
          message: B instanceof DOMException && B.name === "QuotaExceededError" ? `Storage quota exceeded while downloading "${A.fileInfo.name}"` : "Failed to write download chunk to storage",
          transferId: A.streamId,
          fileIndex: A.fileIndex,
          fileName: A.fileInfo.name,
          direction: "download",
          cause: B
        };
        this.emit("error", G), A.reject(new Error(G.message, { cause: B }));
        return;
      }
      if (A.bytesReceived += g.length, A.expectedSize !== void 0 && A.bytesReceived > A.expectedSize * 2) {
        this.activeDownloads.delete(A.streamId), this.abortWriteHandle(A);
        const B = {
          message: `Received ${A.bytesReceived} bytes but expected ${A.expectedSize} - aborting`,
          transferId: A.streamId,
          fileIndex: A.fileIndex,
          fileName: A.fileInfo.name,
          direction: "download"
        };
        this.emit("error", B), A.reject(new Error(B.message));
        return;
      }
      if (A.expectedSize !== void 0) {
        const B = {
          transferId: A.streamId,
          fileIndex: A.fileIndex,
          fileName: A.fileInfo.name,
          bytesTransferred: A.bytesReceived,
          totalBytes: A.expectedSize,
          percentage: Math.min(A.bytesReceived / A.expectedSize * 100, 100)
        };
        this.emit("download-progress", B);
      }
      if (A.expectedSize !== void 0 && A.bytesReceived >= A.expectedSize) {
        this.activeDownloads.delete(A.streamId);
        try {
          const B = await I.finalize();
          this.emit("download-complete", A.fileInfo, B, A.fileIndex, A.streamId), A.resolve(B);
        } catch (B) {
          this.abortWriteHandle(A);
          const E = {
            message: "Failed to finalize downloaded file",
            transferId: A.streamId,
            fileIndex: A.fileIndex,
            fileName: A.fileInfo.name,
            direction: "download",
            cause: B
          };
          this.emit("error", E), A.reject(new Error(E.message, { cause: B }));
        }
      } else
        this.requestNextChunk(A);
    }
  }
  /** Abort and clean up a write handle, ignoring errors. */
  async abortWriteHandle(A) {
    if (A.writeHandle) {
      try {
        await A.writeHandle.abort();
      } catch {
      }
      A.writeHandle = void 0;
    }
  }
  /**
   * Handle the remote's response to one of our outbound Format Lists, surfaced via
   * `on_format_list_response`. Always re-emitted as a `format-list-response` event so
   * a frontend can drive paste from it (inject on accept, retry on reject).
   *
   * On reject (`ok === false`) the remote silently discards the advertised clipboard
   * (MS-RDPECLIP), so an upload still waiting to be pulled can never complete --
   * previously this left `uploadState` set and every later upload threw "Upload
   * already in progress". The watchdog-armed check scopes the release to an upload
   * that was advertised but not yet pulled, so re-paste and an already-progressing
   * transfer are left alone.
   */
  handleFormatListResponse(A) {
    this.emit("format-list-response", A), !A && this.pasteAckTimeout !== void 0 && this.failPendingUpload("The remote rejected the file list, so the paste was not accepted");
  }
  handleLock(A) {
  }
  handleUnlock(A) {
  }
  handleLocksExpired(A) {
    const g = /* @__PURE__ */ new Set();
    for (let I = 0; I < A.length; I++)
      g.add(A[I]);
    for (const [I, B] of this.activeDownloads)
      if (B.clipDataId !== void 0 && g.has(B.clipDataId)) {
        this.activeDownloads.delete(I), this.abortWriteHandle(B);
        const E = `File download timed out for "${B.fileInfo.name}". Clipboard lock expired due to inactivity. This can happen with slow network connections or large files. Try downloading smaller files, increasing chunk size, or checking your network connection.`;
        B.reject(new Error(E)), this.emit("error", {
          message: E,
          transferId: B.streamId,
          fileIndex: B.fileIndex,
          fileName: B.fileInfo.name,
          direction: "download"
        });
      }
  }
  // ==================== Helper Methods ====================
  requestNextChunk(A) {
    if (A.expectedSize === void 0 || A.expectedSize === 0)
      return;
    const g = A.bytesReceived, I = A.expectedSize - g, B = Math.min(this.chunkSize, I);
    try {
      this.sendRequestFileContents(
        A.streamId,
        A.fileIndex,
        b.RANGE,
        g,
        B,
        A.clipDataId
      );
    } catch (E) {
      this.activeDownloads.delete(A.streamId), this.abortWriteHandle(A);
      const D = {
        message: "Failed to request file chunk",
        transferId: A.streamId,
        fileIndex: A.fileIndex,
        fileName: A.fileInfo.name,
        direction: "download",
        cause: E
      };
      this.emit("error", D), A.reject(new Error(D.message, { cause: E }));
    }
  }
  generateStreamId() {
    const A = this.activeDownloads.size + 2;
    for (let g = 0; g < A; g++) {
      const I = this.nextStreamId;
      if (this.nextStreamId = (this.nextStreamId + 1) % 4294967296, this.nextStreamId === 0 && (this.nextStreamId = 1), !this.activeDownloads.has(I) && I !== 0)
        return I;
    }
    throw new Error("Unable to generate unique stream ID");
  }
};
/** Maximum file size for downloads (2GB) to prevent browser out-of-memory errors */
__publicField(_F, "MAX_FILE_SIZE", 2 * 1024 * 1024 * 1024);
/** Timeout for FileReader operations (60 seconds) to prevent stalled uploads */
__publicField(_F, "FILE_READER_TIMEOUT_MS", 60 * 1e3);
/**
 * How long to keep clipboard monitoring suppressed after advertising an upload
 * while waiting for the remote to pull the files (first FileContentsRequest),
 * before giving up. Matches the Rust cliprdr lock inactivity timeout (60s), so
 * the JS side gives up exactly when the protocol lock does. If the remote never
 * requests contents (the paste landed in a non-file target, or the advertise was
 * clobbered), the watchdog resumes monitoring and fails the upload so its state
 * cannot wedge later uploads.
 */
__publicField(_F, "PASTE_ACK_TIMEOUT_MS", 60 * 1e3);
/**
 * Upload inactivity window. After the remote starts pulling, each FileContentsRequest
 * resets this; if pulls then stop for this long -- e.g. the remote grabbed the
 * clipboard with a text/image copy that never reaches handleFilesAvailable -- the
 * upload is failed so `uploadState` is released and later uploads aren't wedged. A
 * slow-but-progressing transfer keeps resetting it, so it is never killed.
 */
__publicField(_F, "UPLOAD_INACTIVITY_TIMEOUT_MS", 60 * 1e3);
/** Timeout for storage backend write handle initialization (30 seconds). */
__publicField(_F, "WRITE_HANDLE_INIT_TIMEOUT_MS", 30 * 1e3);
/** Maximum recursion depth when traversing dropped directories. */
__publicField(_F, "MAX_DIRECTORY_DEPTH", 32);
/** Maximum total entries (files + directories) collected from a single drop. */
__publicField(_F, "MAX_DIRECTORY_ENTRIES", 1e3);
let F = _F;
async function Ag(Q) {
  await lA(), YA(Q);
}
const gg = {
  DesktopSize: S,
  InputTransaction: z,
  SessionBuilder: M,
  ClipboardData: s,
  DeviceEvent: J
};
function Ig(Q) {
  return new w("pcb", Q);
}
function Bg(Q, A = "enhanced") {
  if (Q.trim() === "")
    throw new Error("vmconnect requires a VM ID");
  const g = A === "enhanced" ? `${Q};EnhancedMode=1` : Q;
  return new w("vmconnect", g);
}
function Qg(Q) {
  return new w("display_control", Q);
}
function Cg(Q) {
  return new w("audio_playback", Q);
}
function Eg(Q) {
  return new w("camera", Q);
}
function Dg(Q) {
  return new w("camera_h264", Q);
}
function Gg(Q) {
  return new w("camera_name", Q);
}
function ig(Q) {
  return new w("camera_sample", Q);
}
function wg() {
  return new w("camera_wanted", null);
}
function og(Q) {
  return new w("kdc_proxy_url", Q);
}
function Ng(Q) {
  return new w("outbound_message_size_limit", Q);
}
function Yg(Q) {
  return new w("enable_credssp", Q);
}
function kg(Q) {
  return new w("enable_standard_rdp_security", Q);
}
function Mg(Q) {
  return new w("enable_server_pointer", Q);
}
function yg(Q) {
  return new w("legacy_graphics", Q);
}
function Rg(Q) {
  return new w("performance_flags", Q);
}
function Fg(Q) {
  return new w("desktop_scale_factor", Q);
}
function Jg(Q) {
  return new w("color_depth", Q);
}
function ag(Q) {
  return new w(
    "monitor_layout",
    Q.flatMap((A) => [A.left, A.top, A.right, A.bottom, A.primary ? 1 : 0])
  );
}
function Ug(Q) {
  const A = (g) => {
    const I = [];
    for (let B = 0; B + 4 < g.length; B += 5)
      I.push({
        left: g[B],
        top: g[B + 1],
        right: g[B + 2],
        bottom: g[B + 3],
        primary: (g[B + 4] & 1) !== 0
      });
    Q(I);
  };
  return new w("monitor_layout_callback", A);
}
function cg(Q) {
  return new w("client_name", Q);
}
function hg(Q) {
  return new w("keyboard_layout", Q);
}
export {
  gg as Backend,
  wA as BlobStorageBackend,
  b as FileContentsFlags,
  d as OpfsStorageBackend,
  rA as PrinterDriverName,
  vA as RDCleanPathErrorCode,
  _ as RdpFile,
  F as RdpFileTransferProvider,
  Cg as audioPlayback,
  Eg as camera,
  Dg as cameraH264,
  Gg as cameraName,
  ig as cameraSample,
  wg as cameraWanted,
  cg as clientName,
  Jg as colorDepth,
  Fg as desktopScaleFactor,
  fA as detectStorageBackend,
  Qg as displayControl,
  Yg as enableCredssp,
  Mg as enableServerPointer,
  kg as enableStandardRdpSecurity,
  xA as fileContentsRequestCallback,
  WA as fileContentsResponseCallback,
  bA as filesAvailableCallback,
  Ag as init,
  OA as initiateFileCopy,
  og as kdcProxyUrl,
  hg as keyboardLayout,
  yg as legacyGraphics,
  mA as lockCallback,
  VA as locksExpiredCallback,
  ag as monitorLayout,
  Ug as monitorLayoutCallback,
  Ng as outboundMessageSizeLimit,
  Rg as performanceFlags,
  Ig as preConnectionBlob,
  eA as printJobStreamCallbacks,
  _A as printerDeviceId,
  $A as printerDriverName,
  uA as printerName,
  tA as requestFileContents,
  PA as submitFileContents,
  pA as unlockCallback,
  Bg as vmConnect
};
