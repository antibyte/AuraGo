/**
 * Fluent XCAF document builder for assemblies with colors and names.
 *
 * @example
 * ```ts
 * const doc = XCAFDocument.create(rawKernel);
 * const root = doc.addShape(box, { name: 'housing', color: [0.8, 0.2, 0.1] });
 * doc.addChild(root, gear, {
 *   name: 'gear-1',
 *   location: { tx: 10, ty: 0, tz: 5 },
 *   color: [0.5, 0.5, 0.5],
 * });
 * const step = doc.exportSTEP();
 * const glb = doc.exportGLTF(Module);
 * doc.close();
 * ```
 *
 * Prefer `kernel.createXCAFDocument()` over the static factories. Constructing
 * from a bare raw kernel works, but only a live {@link OcctKernel} registers the
 * module's exception decoder — without one, a failure here reports the
 * undecoded `[object WebAssembly.Exception]` and downgrades to `KERNEL_ERROR`.
 */
import { OcctError, OcctErrorCode, wrap } from "./types.js";
function tag(n) {
    return n;
}
export class XCAFDocument {
    #raw;
    #docId;
    #fs;
    #closed = false;
    constructor(raw, docId, fs) {
        this.#raw = raw;
        this.#docId = docId;
        this.#fs = fs;
    }
    /** Create a new empty XCAF document. */
    static create(raw, fs) {
        const docId = wrap("xcafNewDocument", () => raw.xcafNewDocument());
        return new XCAFDocument(raw, docId, fs);
    }
    /** Import a STEP file into a new XCAF document (preserves colors/names/assemblies). */
    static fromSTEP(raw, stepData, fs) {
        const docId = wrap("xcafImportSTEP", () => raw.xcafImportSTEP(stepData));
        return new XCAFDocument(raw, docId, fs);
    }
    /**
     * Add a shape as a root label.
     *
     * By default the shape becomes a single part, even when it is a compound.
     * With `{ assembly: true }` a compound becomes an assembly instead: one
     * component per top-level child, each a placed reference to a prototype
     * label holding that child's geometry. This is the structure STEP import
     * produces, and the only one `exportSTEP` writes as an assembly. Throws if
     * `assembly` is set and the shape is not a compound.
     */
    addShape(shape, options) {
        this.#ensureOpen();
        const t = options?.assembly
            ? wrap("xcafAddAssembly", () => this.#raw.xcafAddAssembly(this.#docId, shape))
            : wrap("xcafAddShape", () => this.#raw.xcafAddShape(this.#docId, shape));
        this.#applyOptions(t, options);
        return tag(t);
    }
    /**
     * Add a shape as a child component of a parent label.
     *
     * `parent` may be an assembly or a part. A part becomes an assembly on
     * its first child: the geometry it held moves into a first component at
     * identity, carrying the part's name and color, so `getChildren(parent)`
     * then lists that component ahead of the new one and exports keep both.
     * A component label cannot take children; resolve it with
     * {@link getReferredLabel} first.
     */
    addChild(parent, shape, options) {
        this.#ensureOpen();
        const loc = options?.location ?? {};
        const t = wrap("xcafAddComponent", () => this.#raw.xcafAddComponent(this.#docId, parent, shape, loc.tx ?? 0, loc.ty ?? 0, loc.tz ?? 0, loc.rx ?? 0, loc.ry ?? 0, loc.rz ?? 0));
        this.#applyOptions(t, options);
        return tag(t);
    }
    /** Set color on an existing label. */
    setColor(label, color) {
        this.#ensureOpen();
        const [r, g, b] = color;
        wrap("xcafSetColor", () => this.#raw.xcafSetColor(this.#docId, label, r, g, b));
    }
    /** Set name on an existing label. */
    setName(label, name) {
        this.#ensureOpen();
        wrap("xcafSetName", () => this.#raw.xcafSetName(this.#docId, label, name));
    }
    /**
     * Get info about a label.
     * If `shapeHandle` is non-null, the caller owns it and must release it.
     */
    getLabelInfo(label) {
        this.#ensureOpen();
        const raw = wrap("xcafGetLabelInfo", () => this.#raw.xcafGetLabelInfo(this.#docId, label));
        return {
            labelId: raw.labelId,
            name: raw.name,
            hasColor: raw.hasColor,
            color: [raw.r, raw.g, raw.b],
            isAssembly: raw.isAssembly,
            isComponent: raw.isComponent,
            shapeHandle: raw.shapeId > 0 ? raw.shapeId : null,
        };
    }
    /** Get child label tags of a parent. */
    getChildren(parent) {
        this.#ensureOpen();
        return this.#vecToTags(wrap("xcafGetChildLabels", () => this.#raw.xcafGetChildLabels(this.#docId, parent)));
    }
    /** Get root (free) shape label tags. */
    getRoots() {
        this.#ensureOpen();
        return this.#vecToTags(wrap("xcafGetRootLabels", () => this.#raw.xcafGetRootLabels(this.#docId)));
    }
    /**
     * Resolve a component label to the label it instantiates.
     *
     * In an XCAF assembly a component (`isComponent`) is a placed reference:
     * it carries a location and points at a prototype label, the part or
     * sub-assembly that owns the geometry, the sub-shapes and the children.
     * `getChildren` on the component itself is therefore empty; walk into the
     * referred label instead. Returns `null` when `label` is not a reference.
     */
    getReferredLabel(label) {
        this.#ensureOpen();
        const t = wrap("xcafGetReferredLabel", () => this.#raw.xcafGetReferredLabel(this.#docId, label));
        return t > 0 ? tag(t) : null;
    }
    /**
     * The placement of a label relative to its parent, as a 3x4 row-major
     * affine matrix (`[r00,r01,r02,tx, r10,r11,r12,ty, r20,r21,r22,tz]`), the
     * layout `OcctKernel.transform` and `located` accept. Identity for labels
     * that are not components. Compose these down the tree to place a
     * prototype's geometry once per instance.
     */
    getLocation(label) {
        this.#ensureOpen();
        return this.#vecToNumbers(wrap("xcafGetLabelLocation", () => this.#raw.xcafGetLabelLocation(this.#docId, label)));
    }
    /**
     * Get the named sub-shape labels of a part (faces, edges or solids that
     * carry their own name or color). Pass the prototype label, not a
     * component: see {@link getReferredLabel}.
     */
    getSubShapes(label) {
        this.#ensureOpen();
        return this.#vecToTags(wrap("xcafGetSubShapeLabels", () => this.#raw.xcafGetSubShapeLabels(this.#docId, label)));
    }
    /**
     * Register a sub-shape of a part so it can carry its own name or color.
     * `label` must be a top-level part (one added with `addShape`, or the
     * prototype behind a component) and `shape` one of its sub-shapes, for
     * example a face from `kernel.getSubShapes`. Throws otherwise.
     */
    addSubShape(label, shape, options) {
        this.#ensureOpen();
        const t = wrap("xcafAddSubShape", () => this.#raw.xcafAddSubShape(this.#docId, label, shape));
        this.#applyOptions(t, options);
        return tag(t);
    }
    /** Export as STEP with colors and names preserved. */
    exportSTEP() {
        this.#ensureOpen();
        return wrap("xcafExportSTEP", () => this.#raw.xcafExportSTEP(this.#docId));
    }
    exportGLTF(fsOrOptions, maybeOptions) {
        this.#ensureOpen();
        // Resolve overloads: exportGLTF(fs, opts) vs exportGLTF(opts)
        let fs;
        let options;
        if (fsOrOptions && typeof fsOrOptions === "object" && "readFile" in fsOrOptions && "unlink" in fsOrOptions) {
            // Legacy: exportGLTF(fs, options?)
            fs = fsOrOptions;
            options = maybeOptions;
        }
        else {
            // New: exportGLTF(options?)
            const opts = fsOrOptions;
            fs = opts?.fs;
            options = opts;
        }
        fs ??= this.#fs;
        if (!fs) {
            throw new OcctError("xcafExportGLTF", "No Emscripten FS available. Either create the document via OcctKernel.createXCAFDocument(), or pass { fs } in options.");
        }
        const linDefl = options?.linearDeflection ?? 0.1;
        const angDefl = options?.angularDeflection ?? 0.5;
        const glbPath = wrap("xcafExportGLTF", () => this.#raw.xcafExportGLTF(this.#docId, linDefl, angDefl));
        const data = fs.readFile(glbPath);
        fs.unlink(glbPath);
        return data;
    }
    /** Close the document and free OCCT resources. */
    close() {
        if (this.#closed)
            return;
        // Mark closed before the call so a failing close isn't retried by
        // Symbol.dispose, which would throw again during stack unwinding.
        this.#closed = true;
        wrap("xcafClose", () => this.#raw.xcafClose(this.#docId));
    }
    [Symbol.dispose]() {
        this.close();
    }
    #applyOptions(labelId, options) {
        if (options?.name) {
            wrap("xcafSetName", () => this.#raw.xcafSetName(this.#docId, labelId, options.name));
        }
        if (options?.color) {
            const [r, g, b] = options.color;
            wrap("xcafSetColor", () => this.#raw.xcafSetColor(this.#docId, labelId, r, g, b));
        }
    }
    #vecToTags(vec) {
        return this.#vecToNumbers(vec).map(tag);
    }
    #vecToNumbers(vec) {
        try {
            const result = [];
            for (let i = 0; i < vec.size(); i++) {
                result.push(vec.get(i));
            }
            return result;
        }
        finally {
            vec.delete();
        }
    }
    #ensureOpen() {
        if (this.#closed) {
            throw new OcctError("XCAFDocument", "Document is closed", OcctErrorCode.DocumentClosed);
        }
    }
}
//# sourceMappingURL=xcaf-document.js.map