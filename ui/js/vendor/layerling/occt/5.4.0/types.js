/**
 * TopAbs_ShapeEnum values returned by getShapeType, in TopAbs ordinal order
 * (queryBatch indexes this array by the raw enum value). Single source of truth
 * for both the {@link ShapeType} union and runtime validation.
 */
export const SHAPE_TYPES = [
    "compound", "compsolid", "solid", "shell", "face", "wire", "edge", "vertex", "shape",
];
/** TopAbs_Orientation values returned by shapeOrientation. */
export const SHAPE_ORIENTATIONS = ["forward", "reversed", "internal", "external"];
/** BRepClass_FaceClassifier results for a UV point relative to a face boundary. */
export const POINT_CLASSIFICATIONS = ["in", "on", "out"];
/** Transition mode for sweep operations (BRepBuilderAPI_MakeSweep). */
export var TransitionMode;
(function (TransitionMode) {
    /** Transform the profile along the spine (default). */
    TransitionMode[TransitionMode["Transformed"] = 0] = "Transformed";
    /** Apply right-corner transitions at spine vertices. */
    TransitionMode[TransitionMode["RightCorner"] = 1] = "RightCorner";
    /** Apply round-corner transitions at spine vertices. */
    TransitionMode[TransitionMode["RoundCorner"] = 2] = "RoundCorner";
})(TransitionMode || (TransitionMode = {}));
/** Profile-orientation mode for {@link OcctKernel.sweepOriented}. */
export var SweepMode;
(function (SweepMode) {
    /** Minimal-torsion parallel transport — profile does not rotate (corrected Frenet). */
    SweepMode[SweepMode["Fixed"] = 0] = "Fixed";
    /** Profile follows the spine's principal normal (Frenet trihedron). */
    SweepMode[SweepMode["Frenet"] = 1] = "Frenet";
    /** Profile keeps a caller-supplied up/binormal direction constant. */
    SweepMode[SweepMode["FixedUp"] = 2] = "FixedUp";
    /** Orientation driven by an auxiliary guide spine (requires `auxSpine`). */
    SweepMode[SweepMode["Auxiliary"] = 3] = "Auxiliary";
})(SweepMode || (SweepMode = {}));
/** How the swept section relates to the guide spine (BRepFill_TypeOfContact). */
export var SweepContact;
(function (SweepContact) {
    /** The guide only orients the section; it does not touch the result. */
    SweepContact[SweepContact["None"] = 0] = "None";
    /** The section is widened to stay in contact with the guide; section width stays constant. */
    SweepContact[SweepContact["Contact"] = 1] = "Contact";
    /** The guide becomes a boundary of the swept surface; section width varies. */
    SweepContact[SweepContact["ContactOnBorder"] = 2] = "ContactOnBorder";
})(SweepContact || (SweepContact = {}));
/** Homothetic scaling law applied along the spine by {@link OcctKernel.sweepFull}. */
export var SweepLaw;
(function (SweepLaw) {
    /** No scaling; the section keeps its authored size. */
    SweepLaw[SweepLaw["None"] = 0] = "None";
    /** Linear interpolation from 1 to `endFactor` (OCCT `Law_Linear`). */
    SweepLaw[SweepLaw["Linear"] = 1] = "Linear";
    /** S-curve with zero end derivatives, 1 to `endFactor` (OCCT `Law_S`). */
    SweepLaw[SweepLaw["SCurve"] = 2] = "SCurve";
})(SweepLaw || (SweepLaw = {}));
/** How offset faces or edges meet at a joint: `shell`, `offset`, `offsetWire2D`. */
export var JoinType;
(function (JoinType) {
    /** Arc interpolation at joints (default). */
    JoinType[JoinType["Arc"] = 0] = "Arc";
    /** Tangent extension at joints. */
    JoinType[JoinType["Tangent"] = 1] = "Tangent";
    /** Intersection extension at joints. */
    JoinType[JoinType["Intersection"] = 2] = "Intersection";
})(JoinType || (JoinType = {}));
/** Boolean operation code for booleanPipeline. */
export var BooleanOp;
(function (BooleanOp) {
    /** Union: combine volumes. */
    BooleanOp[BooleanOp["Fuse"] = 0] = "Fuse";
    /** Subtraction: remove tool from base. */
    BooleanOp[BooleanOp["Cut"] = 1] = "Cut";
    /** Intersection: keep only overlapping volume. */
    BooleanOp[BooleanOp["Common"] = 2] = "Common";
})(BooleanOp || (BooleanOp = {}));
/**
 * Structured error codes for programmatic error handling.
 * Use `switch (error.code)` instead of parsing error message strings.
 */
export var OcctErrorCode;
(function (OcctErrorCode) {
    /** Shape construction failed (Build()/IsDone() returned false). */
    OcctErrorCode["ConstructionFailed"] = "CONSTRUCTION_FAILED";
    /** Boolean operation failed (fuse/cut/common/intersect/section). */
    OcctErrorCode["BooleanFailed"] = "BOOLEAN_FAILED";
    /** Referenced shape ID does not exist in the arena. */
    OcctErrorCode["InvalidShapeId"] = "INVALID_SHAPE_ID";
    /** Referenced XCAF label ID does not exist. */
    OcctErrorCode["InvalidLabelId"] = "INVALID_LABEL_ID";
    /** Tessellation or meshing operation failed. */
    OcctErrorCode["TessellationFailed"] = "TESSELLATION_FAILED";
    /** STEP/STL/BREP import or export failed. */
    OcctErrorCode["ImportExportFailed"] = "IMPORT_EXPORT_FAILED";
    /** Shape healing or repair operation failed. */
    OcctErrorCode["HealingFailed"] = "HEALING_FAILED";
    /** Operation attempted on a closed XCAF document. */
    OcctErrorCode["DocumentClosed"] = "DOCUMENT_CLOSED";
    /** OCCT kernel raised an internal error (Standard_Failure). */
    OcctErrorCode["KernelError"] = "KERNEL_ERROR";
    /** Error does not match any known pattern. */
    OcctErrorCode["Unknown"] = "UNKNOWN";
})(OcctErrorCode || (OcctErrorCode = {}));
/**
 * Typed error thrown when an OCCT operation fails.
 * The `operation` field identifies which kernel method raised the error.
 * The `code` field enables programmatic error handling via `switch`.
 *
 * @example
 * ```ts
 * try {
 *   kernel.fuse(a, b);
 * } catch (e) {
 *   if (e instanceof OcctError) {
 *     switch (e.code) {
 *       case OcctErrorCode.BooleanFailed:
 *         // retry with simpler geometry
 *         break;
 *       case OcctErrorCode.InvalidShapeId:
 *         // shape was already released
 *         break;
 *     }
 *   }
 * }
 * ```
 */
export class OcctError extends Error {
    /** Name of the kernel method that failed. */
    operation;
    /** Structured error code for programmatic handling. */
    code;
    constructor(operation, message, code) {
        super(`${operation}: ${message}`);
        this.name = "OcctError";
        this.operation = operation;
        this.code = code ?? classifyError(operation, message);
    }
}
/** Operation categories used to infer error codes from context. */
const BOOLEAN_OPS = new Set(["fuse", "cut", "common", "intersect", "section", "sectionPlane", "fuseAll", "cutAll", "split", "booleanPipeline", "fuseWithHistory", "cutWithHistory", "intersectWithHistory"]);
const TESSELLATION_OPS = new Set(["tessellate", "wireframe", "meshShape", "meshBatch"]);
const IO_OPS = new Set(["importStep", "exportStep", "importStl", "exportStl", "toBREP", "fromBREP", "xcafExportSTEP", "xcafImportSTEP", "xcafExportGLTF"]);
const HEALING_OPS = new Set(["fixShape", "unifySameDomain", "healSolid", "healFace", "healWire", "fixFaceOrientations", "removeDegenerateEdges", "fixWireOnFace", "buildCurves3d"]);
/**
 * Classify an error into a structured code by matching known C++ error patterns
 * and operation context.
 */
function classifyError(operation, message) {
    const msg = message.toLowerCase();
    // Exact pattern matches from C++ facade
    if (msg.includes("invalid shape id"))
        return OcctErrorCode.InvalidShapeId;
    if (msg.includes("invalid label id"))
        return OcctErrorCode.InvalidLabelId;
    if (msg.includes("document is closed"))
        return OcctErrorCode.DocumentClosed;
    if (msg.includes("boolean operation failed"))
        return OcctErrorCode.BooleanFailed;
    if (msg.includes("construction failed"))
        return OcctErrorCode.ConstructionFailed;
    // sweepOriented reports MakePipeShell's status instead of the generic
    // "operation failed", but it is still a construction failure.
    if (msg.includes("does not intersect the guide wire"))
        return OcctErrorCode.ConstructionFailed;
    if (msg.includes("in contact with the guide wire"))
        return OcctErrorCode.ConstructionFailed;
    // Operation-category fallback
    if (BOOLEAN_OPS.has(operation))
        return OcctErrorCode.BooleanFailed;
    if (TESSELLATION_OPS.has(operation))
        return OcctErrorCode.TessellationFailed;
    if (IO_OPS.has(operation))
        return OcctErrorCode.ImportExportFailed;
    if (HEALING_OPS.has(operation))
        return OcctErrorCode.HealingFailed;
    // "operation failed" is the generic SetupShape/FilletLike pattern
    if (msg.includes("operation failed"))
        return OcctErrorCode.ConstructionFailed;
    // Unmatched errors from known OCCT operations are Standard_Failure propagations
    if (operation && operation !== "XCAFDocument")
        return OcctErrorCode.KernelError;
    return OcctErrorCode.Unknown;
}
/**
 * Every live kernel's decoder. Decoding a `WebAssembly.Exception` needs the
 * throwing Emscripten module's memory and exception tag, and kernels over
 * separate modules can coexist — so decoders accumulate rather than replace,
 * and each is tried in turn. A foreign module can't return a wrong message:
 * `WebAssembly.Exception.getArg` rejects a tag it doesn't own.
 */
const exceptionDecoders = new Set();
/**
 * Register a module-backed decoder used to recover C++ `what()` strings from
 * thrown `WebAssembly.Exception` objects. Returns a function that unregisters
 * it, which the kernel calls on disposal so the module isn't retained.
 */
export function addExceptionDecoder(decoder) {
    exceptionDecoders.add(decoder);
    return () => {
        exceptionDecoders.delete(decoder);
    };
}
/**
 * Best-effort message extraction for a thrown value. Under `-fwasm-exceptions`
 * a C++ throw crosses the Embind boundary as a `WebAssembly.Exception` rather
 * than an `Error`, and stringifies to a useless `[object WebAssembly.Exception]`.
 */
function messageOf(e) {
    if (e instanceof Error)
        return e.message;
    for (const decoder of exceptionDecoders) {
        try {
            const decoded = decoder(e);
            if (decoded?.[1])
                return decoded[1];
        }
        catch {
            // Wrong module, or no helper — decoding is diagnostics-only, so
            // exhausting every decoder just falls back to stringification.
        }
    }
    return String(e);
}
/**
 * Run `fn`, re-throwing any failure as an {@link OcctError} tagged with the
 * given operation name. Shared by the kernel and the XCAF document so error
 * classification stays in one place.
 */
export function wrap(operation, fn) {
    try {
        return fn();
    }
    catch (e) {
        if (e instanceof OcctError) {
            // Already classified by an inner wrapped call. Preserve the original
            // (most-specific) code and just retag the operation, so re-wrapping
            // a passthrough like cacheStep/loadCached doesn't reclassify e.g.
            // ImportExportFailed down to KernelError.
            throw new OcctError(operation, e.message, e.code);
        }
        // The C++ facade already prefixes its throws with the method name, and
        // OcctError prepends it again — drop the duplicate.
        const message = messageOf(e);
        const prefix = `${operation}: `;
        throw new OcctError(operation, message.startsWith(prefix) ? message.slice(prefix.length) : message);
    }
}
//# sourceMappingURL=types.js.map