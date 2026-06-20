/**
 * Author and copyright: Stefan Haack (https://shaack.com)
 * Repository: https://github.com/shaack/cm-chessboard
 * License: MIT, see file 'LICENSE'
 */
const FEN = {
    empty: "8/8/8/8/8/8/8/8"
};

class Position {

    constructor(fen = FEN.empty) {
        this.squares = new Array(64).fill(null);
        this.setFen(fen);
    }

    setFen(fen = FEN.empty) {
        const parts = fen.replace(/^\s*/, "").replace(/\s*$/, "").split(/\/|\s/);
        for (let part = 0; part < 8; part++) {
            const row = parts[7 - part].replace(/\d/g, (str) => {
                const numSpaces = parseInt(str);
                let ret = '';
                for (let i = 0; i < numSpaces; i++) {
                    ret += '-';
                }
                return ret
            });
            for (let c = 0; c < 8; c++) {
                const char = row.substring(c, c + 1);
                let piece = null;
                if (char !== '-') {
                    if (char.toUpperCase() === char) {
                        piece = `w${char.toLowerCase()}`;
                    } else {
                        piece = `b${char}`;
                    }
                }
                this.squares[part * 8 + c] = piece;
            }
        }
    }

    getFen() {
        let parts = new Array(8).fill("");
        for (let part = 0; part < 8; part++) {
            let spaceCounter = 0;
            for (let i = 0; i < 8; i++) {
                const piece = this.squares[part * 8 + i];
                if (!piece) {
                    spaceCounter++;
                } else {
                    if (spaceCounter > 0) {
                        parts[7 - part] += spaceCounter;
                        spaceCounter = 0;
                    }
                    const color = piece.substring(0, 1);
                    const name = piece.substring(1, 2);
                    if (color === "w") {
                        parts[7 - part] += name.toUpperCase();
                    } else {
                        parts[7 - part] += name;
                    }
                }
            }
            if (spaceCounter > 0) {
                parts[7 - part] += spaceCounter;
                spaceCounter = 0;
            }
        }
        return parts.join("/")
    }

    getPieces(pieceColor = undefined, pieceType = undefined, sortBy = ['k', 'q', 'r', 'b', 'n', 'p']) {
        const pieces = [];
        const sort = (a, b) => {
            return sortBy.indexOf(a.name) - sortBy.indexOf(b.name)
        };
        for (let i = 0; i < 64; i++) {
            const piece = this.squares[i];
            if (piece) {
                const type = piece.charAt(1);
                const color = piece.charAt(0);
                const square = Position.indexToSquare(i);
                if(pieceType && pieceType !== type || pieceColor && pieceColor !== color) {
                    continue
                }
                pieces.push({
                    name: type, // deprecated, use type
                    type: type,
                    color: color,
                    position: square, // deprecated, use square
                    square: square
                });
            }
        }
        if (sortBy) {
            pieces.sort(sort);
        }
        return pieces
    }

    movePiece(squareFrom, squareTo) {
        if (!this.squares[Position.squareToIndex(squareFrom)]) {
            console.warn("no piece on", squareFrom);
            return
        }
        this.squares[Position.squareToIndex(squareTo)] = this.squares[Position.squareToIndex(squareFrom)];
        this.squares[Position.squareToIndex(squareFrom)] = null;
    }

    setPiece(square, piece) {
        this.squares[Position.squareToIndex(square)] = piece;
    }

    getPiece(square) {
        return this.squares[Position.squareToIndex(square)]
    }

    static squareToIndex(square) {
        const coordinates = Position.squareToCoordinates(square);
        return coordinates[0] + coordinates[1] * 8
    }

    static indexToSquare(index) {
        return this.coordinatesToSquare([Math.floor(index % 8), index / 8])
    }

    static squareToCoordinates(square) {
        const file = square.charCodeAt(0) - 97;
        const rank = square.charCodeAt(1) - 49;
        return [file, rank]
    }

    static coordinatesToSquare(coordinates) {
        const file = String.fromCharCode(coordinates[0] + 97);
        const rank = String.fromCharCode(coordinates[1] + 49);
        return file + rank
    }

    toString() {
        return this.getFen()
    }

    clone() {
        const cloned = Object.create(Position.prototype);
        cloned.squares = this.squares.slice(0);
        return cloned
    }

}

/**
 * Author and copyright: Stefan Haack (https://shaack.com)
 * Repository: https://github.com/shaack/cm-chessboard
 * License: MIT, see file 'LICENSE'
 */

class ChessboardState {

    constructor() {
        this.position = new Position();
        this.orientation = undefined;
        this.inputWhiteEnabled = false;
        this.inputBlackEnabled = false;
        this.squareSelectEnabled = false;
        this.moveInputCallback = null;
        this.extensionPoints = {};
        this.moveInputProcess = Promise.resolve();
    }

    inputEnabled() {
        return this.inputWhiteEnabled || this.inputBlackEnabled
    }

    invokeExtensionPoints(name, data = {}) {
        const extensionPoints = this.extensionPoints[name];
        const dataCloned = Object.assign({}, data);
        dataCloned.extensionPoint = name;
        let returnValue = true;
        if (extensionPoints) {
            for (const extensionPoint of extensionPoints) {
                if(extensionPoint(dataCloned) === false) {
                    returnValue = false;
                }
            }
        }
        return returnValue
    }

}

/**
 * Author and copyright: Stefan Haack (https://shaack.com)
 * Repository: https://github.com/shaack/cm-chessboard
 * License: MIT, see file 'LICENSE'
 */

const SVG_NAMESPACE = "http://www.w3.org/2000/svg";

class Svg {

    /**
     * create the Svg in the HTML DOM
     * @param containerElement
     * @returns {Element}
     */
    static createSvg(containerElement = undefined) {
        let svg = document.createElementNS(SVG_NAMESPACE, "svg");
        if (containerElement) {
            svg.setAttribute("width", "100%");
            svg.setAttribute("height", "100%");
            containerElement.appendChild(svg);
        }
        return svg
    }

    /**
     * Add an Element to an SVG DOM
     * @param parent
     * @param name
     * @param attributes
     * @returns {Element}
     */
    static addElement(parent, name, attributes = {}) {
        let element = document.createElementNS(SVG_NAMESPACE, name);
        if (name === "use") {
            attributes["xlink:href"] = attributes["href"]; // fix for safari
        }
        for (let attribute in attributes) {
            if (attributes.hasOwnProperty(attribute)) {
                if (attribute.indexOf(":") !== -1) {
                    const value = attribute.split(":");
                    element.setAttributeNS("http://www.w3.org/1999/" + value[0], value[1], attributes[attribute]);
                } else {
                    element.setAttribute(attribute, attributes[attribute]);
                }
            }
        }
        parent.appendChild(element);
        return element
    }

    /**
     * Remove an element from an SVG DOM
     * @param element
     */
    static removeElement(element) {
        if(!element) {
            console.warn("removeElement, element is", element);
            return
        }
        if (element.parentNode) {
            element.parentNode.removeChild(element);
        } else {
            console.warn(element, "without parentNode");
        }
    }

}

/**
 * Author and copyright: Stefan Haack (https://shaack.com)
 * Repository: https://github.com/shaack/cm-chessboard
 * License: MIT, see file 'LICENSE'
 */

const EXTENSION_POINT = {
    positionChanged: "positionChanged", // the positions of the pieces was changed
    boardChanged: "boardChanged", // the board (orientation) was changed
    moveInputToggled: "moveInputToggled", // move input was enabled or disabled
    moveInput: "moveInput", // move started, moving over a square, validating or canceled
    beforeRedrawBoard: "beforeRedrawBoard", // called before redrawing the board
    afterRedrawBoard: "afterRedrawBoard", // called after redrawing the board
    redrawBoard: "redrawBoard", // called after redrawing the board, DEPRECATED, use afterRedrawBoard 2023-09-18
    animation: "animation", // called on animation start, end, and on every animation frame
    destroy: "destroy" // called, before the board is destroyed
};

class Extension {

    constructor(chessboard) {
        this.chessboard = chessboard;
    }

    registerExtensionPoint(name, callback) {
        if(name === EXTENSION_POINT.redrawBoard) { // deprecated 2023-09-18
            console.warn("EXTENSION_POINT.redrawBoard is deprecated, use EXTENSION_POINT.afterRedrawBoard");
            name = EXTENSION_POINT.afterRedrawBoard;
        }
        if (!this.chessboard.state.extensionPoints[name]) {
            this.chessboard.state.extensionPoints[name] = [];
        }
        this.chessboard.state.extensionPoints[name].push(callback);
    }

    /** @deprecated 2023-05-18 */
    registerMethod(name, callback) {
        console.warn("registerMethod is deprecated, just add methods directly to the chessboard instance");
        if (!this.chessboard[name]) {
            this.chessboard[name] = (...args) => {
                return callback.apply(this, args)
            };
        } else {
            log.error("method", name, "already exists");
        }
    }

}

/**
 * Author and copyright: Stefan Haack (https://shaack.com)
 * Repository: https://github.com/shaack/cm-chessboard
 * License: MIT, see file 'LICENSE'
 */

class Utils {

    static delegate(element, eventName, selector, handler) {
        const eventListener = function (event) {
            const match = event.target.closest(selector);
            if (match && this.contains(match)) {
                handler.call(match, event);
            }
        };
        element.addEventListener(eventName, eventListener);
        return {
            remove: function () {
                element.removeEventListener(eventName, eventListener);
            }
        }
    }

    static mergeObjects(target, source) {
        const isObject = (obj) => obj && typeof obj === 'object';
        if (!isObject(target) || !isObject(source)) {
            return source
        }
        for (const key of Object.keys(source)) {
            if (source[key] instanceof Object) {
                Object.assign(source[key], Utils.mergeObjects(target[key], source[key]));
            }
        }
        Object.assign(target || {}, source);
        return target
    }

    static createDomElement(html) {
        const template = document.createElement('template');
        template.innerHTML = html.trim();
        return template.content.firstChild
    }

    static createTask() {
        let resolve, reject;
        const promise = new Promise(function (_resolve, _reject) {
            resolve = _resolve;
            reject = _reject;
        });
        promise.resolve = resolve;
        promise.reject = reject;
        return promise
    }

    static isAbsoluteUrl(url) {
        return url.indexOf("://") !== -1 || url.startsWith("/")
    }

}

/**
 * Author and copyright: Stefan Haack (https://shaack.com)
 * Repository: https://github.com/shaack/cm-chessboard
 * License: MIT, see file 'LICENSE'
 */

/*
* Thanks to markosyan for the idea of the PromiseQueue
* https://medium.com/@karenmarkosyan/how-to-manage-promises-into-dynamic-queue-with-vanilla-javascript-9d0d1f8d4df5
*/

const ANIMATION_EVENT_TYPE = {
    start: "start",
    frame: "frame",
    end: "end"
};

class PromiseQueue {

    constructor() {
        this.queue = [];
        this.workingOnPromise = false;
        this.stop = false;
    }

    async enqueue(promise) {
        return new Promise((resolve, reject) => {
            this.queue.push({
                promise, resolve, reject,
            });
            this.dequeue();
        })
    }

    dequeue() {
        if (this.workingOnPromise) {
            return
        }
        if (this.stop) {
            this.queue = [];
            this.stop = false;
            return
        }
        const entry = this.queue.shift();
        if (!entry) {
            return
        }
        try {
            this.workingOnPromise = true;
            entry.promise().then((value) => {
                this.workingOnPromise = false;
                entry.resolve(value);
                this.dequeue();
            }).catch(err => {
                this.workingOnPromise = false;
                entry.reject(err);
                this.dequeue();
            });
        } catch (err) {
            this.workingOnPromise = false;
            entry.reject(err);
            this.dequeue();
        }
        return true
    }

    destroy() {
        this.stop = true;
    }

}


const CHANGE_TYPE = {
    move: 0,
    appear: 1,
    disappear: 2
};

class PositionsAnimation {

    constructor(view, fromPosition, toPosition, duration, callback) {
        this.view = view;
        if (fromPosition && toPosition) {
            this.animatedElements = this.createAnimation(fromPosition.squares, toPosition.squares);
            this.duration = duration;
            this.callback = callback;
            this.frameHandle = requestAnimationFrame(this.animationStep.bind(this));
        } else {
            console.error("fromPosition", fromPosition, "toPosition", toPosition);
        }
        this.view.positionsAnimationTask = Utils.createTask();
        this.view.chessboard.state.invokeExtensionPoints(EXTENSION_POINT.animation, {
            type: ANIMATION_EVENT_TYPE.start
        });
    }

    static seekChanges(fromSquares, toSquares) {
        const appearedList = [], disappearedList = [], changes = [];
        for (let i = 0; i < 64; i++) {
            const previousSquare = fromSquares[i];
            const newSquare = toSquares[i];
            if (newSquare !== previousSquare) {
                if (newSquare) {
                    appearedList.push({piece: newSquare, index: i});
                }
                if (previousSquare) {
                    disappearedList.push({piece: previousSquare, index: i});
                }
            }
        }
        appearedList.forEach((appeared) => {
            let shortestDistance = 8;
            let foundMoved = null;
            disappearedList.forEach((disappeared) => {
                if (appeared.piece === disappeared.piece) {
                    const moveDistance = PositionsAnimation.squareDistance(appeared.index, disappeared.index);
                    if (moveDistance < shortestDistance) {
                        foundMoved = disappeared;
                        shortestDistance = moveDistance;
                    }
                }
            });
            if (foundMoved) {
                disappearedList.splice(disappearedList.indexOf(foundMoved), 1); // remove from disappearedList, because it is moved now
                changes.push({
                    type: CHANGE_TYPE.move,
                    piece: appeared.piece,
                    atIndex: foundMoved.index,
                    toIndex: appeared.index
                });
            } else {
                changes.push({type: CHANGE_TYPE.appear, piece: appeared.piece, atIndex: appeared.index});
            }
        });
        disappearedList.forEach((disappeared) => {
            changes.push({type: CHANGE_TYPE.disappear, piece: disappeared.piece, atIndex: disappeared.index});
        });
        return changes
    }

    createAnimation(fromSquares, toSquares) {
        const changes = PositionsAnimation.seekChanges(fromSquares, toSquares);
        const animatedElements = [];
        changes.forEach((change) => {
            const animatedItem = {
                type: change.type
            };
            switch (change.type) {
                case CHANGE_TYPE.move:
                    animatedItem.element = this.view.getPieceElement(Position.indexToSquare(change.atIndex));
                    animatedItem.element.parentNode.appendChild(animatedItem.element); // move element to top layer
                    animatedItem.atPoint = this.view.indexToPoint(change.atIndex);
                    animatedItem.toPoint = this.view.indexToPoint(change.toIndex);
                    break
                case CHANGE_TYPE.appear:
                    animatedItem.element = this.view.drawPieceOnSquare(Position.indexToSquare(change.atIndex), change.piece);
                    animatedItem.element.style.opacity = 0;
                    break
                case CHANGE_TYPE.disappear:
                    animatedItem.element = this.view.getPieceElement(Position.indexToSquare(change.atIndex));
                    break
            }
            animatedElements.push(animatedItem);
        });
        return animatedElements
    }

    animationStep(time) {
        if(!this.view || !this.view.chessboard.state) { // board was destroyed
            return
        }
        if (!this.startTime) {
            this.startTime = time;
        }
        const timeDiff = time - this.startTime;
        if (timeDiff <= this.duration) {
            this.frameHandle = requestAnimationFrame(this.animationStep.bind(this));
        } else {
            cancelAnimationFrame(this.frameHandle);
            this.animatedElements.forEach((animatedItem) => {
                if (animatedItem.type === CHANGE_TYPE.disappear) {
                    Svg.removeElement(animatedItem.element);
                }
            });
            this.view.positionsAnimationTask.resolve();
            this.view.chessboard.state.invokeExtensionPoints(EXTENSION_POINT.animation, {
                type: ANIMATION_EVENT_TYPE.end
            });
            this.callback();
            return
        }
        const t = Math.min(1, timeDiff / this.duration);
        let progress = t < .5 ? 2 * t * t : -1 + (4 - 2 * t) * t; // easeInOut
        if (isNaN(progress) || progress > 0.99) {
            progress = 1;
        }
        this.animatedElements.forEach((animatedItem) => {
            if (animatedItem.element) {
                switch (animatedItem.type) {
                    case CHANGE_TYPE.move:
                        animatedItem.element.transform.baseVal.removeItem(0);
                        const transform = (this.view.svg.createSVGTransform());
                        transform.setTranslate(
                            animatedItem.atPoint.x + (animatedItem.toPoint.x - animatedItem.atPoint.x) * progress,
                            animatedItem.atPoint.y + (animatedItem.toPoint.y - animatedItem.atPoint.y) * progress);
                        animatedItem.element.transform.baseVal.appendItem(transform);
                        break
                    case CHANGE_TYPE.appear:
                        animatedItem.element.style.opacity = Math.round(progress * 100) / 100;
                        break
                    case CHANGE_TYPE.disappear:
                        animatedItem.element.style.opacity = Math.round((1 - progress) * 100) / 100;
                        break
                }
            } else {
                console.warn("animatedItem has no element", animatedItem);
            }
        });
        this.view.chessboard.state.invokeExtensionPoints(EXTENSION_POINT.animation, {
            type: ANIMATION_EVENT_TYPE.frame,
            progress: progress
        });
    }

    static squareDistance(index1, index2) {
        const file1 = index1 % 8;
        const rank1 = Math.floor(index1 / 8);
        const file2 = index2 % 8;
        const rank2 = Math.floor(index2 / 8);
        return Math.max(Math.abs(rank2 - rank1), Math.abs(file2 - file1))
    }

}

class PositionAnimationsQueue extends PromiseQueue {

    constructor(chessboard) {
        super();
        this.chessboard = chessboard;
    }

    async enqueuePositionChange(positionFrom, positionTo, animated) {
        if(positionFrom.getFen() === positionTo.getFen()) {
            // No diff to animate. Still go through the queue so the promise
            // resolves after any animations already in flight (e.g. an
            // earlier movePiece from a drag). See issue #154.
            return super.enqueue(() => Promise.resolve())
        } else {
            return super.enqueue(() => new Promise((resolve) => {
                let duration = animated ? this.chessboard.props.style.animationDuration : 0;
                if (this.queue.length > 0) {
                    duration = duration / (1 + Math.pow(this.queue.length / 5, 2));
                }
                new PositionsAnimation(this.chessboard.view,
                    positionFrom, positionTo, animated ? duration : 0,
                    () => {
                        if (this.chessboard.view) { // if destroyed, no view anymore
                            this.chessboard.view.redrawPieces(positionTo.squares);
                        }
                        resolve();
                    }
                );
            }))
        }
    }

    async enqueueTurnBoard(position, color, animated) {
        return super.enqueue(() => new Promise((resolve) => {
            const emptyPosition = new Position(FEN.empty);
            let duration = animated ? this.chessboard.props.style.animationDuration : 0;
            if(this.queue.length > 0) {
                duration = duration / (1 + Math.pow(this.queue.length / 5, 2));
            }
            new PositionsAnimation(this.chessboard.view,
                position, emptyPosition, animated ? duration : 0,
                () => {
                    this.chessboard.state.orientation = color;
                    this.chessboard.view.redrawBoard();
                    this.chessboard.view.redrawPieces(emptyPosition.squares);
                    new PositionsAnimation(this.chessboard.view,
                        emptyPosition, position, animated ? duration : 0,
                        () => {
                            this.chessboard.view.redrawPieces(position.squares);
                            resolve();
                        }
                    );
                }
            );
        }))
    }

}

/**
 * Author and copyright: Stefan Haack (https://shaack.com)
 * Repository: https://github.com/shaack/cm-chessboard
 * License: MIT, see file 'LICENSE'
 */


const MOVE_INPUT_STATE = {
    waitForInputStart: "waitForInputStart",
    pieceClickedThreshold: "pieceClickedThreshold",
    clickTo: "clickTo",
    secondClickThreshold: "secondClickThreshold",
    dragTo: "dragTo",
    clickDragTo: "clickDragTo",
    moveDone: "moveDone",
    reset: "reset"
};

const MOVE_CANCELED_REASON = {
    secondClick: "secondClick", // clicked the same piece
    secondaryClick: "secondaryClick", // right click while moving
    movedOutOfBoard: "movedOutOfBoard",
    draggedBack: "draggedBack", // dragged to the start square
    clickedAnotherPiece: "clickedAnotherPiece" // of the same color
};

const DRAG_THRESHOLD = 4;

class VisualMoveInput {

    constructor(view) {
        this.view = view;
        this.chessboard = view.chessboard;
        this.moveInputState = null;
        this.fromSquare = null;
        this.toSquare = null;

        this.setMoveInputState(MOVE_INPUT_STATE.waitForInputStart);
    }

    moveInputStartedCallback(square) {
        const result = this.view.moveInputStartedCallback(square);
        if (result) {
            this.chessboard.state.moveInputProcess = Utils.createTask();
            this.chessboard.state.moveInputProcess.then((result) => {
                if (this.moveInputState === MOVE_INPUT_STATE.waitForInputStart ||
                    this.moveInputState === MOVE_INPUT_STATE.moveDone) {
                    this.view.moveInputFinishedCallback(this.fromSquare, this.toSquare, result);
                }
            });
        }
        return result
    }

    movingOverSquareCallback(fromSquare, toSquare) {
        this.view.movingOverSquareCallback(fromSquare, toSquare);
    }

    validateMoveInputCallback(fromSquare, toSquare) {
        const result = this.view.validateMoveInputCallback(fromSquare, toSquare);
        this.chessboard.state.moveInputProcess.resolve(result);
        return result
    }

    moveInputCanceledCallback(fromSquare, toSquare, reason) {
        this.view.moveInputCanceledCallback(fromSquare, toSquare, reason);
        this.chessboard.state.moveInputProcess.resolve();
    }

    setMoveInputState(newState, params = undefined) {
        const prevState = this.moveInputState;
        this.moveInputState = newState;

        switch (newState) {

            case MOVE_INPUT_STATE.waitForInputStart:
                break

            case MOVE_INPUT_STATE.pieceClickedThreshold:
                if (MOVE_INPUT_STATE.waitForInputStart !== prevState && MOVE_INPUT_STATE.clickTo !== prevState) {
                    throw new Error("moveInputState")
                }
                if (this.pointerMoveListener) {
                    removeEventListener(this.pointerMoveListener.type, this.pointerMoveListener);
                    this.pointerMoveListener = null;
                }
                if (this.pointerUpListener) {
                    removeEventListener(this.pointerUpListener.type, this.pointerUpListener);
                    this.pointerUpListener = null;
                }
                this.fromSquare = params.square;
                this.toSquare = null;
                this.movedPiece = params.piece;
                this.startPoint = params.point;
                if (!this.pointerMoveListener && !this.pointerUpListener) {
                    if (params.type === "mousedown") {
                        this.pointerMoveListener = this.onPointerMove.bind(this);
                        this.pointerMoveListener.type = "mousemove";
                        addEventListener("mousemove", this.pointerMoveListener);
                        this.pointerUpListener = this.onPointerUp.bind(this);
                        this.pointerUpListener.type = "mouseup";
                        addEventListener("mouseup", this.pointerUpListener);
                    } else if (params.type === "touchstart") {
                        this.pointerMoveListener = this.onPointerMove.bind(this);
                        this.pointerMoveListener.type = "touchmove";
                        addEventListener("touchmove", this.pointerMoveListener);
                        this.pointerUpListener = this.onPointerUp.bind(this);
                        this.pointerUpListener.type = "touchend";
                        addEventListener("touchend", this.pointerUpListener);
                    } else {
                        throw Error("4b74af")
                    }
                    if (!this.contextMenuListener) {
                        this.contextMenuListener = this.onContextMenu.bind(this);
                        this.chessboard.view.svg.addEventListener("contextmenu", this.contextMenuListener);
                    }
                } else {
                    throw Error("94ad0c")
                }
                break

            case MOVE_INPUT_STATE.clickTo:
                if (this.draggablePiece) {
                    Svg.removeElement(this.draggablePiece);
                    this.draggablePiece = null;
                }
                if (prevState === MOVE_INPUT_STATE.dragTo) {
                    this.view.setPieceVisibility(params.square, true);
                }
                break

            case MOVE_INPUT_STATE.secondClickThreshold:
                if (MOVE_INPUT_STATE.clickTo !== prevState) {
                    throw new Error("moveInputState")
                }
                this.startPoint = params.point;
                break

            case MOVE_INPUT_STATE.dragTo:
                if (MOVE_INPUT_STATE.pieceClickedThreshold !== prevState) {
                    throw new Error("moveInputState")
                }
                if (this.view.chessboard.state.inputEnabled()) {
                    this.view.setPieceVisibility(params.square, false);
                    this.createDraggablePiece(params.piece);
                }
                break

            case MOVE_INPUT_STATE.clickDragTo:
                if (MOVE_INPUT_STATE.secondClickThreshold !== prevState) {
                    throw new Error("moveInputState")
                }
                if (this.view.chessboard.state.inputEnabled()) {
                    this.view.setPieceVisibility(params.square, false);
                    this.createDraggablePiece(params.piece);
                }
                break

            case MOVE_INPUT_STATE.moveDone:
                if ([MOVE_INPUT_STATE.dragTo, MOVE_INPUT_STATE.clickTo, MOVE_INPUT_STATE.clickDragTo].indexOf(prevState) === -1) {
                    throw new Error("moveInputState")
                }
                this.toSquare = params.square;
                if (this.toSquare && this.validateMoveInputCallback(this.fromSquare, this.toSquare)) {
                    this.chessboard.movePiece(this.fromSquare, this.toSquare, prevState === MOVE_INPUT_STATE.clickTo).then(() => {
                        if (prevState === MOVE_INPUT_STATE.clickTo) {
                            this.view.setPieceVisibility(this.toSquare, true);
                        }
                        this.setMoveInputState(MOVE_INPUT_STATE.reset);
                    });
                } else {
                    this.view.setPieceVisibility(this.fromSquare, true);
                    this.setMoveInputState(MOVE_INPUT_STATE.reset);
                }
                break

            case MOVE_INPUT_STATE.reset:
                if (this.fromSquare && !this.toSquare && this.movedPiece) {
                    this.chessboard.state.position.setPiece(this.fromSquare, this.movedPiece);
                }
                this.fromSquare = null;
                this.toSquare = null;
                this.movedPiece = null;
                if (this.draggablePiece) {
                    Svg.removeElement(this.draggablePiece);
                    this.draggablePiece = null;
                }
                if (this.pointerMoveListener) {
                    removeEventListener(this.pointerMoveListener.type, this.pointerMoveListener);
                    this.pointerMoveListener = null;
                }
                if (this.pointerUpListener) {
                    removeEventListener(this.pointerUpListener.type, this.pointerUpListener);
                    this.pointerUpListener = null;
                }
                if (this.contextMenuListener) {
                    removeEventListener("contextmenu", this.contextMenuListener);
                    this.contextMenuListener = null;
                }
                this.setMoveInputState(MOVE_INPUT_STATE.waitForInputStart);
                // set temporarily hidden pieces visible again
                const hiddenPieces = this.view.piecesGroup.querySelectorAll("[visibility=hidden]");
                for (let i = 0; i < hiddenPieces.length; i++) {
                    hiddenPieces[i].removeAttribute("visibility");
                }
                break

            default:
                throw Error(`260b09: moveInputState ${newState}`)
        }
    }

    createDraggablePiece(pieceName) {
        // maybe I should use the existing piece from the board and don't create a new one
        if (this.draggablePiece) {
            throw Error("draggablePiece already exists")
        }
        this.draggablePiece = Svg.createSvg(document.body);
        this.draggablePiece.classList.add("cm-chessboard-draggable-piece");
        this.draggablePiece.setAttribute("width", this.view.squareWidth);
        this.draggablePiece.setAttribute("height", this.view.squareHeight);
        this.draggablePiece.setAttribute("style", "pointer-events: none");
        this.draggablePiece.name = pieceName;
        const spriteUrl = this.chessboard.props.assetsCache ? "" : this.view.getSpriteUrl();
        const piece = Svg.addElement(this.draggablePiece, "use", {
            href: `${spriteUrl}#${pieceName}`
        });
        const scaling = this.view.squareHeight / this.chessboard.props.style.pieces.tileSize;
        const transformScale = (this.draggablePiece.createSVGTransform());
        transformScale.setScale(scaling, scaling);
        piece.transform.baseVal.appendItem(transformScale);
    }

    moveDraggablePiece(x, y) {
        this.draggablePiece.setAttribute("style",
            `pointer-events: none; position: absolute; left: ${x - (this.view.squareHeight / 2)}px; top: ${y - (this.view.squareHeight / 2)}px`);
    }

    onPointerDown(e) {
        if (!(e.type === "mousedown" && e.button === 0 || e.type === "touchstart")) {
            return
        }
        const square = e.target.getAttribute("data-square");
        if (!square) { // pointer on square
            return
        }
        const pieceName = this.chessboard.getPiece(square);
        let color;
        if (pieceName) {
            color = pieceName ? pieceName.substring(0, 1) : null;
            // allow scrolling, if not pointed on draggable piece
            if (color === "w" && this.chessboard.state.inputWhiteEnabled ||
                color === "b" && this.chessboard.state.inputBlackEnabled) {
                e.preventDefault();
            }
        }
        if (this.moveInputState !== MOVE_INPUT_STATE.waitForInputStart ||
            this.chessboard.state.inputWhiteEnabled && color === "w" ||
            this.chessboard.state.inputBlackEnabled && color === "b") {
            let point;
            if (e.type === "mousedown") {
                point = {x: e.clientX, y: e.clientY};
            } else if (e.type === "touchstart") {
                point = {x: e.touches[0].clientX, y: e.touches[0].clientY};
            }
            if (this.moveInputState === MOVE_INPUT_STATE.waitForInputStart && pieceName && this.moveInputStartedCallback(square)) {
                this.setMoveInputState(MOVE_INPUT_STATE.pieceClickedThreshold, {
                    square: square,
                    piece: pieceName,
                    point: point,
                    type: e.type
                });
            } else if (this.moveInputState === MOVE_INPUT_STATE.clickTo) {
                if (square === this.fromSquare) {
                    this.setMoveInputState(MOVE_INPUT_STATE.secondClickThreshold, {
                        square: square,
                        piece: pieceName,
                        point: point,
                        type: e.type
                    });
                } else {
                    const pieceName = this.chessboard.getPiece(square);
                    const pieceColor = pieceName ? pieceName.substring(0, 1) : null;
                    const startPieceName = this.chessboard.getPiece(this.fromSquare);
                    const startPieceColor = startPieceName ? startPieceName.substring(0, 1) : null;
                    if (color && startPieceColor === pieceColor) {
                        // added to allow chess960 castling
                        const result = this.validateMoveInputCallback(this.fromSquare, square);
                        if(!result) {
                            this.moveInputCanceledCallback(this.fromSquare, square, MOVE_CANCELED_REASON.clickedAnotherPiece);
                            if (this.moveInputStartedCallback(square)) {
                                this.setMoveInputState(MOVE_INPUT_STATE.pieceClickedThreshold, {
                                    square: square,
                                    piece: pieceName,
                                    point: point,
                                    type: e.type
                                });
                            } else {
                                this.setMoveInputState(MOVE_INPUT_STATE.reset);
                            }
                        }
                    } else {
                        this.setMoveInputState(MOVE_INPUT_STATE.moveDone, {square: square});
                    }
                }
            }
        }
    }

    onPointerMove(e) {
        let pageX, pageY, clientX, clientY, target;
        if (e.type === "mousemove") {
            clientX = e.clientX;
            clientY = e.clientY;
            pageX = e.pageX;
            pageY = e.pageY;
            target = e.target;
        } else if (e.type === "touchmove") {
            clientX = e.touches[0].clientX;
            clientY = e.touches[0].clientY;
            pageX = e.touches[0].pageX;
            pageY = e.touches[0].pageY;
            target = document.elementFromPoint(clientX, clientY);
        }
        if (this.moveInputState === MOVE_INPUT_STATE.pieceClickedThreshold || this.moveInputState === MOVE_INPUT_STATE.secondClickThreshold) {
            if (Math.abs(this.startPoint.x - clientX) > DRAG_THRESHOLD || Math.abs(this.startPoint.y - clientY) > DRAG_THRESHOLD) {
                if (this.moveInputState === MOVE_INPUT_STATE.secondClickThreshold) {
                    this.setMoveInputState(MOVE_INPUT_STATE.clickDragTo, {
                        square: this.fromSquare,
                        piece: this.movedPiece
                    });
                } else {
                    this.setMoveInputState(MOVE_INPUT_STATE.dragTo, {square: this.fromSquare, piece: this.movedPiece});
                }
                if (this.view.chessboard.state.inputEnabled()) {
                    this.moveDraggablePiece(pageX, pageY);
                }
            }
        } else if (this.moveInputState === MOVE_INPUT_STATE.dragTo || this.moveInputState === MOVE_INPUT_STATE.clickDragTo || this.moveInputState === MOVE_INPUT_STATE.clickTo) {
            if (target && target.getAttribute && target.parentElement === this.view.boardGroup) {
                const square = target.getAttribute("data-square");
                if (square !== this.fromSquare && square !== this.toSquare) {
                    this.toSquare = square;
                    this.movingOverSquareCallback(this.fromSquare, this.toSquare);
                } else if (square === this.fromSquare && this.toSquare !== null) {
                    this.toSquare = null;
                    this.movingOverSquareCallback(this.fromSquare, null);
                }
            } else if (this.toSquare !== null) {
                this.toSquare = null;
                this.movingOverSquareCallback(this.fromSquare, null);
            }

            if (this.view.chessboard.state.inputEnabled() && (this.moveInputState === MOVE_INPUT_STATE.dragTo || this.moveInputState === MOVE_INPUT_STATE.clickDragTo)) {
                this.moveDraggablePiece(pageX, pageY);
            }
        }
    }

    onPointerUp(e) {
        let target;
        if (e.type === "mouseup") {
            target = e.target;
        } else if (e.type === "touchend") {
            target = document.elementFromPoint(e.changedTouches[0].clientX, e.changedTouches[0].clientY);
        }
        if (target && target.getAttribute) {
            const square = target.getAttribute("data-square");

            if (square) {
                if (this.moveInputState === MOVE_INPUT_STATE.dragTo || this.moveInputState === MOVE_INPUT_STATE.clickDragTo) {
                    if (this.fromSquare === square) {
                        if (this.moveInputState === MOVE_INPUT_STATE.clickDragTo) {
                            this.chessboard.state.position.setPiece(this.fromSquare, this.movedPiece);
                            this.view.setPieceVisibility(this.fromSquare);
                            this.moveInputCanceledCallback(square, null, MOVE_CANCELED_REASON.draggedBack);
                            this.setMoveInputState(MOVE_INPUT_STATE.reset);
                        } else {
                            this.setMoveInputState(MOVE_INPUT_STATE.clickTo, {square: square});
                        }
                    } else {
                        this.setMoveInputState(MOVE_INPUT_STATE.moveDone, {square: square});
                    }
                } else if (this.moveInputState === MOVE_INPUT_STATE.pieceClickedThreshold) {
                    this.setMoveInputState(MOVE_INPUT_STATE.clickTo, {square: square});
                } else if (this.moveInputState === MOVE_INPUT_STATE.secondClickThreshold) {
                    this.setMoveInputState(MOVE_INPUT_STATE.reset);
                    this.moveInputCanceledCallback(square, null, MOVE_CANCELED_REASON.secondClick);
                }
            } else {
                this.view.redrawPieces();
                const moveStartSquare = this.fromSquare;
                this.setMoveInputState(MOVE_INPUT_STATE.reset);
                this.moveInputCanceledCallback(moveStartSquare, null, MOVE_CANCELED_REASON.movedOutOfBoard);
            }
        } else {
            this.view.redrawPieces();
            this.setMoveInputState(MOVE_INPUT_STATE.reset);
        }
    }

    onContextMenu(e) { // while moving
        e.preventDefault();
        this.view.redrawPieces();
        this.setMoveInputState(MOVE_INPUT_STATE.reset);
        this.moveInputCanceledCallback(this.fromSquare, null, MOVE_CANCELED_REASON.secondaryClick);
    }

    isDragging() {
        return this.moveInputState === MOVE_INPUT_STATE.dragTo || this.moveInputState === MOVE_INPUT_STATE.clickDragTo
    }

    destroy() {
        this.setMoveInputState(MOVE_INPUT_STATE.reset);
    }

}

/**
 * Author and copyright: Stefan Haack (https://shaack.com)
 * Repository: https://github.com/shaack/cm-chessboard
 * License: MIT, see file 'LICENSE'
 */


const COLOR = {
    white: "w",
    black: "b"
};
const INPUT_EVENT_TYPE = {
    moveInputStarted: "moveInputStarted",
    movingOverSquare: "movingOverSquare", // while dragging or hover after click
    validateMoveInput: "validateMoveInput",
    moveInputCanceled: "moveInputCanceled",
    moveInputFinished: "moveInputFinished"
};
const POINTER_EVENTS = {
    pointerdown: "pointerdown"};
const BORDER_TYPE = {
    none: "none", // no border
    thin: "thin", // thin border
    frame: "frame" // wide border with coordinates in it
};

class ChessboardView {
    constructor(chessboard) {
        this.chessboard = chessboard;
        this.visualMoveInput = new VisualMoveInput(this);
        if (chessboard.props.assetsCache) {
            this.cacheSpriteToDiv("cm-chessboard-sprite", this.getSpriteUrl());
        }
        this.container = document.createElement("div");
        this.chessboard.context.appendChild(this.container);
        if (chessboard.props.responsive) {
            if (typeof ResizeObserver !== "undefined") {
                this.resizeObserver = new ResizeObserver(() => {
                    // Defer via setTimeout to avoid "ResizeObserver loop
                    // completed with undelivered notifications." The timeout
                    // id is tracked so destroy() can cancel a pending call
                    // and avoid running handleResize on a destroyed board.
                    this.resizeTimeout = setTimeout(() => {
                        this.resizeTimeout = null;
                        this.handleResize();
                    });
                });
                this.resizeObserver.observe(this.chessboard.context);
            } else {
                this.resizeListener = this.handleResize.bind(this);
                window.addEventListener("resize", this.resizeListener);
            }
        }
        this.positionsAnimationTask = Promise.resolve();
        this.pointerDownListener = this.pointerDownHandler.bind(this);
        this.container.addEventListener("mousedown", this.pointerDownListener);
        this.container.addEventListener("touchstart", this.pointerDownListener, {passive: false});
        this.createSvgAndGroups();
        this.handleResize();
    }

    pointerDownHandler(e) {
        this.visualMoveInput.onPointerDown(e);
    }

    destroy() {
        this.visualMoveInput.destroy();
        if (this.resizeObserver) {
            this.resizeObserver.unobserve(this.chessboard.context);
        }
        // Cancel any pending handleResize that the ResizeObserver callback
        // has already scheduled via setTimeout. unobserve() stops new
        // notifications but does not clear timeouts we scheduled ourselves.
        if (this.resizeTimeout) {
            clearTimeout(this.resizeTimeout);
            this.resizeTimeout = null;
        }
        if (this.resizeListener) {
            window.removeEventListener("resize", this.resizeListener);
        }
        this.chessboard.context.removeEventListener("mousedown", this.pointerDownListener);
        this.chessboard.context.removeEventListener("touchstart", this.pointerDownListener);
        Svg.removeElement(this.svg);
        this.container.remove();
    }

    // Sprite //

    cacheSpriteToDiv(wrapperId, url) {
        if (!document.getElementById(wrapperId)) {
            const wrapper = document.createElement("div");
            wrapper.style.transform = "scale(0)";
            wrapper.style.position = "absolute";
            wrapper.setAttribute("aria-hidden", "true");
            wrapper.id = wrapperId;
            document.body.appendChild(wrapper);
            const xhr = new XMLHttpRequest();
            xhr.open("GET", url, true);
            xhr.onload = function () {
                wrapper.insertAdjacentHTML('afterbegin', xhr.response);
            };
            xhr.send();
        }
    }

    createSvgAndGroups() {
        this.svg = Svg.createSvg(this.container);
        // let description = document.createElement("description")
        // description.innerText = "Chessboard"
        // description.id = "svg-description"
        // this.svg.appendChild(description)
        let cssClass = this.chessboard.props.style.cssClass ? this.chessboard.props.style.cssClass : "default";
        this.svg.setAttribute("class", "cm-chessboard border-type-" + this.chessboard.props.style.borderType + " " + cssClass);
        // this.svg.setAttribute("aria-describedby", "svg-description")
        this.svg.setAttribute("role", "img");
        this.updateMetrics();
        this.boardGroup = Svg.addElement(this.svg, "g", {class: "board"});
        this.coordinatesGroup = Svg.addElement(this.svg, "g", {class: "coordinates", "aria-hidden": "true"});
        this.markersLayer = Svg.addElement(this.svg, "g", {class: "markers-layer"});
        this.piecesLayer = Svg.addElement(this.svg, "g", {class: "pieces-layer"});
        this.piecesGroup = Svg.addElement(this.piecesLayer, "g", {class: "pieces"});
        this.markersTopLayer = Svg.addElement(this.svg, "g", {class: "markers-top-layer"});
        this.interactiveTopLayer = Svg.addElement(this.svg, "g", {class: "interactive-top-layer"});
    }

    updateMetrics() {
        const piecesTileSize = this.chessboard.props.style.pieces.tileSize;
        this.width = this.container.clientWidth;
        this.height = this.container.clientWidth * (this.chessboard.props.style.aspectRatio || 1);
        if (this.chessboard.props.style.borderType === BORDER_TYPE.frame) {
            this.borderSize = this.width / 25;
        } else if (this.chessboard.props.style.borderType === BORDER_TYPE.thin) {
            this.borderSize = this.width / 320;
        } else {
            this.borderSize = 0;
        }
        this.innerWidth = this.width - 2 * this.borderSize;
        this.innerHeight = this.height - 2 * this.borderSize;
        this.squareWidth = this.innerWidth / 8;
        this.squareHeight = this.innerHeight / 8;
        this.scalingX = this.squareWidth / piecesTileSize;
        this.scalingY = this.squareHeight / piecesTileSize;
        this.pieceXTranslate = (this.squareWidth / 2 - piecesTileSize * this.scalingY / 2);
    }

    handleResize() {
        // Skip if the board has already been destroyed. The resizeObserver
        // or window resize listener may fire a callback whose deferred work
        // is still pending when destroy() runs.
        if (!this.chessboard || !this.chessboard.state) {
            return
        }
        this.container.style.width = (this.chessboard.context.clientWidth) + "px";
        this.container.style.height = (this.chessboard.context.clientWidth * this.chessboard.props.style.aspectRatio) + "px";
        if (this.container.clientWidth !== this.width || this.container.clientHeight !== this.height) {
            this.updateMetrics();
            this.redrawBoard();
            this.redrawPieces();
        }
        this.svg.setAttribute("width", "100%");
        this.svg.setAttribute("height", "100%");
    }

    redrawBoard() {
        this.chessboard.state.invokeExtensionPoints(EXTENSION_POINT.beforeRedrawBoard);
        this.redrawSquares();
        this.drawCoordinates();
        this.chessboard.state.invokeExtensionPoints(EXTENSION_POINT.afterRedrawBoard);
        this.visualizeInputState();
    }

    // Board //

    redrawSquares() {
        while (this.boardGroup.firstChild) {
            this.boardGroup.removeChild(this.boardGroup.lastChild);
        }

        let boardBorder = Svg.addElement(this.boardGroup, "rect", {width: this.width, height: this.height});
        boardBorder.setAttribute("class", "border");
        if (this.chessboard.props.style.borderType === BORDER_TYPE.frame) {
            const innerPos = this.borderSize;
            let borderInner = Svg.addElement(this.boardGroup, "rect", {
                x: innerPos, y: innerPos, width: this.width - innerPos * 2, height: this.height - innerPos * 2
            });
            borderInner.setAttribute("class", "border-inner");
        }

        for (let i = 0; i < 64; i++) {
            const index = this.chessboard.state.orientation === COLOR.white ? i : 63 - i;
            const squareColor = ((9 * index) & 8) === 0 ? 'black' : 'white';
            const fieldClass = `square ${squareColor}`;
            const point = this.squareToPoint(Position.indexToSquare(index));
            const squareRect = Svg.addElement(this.boardGroup, "rect", {
                x: point.x, y: point.y, width: this.squareWidth, height: this.squareHeight
            });
            squareRect.setAttribute("class", fieldClass);
            squareRect.setAttribute("data-square", Position.indexToSquare(index));
        }
    }

    drawCoordinates() {
        if (!this.chessboard.props.style.showCoordinates) {
            return
        }
        while (this.coordinatesGroup.firstChild) {
            this.coordinatesGroup.removeChild(this.coordinatesGroup.lastChild);
        }
        const inline = this.chessboard.props.style.borderType !== BORDER_TYPE.frame;
        for (let file = 0; file < 8; file++) {
            let x = this.borderSize + (17 + this.chessboard.props.style.pieces.tileSize * file) * this.scalingX;
            let y = this.height - this.scalingY * 3.5;
            let cssClass = "coordinate file";
            if (inline) {
                x = x + this.scalingX * 15.5;
                cssClass += file % 2 ? " white" : " black";
            }
            const textElement = Svg.addElement(this.coordinatesGroup, "text", {
                class: cssClass, x: x, y: y, style: `font-size: ${this.scalingY * 10}px`
            });
            if (this.chessboard.state.orientation === COLOR.white) {
                textElement.textContent = String.fromCharCode(97 + file);
            } else {
                textElement.textContent = String.fromCharCode(104 - file);
            }
        }
        for (let rank = 0; rank < 8; rank++) {
            let x = (this.borderSize / 3.7);
            let y = this.borderSize + 25 * this.scalingY + rank * this.squareHeight;
            let cssClass = "coordinate rank";
            if (inline) {
                cssClass += rank % 2 ? " black" : " white";
                if (this.chessboard.props.style.borderType === BORDER_TYPE.frame) {
                    x = x + this.scalingX * 10;
                    y = y - this.scalingY * 15;
                } else {
                    x = x + this.scalingX * 2;
                    y = y - this.scalingY * 15;
                }
            }
            const textElement = Svg.addElement(this.coordinatesGroup, "text", {
                class: cssClass, x: x, y: y, style: `font-size: ${this.scalingY * 10}px`
            });
            if (this.chessboard.state.orientation === COLOR.white) {
                textElement.textContent = "" + (8 - rank);
            } else {
                textElement.textContent = "" + (1 + rank);
            }
        }
    }

    // Pieces //

    redrawPieces(squares = this.chessboard.state.position.squares) {
        const childNodes = Array.from(this.piecesGroup.childNodes);
        const isDragging = this.visualMoveInput.isDragging();
        for (let i = 0; i < 64; i++) {
            const pieceName = squares[i];
            if (pieceName) {
                const square = Position.indexToSquare(i);
                this.drawPieceOnSquare(square, pieceName, isDragging && square === this.visualMoveInput.fromSquare);
            }
        }
        for (const childNode of childNodes) {
            this.piecesGroup.removeChild(childNode);
        }
    }

    drawPiece(parentGroup, pieceName, point) {
        const pieceGroup = Svg.addElement(parentGroup, "g", {});
        pieceGroup.setAttribute("data-piece", pieceName);
        const transform = (this.svg.createSVGTransform());
        transform.setTranslate(point.x, point.y);
        pieceGroup.transform.baseVal.appendItem(transform);
        const spriteUrl = this.chessboard.props.assetsCache ? "" : this.getSpriteUrl();
        const pieceUse = Svg.addElement(pieceGroup, "use", {
            href: `${spriteUrl}#${pieceName}`, class: "piece"
        });
        const transformScale = (this.svg.createSVGTransform());
        transformScale.setScale(this.scalingY, this.scalingY);
        pieceUse.transform.baseVal.appendItem(transformScale);
        return pieceGroup
    }

    drawPieceOnSquare(square, pieceName, hidden = false) {
        const pieceGroup = Svg.addElement(this.piecesGroup, "g", {});
        pieceGroup.setAttribute("data-piece", pieceName);
        pieceGroup.setAttribute("data-square", square);
        if (hidden) {
            pieceGroup.setAttribute("visibility", "hidden");
        }
        const point = this.squareToPoint(square);
        const transform = (this.svg.createSVGTransform());
        transform.setTranslate(point.x, point.y);
        pieceGroup.transform.baseVal.appendItem(transform);
        const spriteUrl = this.chessboard.props.assetsCache ? "" : this.getSpriteUrl();
        const pieceUse = Svg.addElement(pieceGroup, "use", {
            href: `${spriteUrl}#${pieceName}`, class: "piece"
        });
        // center on square
        const transformTranslate = (this.svg.createSVGTransform());
        transformTranslate.setTranslate(this.pieceXTranslate, 0);
        pieceUse.transform.baseVal.appendItem(transformTranslate);
        // scale
        const transformScale = (this.svg.createSVGTransform());
        transformScale.setScale(this.scalingY, this.scalingY);
        pieceUse.transform.baseVal.appendItem(transformScale);
        return pieceGroup
    }

    setPieceVisibility(square, visible = true) {
        const piece = this.getPieceElement(square);
        if (piece) {
            if (visible) {
                piece.setAttribute("visibility", "visible");
            } else {
                piece.setAttribute("visibility", "hidden");
            }
        } else {
            console.warn("no piece on", square);
        }
    }

    getPieceElement(square) {
        if (!square || square.length < 2) {
            console.warn("invalid square", square);
            return null
        }
        const piece = this.piecesGroup.querySelector(`g[data-square='${square}']`);
        if (!piece) {
            console.warn("no piece on", square);
            return null
        }
        return piece
    }

    // enable and disable move input //

    enableMoveInput(eventHandler, color = null) {
        if (this.chessboard.state.moveInputCallback) {
            throw Error("moveInput already enabled")
        }
        if (color === COLOR.white) {
            this.chessboard.state.inputWhiteEnabled = true;
        } else if (color === COLOR.black) {
            this.chessboard.state.inputBlackEnabled = true;
        } else {
            this.chessboard.state.inputWhiteEnabled = true;
            this.chessboard.state.inputBlackEnabled = true;
        }
        this.chessboard.state.moveInputCallback = eventHandler;
        this.chessboard.state.invokeExtensionPoints(EXTENSION_POINT.moveInputToggled, {enabled: true, color: color});
        this.visualizeInputState();
    }

    disableMoveInput() {
        this.chessboard.state.inputWhiteEnabled = false;
        this.chessboard.state.inputBlackEnabled = false;
        this.chessboard.state.moveInputCallback = null;
        this.chessboard.state.invokeExtensionPoints(EXTENSION_POINT.moveInputToggled, {enabled: false});
        this.visualizeInputState();
    }

    // callbacks //

    moveInputStartedCallback(square) {
        const data = {
            chessboard: this.chessboard,
            type: INPUT_EVENT_TYPE.moveInputStarted,
            square: square, /** square is deprecated, use squareFrom (2023-05-22) */
            squareFrom: square,
            piece: this.chessboard.getPiece(square)
        };
        if (this.chessboard.state.moveInputCallback) {
            data.moveInputCallbackResult = this.chessboard.state.moveInputCallback(data);
        }
        this.chessboard.state.invokeExtensionPoints(EXTENSION_POINT.moveInput, data);
        return data.moveInputCallbackResult
    }

    movingOverSquareCallback(squareFrom, squareTo) {
        const data = {
            chessboard: this.chessboard,
            type: INPUT_EVENT_TYPE.movingOverSquare,
            squareFrom: squareFrom,
            squareTo: squareTo,
            piece: this.chessboard.getPiece(squareFrom)
        };
        if (this.chessboard.state.moveInputCallback) {
            data.moveInputCallbackResult = this.chessboard.state.moveInputCallback(data);
        }
        this.chessboard.state.invokeExtensionPoints(EXTENSION_POINT.moveInput, data);
    }

    validateMoveInputCallback(squareFrom, squareTo) {
        const data = {
            chessboard: this.chessboard,
            type: INPUT_EVENT_TYPE.validateMoveInput,
            squareFrom: squareFrom,
            squareTo: squareTo,
            piece: this.chessboard.getPiece(squareFrom)
        };
        if (this.chessboard.state.moveInputCallback) {
            data.moveInputCallbackResult = this.chessboard.state.moveInputCallback(data);
        }
        this.chessboard.state.invokeExtensionPoints(EXTENSION_POINT.moveInput, data);
        return data.moveInputCallbackResult
    }

    moveInputCanceledCallback(squareFrom, squareTo, reason) {
        const data = {
            chessboard: this.chessboard,
            type: INPUT_EVENT_TYPE.moveInputCanceled,
            reason: reason,
            squareFrom: squareFrom,
            squareTo: squareTo
        };
        if (this.chessboard.state.moveInputCallback) {
            this.chessboard.state.moveInputCallback(data);
        }
        this.chessboard.state.invokeExtensionPoints(EXTENSION_POINT.moveInput, data);
    }

    moveInputFinishedCallback(squareFrom, squareTo, legalMove) {
        const data = {
            chessboard: this.chessboard,
            type: INPUT_EVENT_TYPE.moveInputFinished,
            squareFrom: squareFrom,
            squareTo: squareTo,
            legalMove: legalMove
        };
        if (this.chessboard.state.moveInputCallback) {
            this.chessboard.state.moveInputCallback(data);
        }
        this.chessboard.state.invokeExtensionPoints(EXTENSION_POINT.moveInput, data);
    }

    // Helpers //

    visualizeInputState() {
        if (this.chessboard.state) { // fix https://github.com/shaack/cm-chessboard/issues/47
            if (this.chessboard.state.inputWhiteEnabled || this.chessboard.state.inputBlackEnabled) {
                this.boardGroup.setAttribute("class", "board input-enabled");
            } else {
                this.boardGroup.setAttribute("class", "board");
            }
        }
    }

    indexToPoint(index) {
        let x, y;
        if (this.chessboard.state.orientation === COLOR.white) {
            x = this.borderSize + (index % 8) * this.squareWidth;
            y = this.borderSize + (7 - Math.floor(index / 8)) * this.squareHeight;
        } else {
            x = this.borderSize + (7 - index % 8) * this.squareWidth;
            y = this.borderSize + (Math.floor(index / 8)) * this.squareHeight;
        }
        return {x: x, y: y}
    }

    squareToPoint(square) {
        const index = Position.squareToIndex(square);
        return this.indexToPoint(index)
    }

    getSpriteUrl() {
        if (Utils.isAbsoluteUrl(this.chessboard.props.style.pieces.file)) {
            return this.chessboard.props.style.pieces.file
        } else {
            return this.chessboard.props.assetsUrl + this.chessboard.props.style.pieces.file
        }
    }
}

/**
 * Author and copyright: Stefan Haack (https://shaack.com)
 * Repository: https://github.com/shaack/cm-chessboard
 * License: MIT, see file 'LICENSE'
 */


const PIECE = {
    wp: "wp", wb: "wb", wn: "wn", wr: "wr", wq: "wq", wk: "wk",
    bp: "bp", bb: "bb", bn: "bn", br: "br", bq: "bq", bk: "bk"
};
const PIECES_FILE_TYPE = {
    svgSprite: "svgSprite"
};

class Chessboard {

    constructor(context, props = {}) {
        if (!context) {
            throw new Error("container element is " + context)
        }
        this.context = context;
        this.id = (Math.random() + 1).toString(36).substring(2, 8);
        this.extensions = [];
        this.props = {
            position: FEN.empty, // set position as fen, use FEN.start or FEN.empty as shortcuts
            orientation: COLOR.white, // white on bottom
            responsive: true, // resize the board automatically to the size of the context element
            assetsUrl: "./assets/", // put all css and sprites in this folder, will be ignored for absolute urls of assets files
            assetsCache: true, // cache the sprites, deactivate if you want to use multiple pieces sets in one page
            style: {
                cssClass: "default", // set the css theme of the board, try "green", "blue" or "chess-club"
                showCoordinates: true, // show ranks and files
                borderType: BORDER_TYPE.none, // "thin" thin border, "frame" wide border with coordinates in it, "none" no border
                aspectRatio: 1, // height/width of the board
                pieces: {
                    type: PIECES_FILE_TYPE.svgSprite, // pieces are in an SVG sprite, no other type supported for now
                    file: "pieces/standard.svg", // the filename of the sprite in `assets/pieces/` or an absolute url like `https://…` or `/…`
                    tileSize: 40 // the tile size in the sprite
                },
                animationDuration: 300 // pieces animation duration in milliseconds. Disable all animations with `0`
            },
            extensions: [ /* {class: ExtensionClass, props: { ... }} */] // add extensions here
        };
        Utils.mergeObjects(this.props, props);
        this.state = new ChessboardState();
        this.view = new ChessboardView(this);
        this.positionAnimationsQueue = new PositionAnimationsQueue(this);
        this.state.orientation = this.props.orientation;
        // instantiate extensions
        for (const extensionData of this.props.extensions) {
            this.addExtension(extensionData.class, extensionData.props);
        }
        this.view.redrawBoard();
        this.state.position = new Position(this.props.position);
        this.view.redrawPieces();
        this.state.invokeExtensionPoints(EXTENSION_POINT.positionChanged);
        this.initialized = Promise.resolve(); // deprecated 2023-09-19 don't use this anymore
    }

    // API //

    async setPiece(square, piece, animated = false) {
        const positionFrom = this.state.position.clone();
        this.state.position.setPiece(square, piece);
        this.state.invokeExtensionPoints(EXTENSION_POINT.positionChanged);
        return this.positionAnimationsQueue.enqueuePositionChange(positionFrom, this.state.position.clone(), animated)
    }

    async movePiece(squareFrom, squareTo, animated = false) {
        const positionFrom = this.state.position.clone();
        this.state.position.movePiece(squareFrom, squareTo);
        this.state.invokeExtensionPoints(EXTENSION_POINT.positionChanged);
        return this.positionAnimationsQueue.enqueuePositionChange(positionFrom, this.state.position.clone(), animated)
    }

    async setPosition(fen, animated = false) {
        const positionFrom = this.state.position.clone();
        const positionTo = new Position(fen);
        if (positionFrom.getFen() !== positionTo.getFen()) {
            this.state.position.setFen(fen);
            this.state.invokeExtensionPoints(EXTENSION_POINT.positionChanged);
        }
        return this.positionAnimationsQueue.enqueuePositionChange(positionFrom, this.state.position.clone(), animated)
    }

    async setOrientation(color, animated = false) {
        const position = this.state.position.clone();
        if (this.boardTurning) {
            console.warn("setOrientation is only once in queue allowed");
            return
        }
        this.boardTurning = true;
        return this.positionAnimationsQueue.enqueueTurnBoard(position, color, animated).then(() => {
            this.boardTurning = false;
            this.state.invokeExtensionPoints(EXTENSION_POINT.boardChanged);
        })
    }

    getPiece(square) {
        return this.state.position.getPiece(square)
    }

    getPosition() {
        return this.state.position.getFen()
    }

    getOrientation() {
        return this.state.orientation
    }

    enableMoveInput(eventHandler, color = undefined) {
        this.view.enableMoveInput(eventHandler, color);
    }

    disableMoveInput() {
        this.view.disableMoveInput();
    }

    isMoveInputEnabled() {
        return this.state.inputWhiteEnabled || this.state.inputBlackEnabled
    }

    enableSquareSelect(eventType = POINTER_EVENTS.pointerdown, eventHandler) {
        if (!this.squareSelectListener) {
            this.squareSelectListener = function (e) {
                const square = e.target.getAttribute("data-square");
                eventHandler({
                    eventType: e.type,
                    event: e,
                    chessboard: this,
                    square: square
                });
            };
        }
        this.context.addEventListener(eventType, this.squareSelectListener);
        this.state.squareSelectEnabled = true;
        this.view.visualizeInputState();
    }

    disableSquareSelect(eventType) {
        this.context.removeEventListener(eventType, this.squareSelectListener);
        this.squareSelectListener = undefined;
        this.state.squareSelectEnabled = false;
        this.view.visualizeInputState();
    }

    isSquareSelectEnabled() {
        return this.state.squareSelectEnabled
    }

    addExtension(extensionClass, props) {
        if (this.getExtension(extensionClass)) {
            throw Error("extension \"" + extensionClass.name + "\" already added")
        }
        this.extensions.push(new extensionClass(this, props));
    }

    getExtension(extensionClass) {
        for (const extension of this.extensions) {
            if (extension instanceof extensionClass) {
                return extension
            }
        }
        return null
    }

    destroy() {
        this.state.invokeExtensionPoints(EXTENSION_POINT.destroy);
        this.positionAnimationsQueue.destroy();
        this.view.destroy();
        this.view = undefined;
        this.state = undefined;
    }

}

/**
 * Author and copyright: Stefan Haack (https://shaack.com)
 * Repository: https://github.com/shaack/cm-chessboard
 * License: MIT, see file 'LICENSE'
 */

const MARKER_TYPE = {
    frame: {class: "marker-frame", slice: "markerFrame"},
    framePrimary: {class: "marker-frame-primary", slice: "markerFrame"},
    frameDanger: {class: "marker-frame-danger", slice: "markerFrame"},
    circle: {class: "marker-circle", slice: "markerCircle"},
    circlePrimary: {class: "marker-circle-primary", slice: "markerCircle"},
    circleDanger: {class: "marker-circle-danger", slice: "markerCircle"},
    circleDangerFilled: {class: "marker-circle-danger-filled", slice: "markerCircleFilled"},
    square: {class: "marker-square", slice: "markerSquare"},
    dot: {class: "marker-dot", slice: "markerDot", position: 'above'},
    bevel: {class: "marker-bevel", slice: "markerBevel"}
};

class Markers extends Extension {

    /** @constructor */
    constructor(chessboard, props = {}) {
        super(chessboard);
        this.registerExtensionPoint(EXTENSION_POINT.afterRedrawBoard, () => {
            this.onRedrawBoard();
        });
        this.registerExtensionPoint(EXTENSION_POINT.destroy, () => {
            this.onDestroy();
        });
        this.props = {
            autoMarkers: MARKER_TYPE.frame, // set to `null` to disable autoMarkers
            sprite: "extensions/markers/markers.svg" // the sprite file of the markers
        };
        Object.assign(this.props, props);
        if (chessboard.props.assetsCache) {
            chessboard.view.cacheSpriteToDiv("cm-chessboard-markers", this.getSpriteUrl());
        }
        chessboard.addMarker = this.addMarker.bind(this);
        chessboard.getMarkers = this.getMarkers.bind(this);
        chessboard.removeMarkers = this.removeMarkers.bind(this);
        chessboard.addLegalMovesMarkers = this.addLegalMovesMarkers.bind(this);
        chessboard.removeLegalMovesMarkers = this.removeLegalMovesMarkers.bind(this);
        this.markerGroupDown = Svg.addElement(chessboard.view.markersLayer, "g", {class: "markers"});
        this.markerGroupUp = Svg.addElement(chessboard.view.markersTopLayer, "g", {class: "markers"});
        this.markers = [];
        if (this.props.autoMarkers) {
            Object.assign(this.props.autoMarkers, this.props.autoMarkers);
            this.registerExtensionPoint(EXTENSION_POINT.moveInput, (event) => {
                this.drawAutoMarkers(event);
            });
        }
    }

    onDestroy() {
        this.markers.length = 0;
        if (this.markerGroupDown && this.markerGroupDown.parentNode) {
            this.markerGroupDown.parentNode.removeChild(this.markerGroupDown);
        }
        if (this.markerGroupUp && this.markerGroupUp.parentNode) {
            this.markerGroupUp.parentNode.removeChild(this.markerGroupUp);
        }
        delete this.chessboard.addMarker;
        delete this.chessboard.getMarkers;
        delete this.chessboard.removeMarkers;
        delete this.chessboard.addLegalMovesMarkers;
        delete this.chessboard.removeLegalMovesMarkers;
    }

    drawAutoMarkers(event) {
        if(event.type !== INPUT_EVENT_TYPE.moveInputFinished) {
            this.removeMarkers(this.props.autoMarkers);
        }
        if (event.type === INPUT_EVENT_TYPE.moveInputStarted &&
            !event.moveInputCallbackResult) {
            return
        }
        if (event.type === INPUT_EVENT_TYPE.moveInputStarted ||
            event.type === INPUT_EVENT_TYPE.movingOverSquare) {
            if (event.squareFrom) {
                this.addMarker(this.props.autoMarkers, event.squareFrom);
            }
            if (event.squareTo) {
                this.addMarker(this.props.autoMarkers, event.squareTo);
            }
        }
    }

    onRedrawBoard() {
        while (this.markerGroupUp.firstChild) {
            this.markerGroupUp.removeChild(this.markerGroupUp.firstChild);
        }
        while (this.markerGroupDown.firstChild) {
            this.markerGroupDown.removeChild(this.markerGroupDown.firstChild);
        }
        this.markers.forEach((marker) => {
                this.drawMarker(marker);
            }
        );
    }

    addLegalMovesMarkers(moves) {
        this.batchUpdate = true;
        try {
            for (const move of moves) {
                if (move.promotion && move.promotion !== "q") {
                    continue
                }
                if (this.chessboard.getPiece(move.to)) {
                    this.chessboard.addMarker(MARKER_TYPE.bevel, move.to);
                } else {
                    this.chessboard.addMarker(MARKER_TYPE.dot, move.to);
                }
            }
        } finally {
            this.batchUpdate = false;
            this.onRedrawBoard();
        }
    }

    removeLegalMovesMarkers() {
        this.batchUpdate = true;
        try {
            this.chessboard.removeMarkers(MARKER_TYPE.bevel);
            this.chessboard.removeMarkers(MARKER_TYPE.dot);
        } finally {
            this.batchUpdate = false;
            this.onRedrawBoard();
        }
    }

    drawMarker(marker) {
        let markerGroup;
        if (marker.type.position === 'above') {
            markerGroup = Svg.addElement(this.markerGroupUp, "g");
        } else {
            markerGroup = Svg.addElement(this.markerGroupDown, "g");
        }
        markerGroup.setAttribute("data-square", marker.square);
        const point = this.chessboard.view.squareToPoint(marker.square);
        const transform = (this.chessboard.view.svg.createSVGTransform());
        transform.setTranslate(point.x, point.y);
        markerGroup.transform.baseVal.appendItem(transform);
        const spriteUrl = this.chessboard.props.assetsCache ? "" : this.getSpriteUrl();
        const markerUse = Svg.addElement(markerGroup, "use",
            {href: `${spriteUrl}#${marker.type.slice}`, class: "marker " + marker.type.class});
        const transformScale = (this.chessboard.view.svg.createSVGTransform());
        transformScale.setScale(this.chessboard.view.scalingX, this.chessboard.view.scalingY);
        markerUse.transform.baseVal.appendItem(transformScale);
        return markerGroup
    }

    addMarker(type, square) {
        if (typeof type === "string" || typeof square === "object") { // todo remove 2022-12-01
            console.error("changed the signature of `addMarker` to `(type, square)` with v5.1.x");
            return
        }
        this.markers.push(new Marker(square, type));
        if (!this.batchUpdate) {
            this.onRedrawBoard();
        }
    }

    getMarkers(type = undefined, square = undefined) {
        if (typeof type === "string" || typeof square === "object") { // todo remove 2022-12-01
            console.error("changed the signature of `getMarkers` to `(type, square)` with v5.1.x");
            return
        }
        let markersFound = [];
        this.markers.forEach((marker) => {
            if (marker.matches(square, type)) {
                markersFound.push(marker);
            }
        });
        return markersFound
    }

    removeMarkers(type = undefined, square = undefined) {
        if (typeof type === "string" || typeof square === "object") { // todo remove 2022-12-01
            console.error("changed the signature of `removeMarkers` to `(type, square)` with v5.1.x");
            return
        }
        this.markers = this.markers.filter((marker) => !marker.matches(square, type));
        if (!this.batchUpdate) {
            this.onRedrawBoard();
        }
    }

    getSpriteUrl() {
        if(Utils.isAbsoluteUrl(this.props.sprite)) {
            return this.props.sprite
        } else {
            return this.chessboard.props.assetsUrl + this.props.sprite
        }
    }
}

class Marker {
    constructor(square, type) {
        this.square = square;
        this.type = type;
    }

    matches(square = undefined, type = undefined) {
        if (!type && !square) {
            return true
        } else if (!type) {
            if (square === this.square) {
                return true
            }
        } else if (!square) {
            if (this.type === type) {
                return true
            }
        } else if (this.type === type && square === this.square) {
            return true
        }
        return false
    }
}

/**
 * Author and copyright: Stefan Haack (https://shaack.com)
 * Repository: https://github.com/shaack/cm-chessboard
 * License: MIT, see file 'LICENSE'
 */

const DISPLAY_STATE = {
    hidden: "hidden",
    displayRequested: "displayRequested",
    shown: "shown"
};

const translations = {
    de: {
        choosePromotion: "Bauernumwandlung wählen",
        promotionDialogTitle: "Bauernumwandlung",
        pieces: {q: "Dame", r: "Turm", b: "Läufer", n: "Springer"},
        promoteTo: "Umwandeln in"
    },
    en: {
        choosePromotion: "Choose promotion piece",
        promotionDialogTitle: "Pawn promotion",
        pieces: {q: "Queen", r: "Rook", b: "Bishop", n: "Knight"},
        promoteTo: "Promote to"
    }
};

const PROMOTION_DIALOG_RESULT_TYPE = {
    pieceSelected: "pieceSelected",
    canceled: "canceled"
};

class PromotionDialog extends Extension {

    /** @constructor */
    constructor(chessboard, props = {}) {
        super(chessboard);
        this.props = {
            language: navigator.language.substring(0, 2).toLowerCase()
        };
        Object.assign(this.props, props);
        if (this.props.language !== "de" && this.props.language !== "en") {
            this.props.language = "en";
        }
        this.t = translations[this.props.language];
        this.pieceOrder = ["q", "r", "b", "n"];
        this.focusedIndex = 0;
        this.previouslyFocusedElement = null;

        this.registerExtensionPoint(EXTENSION_POINT.afterRedrawBoard, this.extensionPointRedrawBoard.bind(this));
        this.registerExtensionPoint(EXTENSION_POINT.destroy, this.destroy.bind(this));
        chessboard.showPromotionDialog = this.showPromotionDialog.bind(this);
        chessboard.isPromotionDialogShown = this.isPromotionDialogShown.bind(this);
        this.promotionDialogGroup = Svg.addElement(chessboard.view.interactiveTopLayer, "g", {
            class: "promotion-dialog-group",
            role: "dialog",
            "aria-modal": "true",
            "aria-label": this.t.choosePromotion
        });

        // Create live region for announcements
        this.liveRegion = document.createElement("div");
        this.liveRegion.setAttribute("aria-live", "polite");
        this.liveRegion.setAttribute("aria-atomic", "true");
        this.liveRegion.className = "cm-chessboard-promotion-live-region visually-hidden";
        this.liveRegion.style.cssText = "position:absolute;width:1px;height:1px;padding:0;margin:-1px;overflow:hidden;clip:rect(0,0,0,0);white-space:nowrap;border:0;";
        chessboard.context.appendChild(this.liveRegion);

        this.state = {
            displayState: DISPLAY_STATE.hidden,
            callback: null,
            dialogParams: {
                square: null,
                color: null
            }
        };

        // Bind keyboard handler
        this.handleKeyDown = this.handleKeyDown.bind(this);
    }

    // public (chessboard.showPromotionDialog)
    showPromotionDialog(square, color, callback) {
        this.previouslyFocusedElement = document.activeElement;
        this.focusedIndex = 0;
        this.state.dialogParams.square = square;
        this.state.dialogParams.color = color;
        this.state.callback = callback;
        this.setDisplayState(DISPLAY_STATE.displayRequested);
        this.showTimeoutId = setTimeout(() => {
            this.showTimeoutId = null;
            if (!this.chessboard.view) return // destroyed before timeout fired
            this.chessboard.view.positionsAnimationTask.then(() => {
                if (this.state.displayState !== DISPLAY_STATE.displayRequested) return
                this.setDisplayState(DISPLAY_STATE.shown);
                this.announce(this.t.choosePromotion + ": " +
                    this.pieceOrder.map(p => this.t.pieces[p]).join(", "));
            });
        });
    }

    // public (chessboard.isPromotionDialogShown)
    isPromotionDialogShown() {
        return this.state.displayState === DISPLAY_STATE.shown ||
            this.state.displayState === DISPLAY_STATE.displayRequested
    }

    // private
    extensionPointRedrawBoard() {
        this.redrawDialog();
    }

    drawPieceButton(piece, point, index) {
        const squareWidth = this.chessboard.view.squareWidth;
        const squareHeight = this.chessboard.view.squareHeight;
        const pieceType = piece.charAt(1);
        const pieceName = this.t.pieces[pieceType];
        const buttonGroup = Svg.addElement(this.promotionDialogGroup, "g", {
            class: "promotion-dialog-button-group",
            role: "button",
            tabindex: index === 0 ? "0" : "-1",
            "aria-label": pieceName,
            "data-piece": piece,
            "data-index": index
        });
        Svg.addElement(buttonGroup,
            "rect", {
                x: point.x, y: point.y, width: squareWidth, height: squareHeight,
                class: "promotion-dialog-button",
                "data-piece": piece
            });
        this.chessboard.view.drawPiece(buttonGroup, piece, point);
    }

    redrawDialog() {
        while (this.promotionDialogGroup.firstChild) {
            this.promotionDialogGroup.removeChild(this.promotionDialogGroup.firstChild);
        }
        if (this.state.displayState === DISPLAY_STATE.shown) {
            const squareWidth = this.chessboard.view.squareWidth;
            const squareHeight = this.chessboard.view.squareHeight;
            const squareCenterPoint = this.chessboard.view.squareToPoint(this.state.dialogParams.square);
            squareCenterPoint.x = squareCenterPoint.x + squareWidth / 2;
            squareCenterPoint.y = squareCenterPoint.y + squareHeight / 2;
            this.turned = false;
            const rank = parseInt(this.state.dialogParams.square.charAt(1), 10);
            if (this.chessboard.getOrientation() === COLOR.white && rank < 5 ||
                this.chessboard.getOrientation() === COLOR.black && rank >= 5) {
                this.turned = true;
            }
            const turned = this.turned;
            const offsetY = turned ? -4 * squareHeight : 0;
            const offsetX = squareCenterPoint.x + squareWidth > this.chessboard.view.width ? -squareWidth : 0;
            Svg.addElement(this.promotionDialogGroup,
                "rect", {
                    x: squareCenterPoint.x + offsetX,
                    y: squareCenterPoint.y + offsetY,
                    width: squareWidth,
                    height: squareHeight * 4,
                    class: "promotion-dialog"
                });
            const dialogParams = this.state.dialogParams;
            if (turned) {
                this.drawPieceButton(PIECE[dialogParams.color + "q"], {
                    x: squareCenterPoint.x + offsetX,
                    y: squareCenterPoint.y - squareHeight
                }, 0);
                this.drawPieceButton(PIECE[dialogParams.color + "r"], {
                    x: squareCenterPoint.x + offsetX,
                    y: squareCenterPoint.y - squareHeight * 2
                }, 1);
                this.drawPieceButton(PIECE[dialogParams.color + "b"], {
                    x: squareCenterPoint.x + offsetX,
                    y: squareCenterPoint.y - squareHeight * 3
                }, 2);
                this.drawPieceButton(PIECE[dialogParams.color + "n"], {
                    x: squareCenterPoint.x + offsetX,
                    y: squareCenterPoint.y - squareHeight * 4
                }, 3);
            } else {
                this.drawPieceButton(PIECE[dialogParams.color + "q"], {
                    x: squareCenterPoint.x + offsetX,
                    y: squareCenterPoint.y
                }, 0);
                this.drawPieceButton(PIECE[dialogParams.color + "r"], {
                    x: squareCenterPoint.x + offsetX,
                    y: squareCenterPoint.y + squareHeight
                }, 1);
                this.drawPieceButton(PIECE[dialogParams.color + "b"], {
                    x: squareCenterPoint.x + offsetX,
                    y: squareCenterPoint.y + squareHeight * 2
                }, 2);
                this.drawPieceButton(PIECE[dialogParams.color + "n"], {
                    x: squareCenterPoint.x + offsetX,
                    y: squareCenterPoint.y + squareHeight * 3
                }, 3);
            }
        }
    }

    promotionDialogOnClickPiece(event) {
        if (event.button !== 2) {
            // Find piece data from target or parent button group
            let piece = event.target.dataset.piece;
            if (!piece && event.target.closest) {
                const buttonGroup = event.target.closest(".promotion-dialog-button-group");
                if (buttonGroup) {
                    piece = buttonGroup.dataset.piece;
                }
            }
            if (piece) {
                this.selectPiece(piece);
            } else {
                this.promotionDialogOnCancel(event);
            }
        }
    }

    selectPiece(piece) {
        if (this.state.callback) {
            this.state.callback({
                type: PROMOTION_DIALOG_RESULT_TYPE.pieceSelected,
                square: this.state.dialogParams.square,
                piece: piece
            });
        }
        this.setDisplayState(DISPLAY_STATE.hidden);
    }

    promotionDialogOnCancel(event) {
        if (this.state.displayState === DISPLAY_STATE.shown) {
            event.preventDefault();
            this.setDisplayState(DISPLAY_STATE.hidden);
            if(this.state.callback) {
                this.state.callback({type: PROMOTION_DIALOG_RESULT_TYPE.canceled});
            }
        }
    }

    contextMenu(event) {
        event.preventDefault();
        this.setDisplayState(DISPLAY_STATE.hidden);
        if(this.state.callback) {
            this.state.callback({type: PROMOTION_DIALOG_RESULT_TYPE.canceled});
        }
    }

    setDisplayState(displayState) {
        const prevState = this.state.displayState;
        this.state.displayState = displayState;
        if (displayState === DISPLAY_STATE.shown) {
            this.clickDelegate = Utils.delegate(this.chessboard.view.svg,
                "pointerdown",
                "*",
                this.promotionDialogOnClickPiece.bind(this));
            this.contextMenuListener = this.contextMenu.bind(this);
            this.chessboard.view.svg.addEventListener("contextmenu", this.contextMenuListener);
            // Add keyboard listener
            document.addEventListener("keydown", this.handleKeyDown);
        } else if (displayState === DISPLAY_STATE.hidden) {
            if (this.clickDelegate) {
                this.clickDelegate.remove();
                this.clickDelegate = null;
            }
            if (this.contextMenuListener && this.chessboard.view) {
                this.chessboard.view.svg.removeEventListener("contextmenu", this.contextMenuListener);
                this.contextMenuListener = null;
            }
            // Remove keyboard listener
            document.removeEventListener("keydown", this.handleKeyDown);
            // Restore focus (only if the dialog was actually shown before)
            if (prevState === DISPLAY_STATE.shown &&
                this.previouslyFocusedElement && this.previouslyFocusedElement.focus) {
                this.previouslyFocusedElement.focus();
            }
        }
        this.redrawDialog();
        // Focus first button after redraw when shown
        if (displayState === DISPLAY_STATE.shown) {
            this.focusTimeoutId = setTimeout(() => {
                this.focusTimeoutId = null;
                this.focusButton(0);
            }, 0);
        }
    }

    handleKeyDown(event) {
        if (this.state.displayState !== DISPLAY_STATE.shown) {
            return
        }
        switch (event.key) {
            case "ArrowDown":
                event.preventDefault();
                if (this.turned) {
                    this.focusedIndex = (this.focusedIndex - 1 + 4) % 4;
                } else {
                    this.focusedIndex = (this.focusedIndex + 1) % 4;
                }
                this.focusButton(this.focusedIndex);
                break
            case "ArrowRight":
                event.preventDefault();
                this.focusedIndex = (this.focusedIndex + 1) % 4;
                this.focusButton(this.focusedIndex);
                break
            case "ArrowUp":
                event.preventDefault();
                if (this.turned) {
                    this.focusedIndex = (this.focusedIndex + 1) % 4;
                } else {
                    this.focusedIndex = (this.focusedIndex - 1 + 4) % 4;
                }
                this.focusButton(this.focusedIndex);
                break
            case "ArrowLeft":
                event.preventDefault();
                this.focusedIndex = (this.focusedIndex - 1 + 4) % 4;
                this.focusButton(this.focusedIndex);
                break
            case "Enter":
            case " ":
                event.preventDefault();
                const buttons = this.promotionDialogGroup.querySelectorAll(".promotion-dialog-button-group");
                if (buttons[this.focusedIndex]) {
                    const piece = buttons[this.focusedIndex].dataset.piece;
                    this.selectPiece(piece);
                }
                break
            case "Escape":
                event.preventDefault();
                this.setDisplayState(DISPLAY_STATE.hidden);
                if (this.state.callback) {
                    this.state.callback({type: PROMOTION_DIALOG_RESULT_TYPE.canceled});
                }
                break
            case "Tab":
                // Trap focus within dialog
                event.preventDefault();
                if (event.shiftKey) {
                    this.focusedIndex = (this.focusedIndex - 1 + 4) % 4;
                } else {
                    this.focusedIndex = (this.focusedIndex + 1) % 4;
                }
                this.focusButton(this.focusedIndex);
                break
        }
    }

    focusButton(index) {
        const buttons = this.promotionDialogGroup.querySelectorAll(".promotion-dialog-button-group");
        buttons.forEach((btn, i) => {
            btn.setAttribute("tabindex", i === index ? "0" : "-1");
        });
        if (buttons[index]) {
            buttons[index].focus();
            const pieceType = this.pieceOrder[index];
            this.announce(this.t.pieces[pieceType]);
        }
    }

    announce(message) {
        if (!this.liveRegion) return
        this.liveRegion.textContent = "";
        if (this.announceTimeoutId) {
            clearTimeout(this.announceTimeoutId);
        }
        // Small delay to ensure screen readers pick up the change
        this.announceTimeoutId = setTimeout(() => {
            this.announceTimeoutId = null;
            if (this.liveRegion) {
                this.liveRegion.textContent = message;
            }
        }, 50);
    }

    destroy() {
        // Close the dialog first so its listeners are removed
        if (this.state.displayState === DISPLAY_STATE.shown) {
            this.setDisplayState(DISPLAY_STATE.hidden);
        }
        // Cancel any pending timeouts so callbacks don't fire on a destroyed board
        if (this.showTimeoutId) {
            clearTimeout(this.showTimeoutId);
            this.showTimeoutId = null;
        }
        if (this.focusTimeoutId) {
            clearTimeout(this.focusTimeoutId);
            this.focusTimeoutId = null;
        }
        if (this.announceTimeoutId) {
            clearTimeout(this.announceTimeoutId);
            this.announceTimeoutId = null;
        }
        document.removeEventListener("keydown", this.handleKeyDown);
        if (this.liveRegion && this.liveRegion.parentNode) {
            this.liveRegion.parentNode.removeChild(this.liveRegion);
            this.liveRegion = null;
        }
        delete this.chessboard.showPromotionDialog;
        delete this.chessboard.isPromotionDialogShown;
    }

}

// @generated by Peggy 4.2.0.
//
// https://peggyjs.org/



  function rootNode(comment) {
    return comment !== null ? { comment, variations: [] } : { variations: []}
  }

  function node(move, suffix, nag, comment, variations) {
    const node = { move, variations };

    if (suffix) {
      node.suffix = suffix;
    }

    if (nag) {
      node.nag = nag;
    }

    if (comment !== null) {
      node.comment = comment;
    }

    return node
  }

  function lineToTree(...nodes) {
    const [root, ...rest] = nodes;

    let parent = root;

    for (const child of rest) {
      if (child !== null) {
          parent.variations = [child, ...child.variations];
            child.variations = [];
            parent = child;
        }
    }

    return root
  }

  function pgn(headers, game) {
    if (game.marker && game.marker.comment) {
      let node = game.root;
        while (true) {
          const next = node.variations[0];
            if (!next) {
              node.comment = game.marker.comment;
              break
            }
            node = next;
        }
    }

    return {
      headers,
        root: game.root,
        result: (game.marker && game.marker.result) ?? undefined
    }
  }

function peg$subclass(child, parent) {
  function C() { this.constructor = child; }
  C.prototype = parent.prototype;
  child.prototype = new C();
}

function peg$SyntaxError(message, expected, found, location) {
  var self = Error.call(this, message);
  // istanbul ignore next Check is a necessary evil to support older environments
  if (Object.setPrototypeOf) {
    Object.setPrototypeOf(self, peg$SyntaxError.prototype);
  }
  self.expected = expected;
  self.found = found;
  self.location = location;
  self.name = "SyntaxError";
  return self;
}

peg$subclass(peg$SyntaxError, Error);

function peg$padEnd(str, targetLength, padString) {
  padString = padString || " ";
  if (str.length > targetLength) { return str; }
  targetLength -= str.length;
  padString += padString.repeat(targetLength);
  return str + padString.slice(0, targetLength);
}

peg$SyntaxError.prototype.format = function(sources) {
  var str = "Error: " + this.message;
  if (this.location) {
    var src = null;
    var k;
    for (k = 0; k < sources.length; k++) {
      if (sources[k].source === this.location.source) {
        src = sources[k].text.split(/\r\n|\n|\r/g);
        break;
      }
    }
    var s = this.location.start;
    var offset_s = (this.location.source && (typeof this.location.source.offset === "function"))
      ? this.location.source.offset(s)
      : s;
    var loc = this.location.source + ":" + offset_s.line + ":" + offset_s.column;
    if (src) {
      var e = this.location.end;
      var filler = peg$padEnd("", offset_s.line.toString().length, ' ');
      var line = src[s.line - 1];
      var last = s.line === e.line ? e.column : line.length + 1;
      var hatLen = (last - s.column) || 1;
      str += "\n --> " + loc + "\n"
          + filler + " |\n"
          + offset_s.line + " | " + line + "\n"
          + filler + " | " + peg$padEnd("", s.column - 1, ' ')
          + peg$padEnd("", hatLen, "^");
    } else {
      str += "\n at " + loc;
    }
  }
  return str;
};

peg$SyntaxError.buildMessage = function(expected, found) {
  var DESCRIBE_EXPECTATION_FNS = {
    literal: function(expectation) {
      return "\"" + literalEscape(expectation.text) + "\"";
    },

    class: function(expectation) {
      var escapedParts = expectation.parts.map(function(part) {
        return Array.isArray(part)
          ? classEscape(part[0]) + "-" + classEscape(part[1])
          : classEscape(part);
      });

      return "[" + (expectation.inverted ? "^" : "") + escapedParts.join("") + "]";
    },

    any: function() {
      return "any character";
    },

    end: function() {
      return "end of input";
    },

    other: function(expectation) {
      return expectation.description;
    }
  };

  function hex(ch) {
    return ch.charCodeAt(0).toString(16).toUpperCase();
  }

  function literalEscape(s) {
    return s
      .replace(/\\/g, "\\\\")
      .replace(/"/g,  "\\\"")
      .replace(/\0/g, "\\0")
      .replace(/\t/g, "\\t")
      .replace(/\n/g, "\\n")
      .replace(/\r/g, "\\r")
      .replace(/[\x00-\x0F]/g,          function(ch) { return "\\x0" + hex(ch); })
      .replace(/[\x10-\x1F\x7F-\x9F]/g, function(ch) { return "\\x"  + hex(ch); });
  }

  function classEscape(s) {
    return s
      .replace(/\\/g, "\\\\")
      .replace(/\]/g, "\\]")
      .replace(/\^/g, "\\^")
      .replace(/-/g,  "\\-")
      .replace(/\0/g, "\\0")
      .replace(/\t/g, "\\t")
      .replace(/\n/g, "\\n")
      .replace(/\r/g, "\\r")
      .replace(/[\x00-\x0F]/g,          function(ch) { return "\\x0" + hex(ch); })
      .replace(/[\x10-\x1F\x7F-\x9F]/g, function(ch) { return "\\x"  + hex(ch); });
  }

  function describeExpectation(expectation) {
    return DESCRIBE_EXPECTATION_FNS[expectation.type](expectation);
  }

  function describeExpected(expected) {
    var descriptions = expected.map(describeExpectation);
    var i, j;

    descriptions.sort();

    if (descriptions.length > 0) {
      for (i = 1, j = 1; i < descriptions.length; i++) {
        if (descriptions[i - 1] !== descriptions[i]) {
          descriptions[j] = descriptions[i];
          j++;
        }
      }
      descriptions.length = j;
    }

    switch (descriptions.length) {
      case 1:
        return descriptions[0];

      case 2:
        return descriptions[0] + " or " + descriptions[1];

      default:
        return descriptions.slice(0, -1).join(", ")
          + ", or "
          + descriptions[descriptions.length - 1];
    }
  }

  function describeFound(found) {
    return found ? "\"" + literalEscape(found) + "\"" : "end of input";
  }

  return "Expected " + describeExpected(expected) + " but " + describeFound(found) + " found.";
};

function peg$parse(input, options) {
  options = options !== undefined ? options : {};

  var peg$FAILED = {};
  var peg$source = options.grammarSource;

  var peg$startRuleFunctions = { pgn: peg$parsepgn };
  var peg$startRuleFunction = peg$parsepgn;

  var peg$c0 = "[";
  var peg$c1 = "\"";
  var peg$c2 = "]";
  var peg$c3 = ".";
  var peg$c4 = "O-O-O";
  var peg$c5 = "O-O";
  var peg$c6 = "0-0-0";
  var peg$c7 = "0-0";
  var peg$c8 = "$";
  var peg$c9 = "{";
  var peg$c10 = "}";
  var peg$c11 = ";";
  var peg$c12 = "(";
  var peg$c13 = ")";
  var peg$c14 = "1-0";
  var peg$c15 = "0-1";
  var peg$c16 = "1/2-1/2";
  var peg$c17 = "*";

  var peg$r0 = /^[a-zA-Z]/;
  var peg$r1 = /^[^"]/;
  var peg$r2 = /^[0-9]/;
  var peg$r3 = /^[.]/;
  var peg$r4 = /^[a-zA-Z1-8\-=]/;
  var peg$r5 = /^[+#]/;
  var peg$r6 = /^[!?]/;
  var peg$r7 = /^[^}]/;
  var peg$r8 = /^[^\r\n]/;
  var peg$r9 = /^[ \t\r\n]/;

  var peg$e0 = peg$otherExpectation("tag pair");
  var peg$e1 = peg$literalExpectation("[", false);
  var peg$e2 = peg$literalExpectation("\"", false);
  var peg$e3 = peg$literalExpectation("]", false);
  var peg$e4 = peg$otherExpectation("tag name");
  var peg$e5 = peg$classExpectation([["a", "z"], ["A", "Z"]], false, false);
  var peg$e6 = peg$otherExpectation("tag value");
  var peg$e7 = peg$classExpectation(["\""], true, false);
  var peg$e8 = peg$otherExpectation("move number");
  var peg$e9 = peg$classExpectation([["0", "9"]], false, false);
  var peg$e10 = peg$literalExpectation(".", false);
  var peg$e11 = peg$classExpectation(["."], false, false);
  var peg$e12 = peg$otherExpectation("standard algebraic notation");
  var peg$e13 = peg$literalExpectation("O-O-O", false);
  var peg$e14 = peg$literalExpectation("O-O", false);
  var peg$e15 = peg$literalExpectation("0-0-0", false);
  var peg$e16 = peg$literalExpectation("0-0", false);
  var peg$e17 = peg$classExpectation([["a", "z"], ["A", "Z"], ["1", "8"], "-", "="], false, false);
  var peg$e18 = peg$classExpectation(["+", "#"], false, false);
  var peg$e19 = peg$otherExpectation("suffix annotation");
  var peg$e20 = peg$classExpectation(["!", "?"], false, false);
  var peg$e21 = peg$otherExpectation("NAG");
  var peg$e22 = peg$literalExpectation("$", false);
  var peg$e23 = peg$otherExpectation("brace comment");
  var peg$e24 = peg$literalExpectation("{", false);
  var peg$e25 = peg$classExpectation(["}"], true, false);
  var peg$e26 = peg$literalExpectation("}", false);
  var peg$e27 = peg$otherExpectation("rest of line comment");
  var peg$e28 = peg$literalExpectation(";", false);
  var peg$e29 = peg$classExpectation(["\r", "\n"], true, false);
  var peg$e30 = peg$otherExpectation("variation");
  var peg$e31 = peg$literalExpectation("(", false);
  var peg$e32 = peg$literalExpectation(")", false);
  var peg$e33 = peg$otherExpectation("game termination marker");
  var peg$e34 = peg$literalExpectation("1-0", false);
  var peg$e35 = peg$literalExpectation("0-1", false);
  var peg$e36 = peg$literalExpectation("1/2-1/2", false);
  var peg$e37 = peg$literalExpectation("*", false);
  var peg$e38 = peg$otherExpectation("whitespace");
  var peg$e39 = peg$classExpectation([" ", "\t", "\r", "\n"], false, false);

  var peg$f0 = function(headers, game) { return pgn(headers, game) };
  var peg$f1 = function(tagPairs) { return Object.fromEntries(tagPairs) };
  var peg$f2 = function(tagName, tagValue) { return [tagName, tagValue] };
  var peg$f3 = function(root, marker) { return { root, marker} };
  var peg$f4 = function(comment, moves) { return lineToTree(rootNode(comment), ...moves.flat()) };
  var peg$f5 = function(san, suffix, nag, comment, variations) { return node(san, suffix, nag, comment, variations) };
  var peg$f6 = function(nag) { return nag };
  var peg$f7 = function(comment) { return comment.replace(/[\r\n]+/g, " ") };
  var peg$f8 = function(comment) { return comment.trim() };
  var peg$f9 = function(line) { return line };
  var peg$f10 = function(result, comment) { return { result, comment } };
  var peg$currPos = options.peg$currPos | 0;
  var peg$posDetailsCache = [{ line: 1, column: 1 }];
  var peg$maxFailPos = peg$currPos;
  var peg$maxFailExpected = options.peg$maxFailExpected || [];
  var peg$silentFails = options.peg$silentFails | 0;

  var peg$result;

  if (options.startRule) {
    if (!(options.startRule in peg$startRuleFunctions)) {
      throw new Error("Can't start parsing from rule \"" + options.startRule + "\".");
    }

    peg$startRuleFunction = peg$startRuleFunctions[options.startRule];
  }

  function peg$literalExpectation(text, ignoreCase) {
    return { type: "literal", text: text, ignoreCase: ignoreCase };
  }

  function peg$classExpectation(parts, inverted, ignoreCase) {
    return { type: "class", parts: parts, inverted: inverted, ignoreCase: ignoreCase };
  }

  function peg$endExpectation() {
    return { type: "end" };
  }

  function peg$otherExpectation(description) {
    return { type: "other", description: description };
  }

  function peg$computePosDetails(pos) {
    var details = peg$posDetailsCache[pos];
    var p;

    if (details) {
      return details;
    } else {
      if (pos >= peg$posDetailsCache.length) {
        p = peg$posDetailsCache.length - 1;
      } else {
        p = pos;
        while (!peg$posDetailsCache[--p]) {}
      }

      details = peg$posDetailsCache[p];
      details = {
        line: details.line,
        column: details.column
      };

      while (p < pos) {
        if (input.charCodeAt(p) === 10) {
          details.line++;
          details.column = 1;
        } else {
          details.column++;
        }

        p++;
      }

      peg$posDetailsCache[pos] = details;

      return details;
    }
  }

  function peg$computeLocation(startPos, endPos, offset) {
    var startPosDetails = peg$computePosDetails(startPos);
    var endPosDetails = peg$computePosDetails(endPos);

    var res = {
      source: peg$source,
      start: {
        offset: startPos,
        line: startPosDetails.line,
        column: startPosDetails.column
      },
      end: {
        offset: endPos,
        line: endPosDetails.line,
        column: endPosDetails.column
      }
    };
    return res;
  }

  function peg$fail(expected) {
    if (peg$currPos < peg$maxFailPos) { return; }

    if (peg$currPos > peg$maxFailPos) {
      peg$maxFailPos = peg$currPos;
      peg$maxFailExpected = [];
    }

    peg$maxFailExpected.push(expected);
  }

  function peg$buildStructuredError(expected, found, location) {
    return new peg$SyntaxError(
      peg$SyntaxError.buildMessage(expected, found),
      expected,
      found,
      location
    );
  }

  function peg$parsepgn() {
    var s0, s1, s2;

    s0 = peg$currPos;
    s1 = peg$parsetagPairSection();
    s2 = peg$parsemoveTextSection();
    s0 = peg$f0(s1, s2);

    return s0;
  }

  function peg$parsetagPairSection() {
    var s0, s1, s2;

    s0 = peg$currPos;
    s1 = [];
    s2 = peg$parsetagPair();
    while (s2 !== peg$FAILED) {
      s1.push(s2);
      s2 = peg$parsetagPair();
    }
    s2 = peg$parse_();
    s0 = peg$f1(s1);

    return s0;
  }

  function peg$parsetagPair() {
    var s0, s2, s4, s6, s7, s8, s10;

    peg$silentFails++;
    s0 = peg$currPos;
    peg$parse_();
    if (input.charCodeAt(peg$currPos) === 91) {
      s2 = peg$c0;
      peg$currPos++;
    } else {
      s2 = peg$FAILED;
      if (peg$silentFails === 0) { peg$fail(peg$e1); }
    }
    if (s2 !== peg$FAILED) {
      peg$parse_();
      s4 = peg$parsetagName();
      if (s4 !== peg$FAILED) {
        peg$parse_();
        if (input.charCodeAt(peg$currPos) === 34) {
          s6 = peg$c1;
          peg$currPos++;
        } else {
          s6 = peg$FAILED;
          if (peg$silentFails === 0) { peg$fail(peg$e2); }
        }
        if (s6 !== peg$FAILED) {
          s7 = peg$parsetagValue();
          if (input.charCodeAt(peg$currPos) === 34) {
            s8 = peg$c1;
            peg$currPos++;
          } else {
            s8 = peg$FAILED;
            if (peg$silentFails === 0) { peg$fail(peg$e2); }
          }
          if (s8 !== peg$FAILED) {
            peg$parse_();
            if (input.charCodeAt(peg$currPos) === 93) {
              s10 = peg$c2;
              peg$currPos++;
            } else {
              s10 = peg$FAILED;
              if (peg$silentFails === 0) { peg$fail(peg$e3); }
            }
            if (s10 !== peg$FAILED) {
              s0 = peg$f2(s4, s7);
            } else {
              peg$currPos = s0;
              s0 = peg$FAILED;
            }
          } else {
            peg$currPos = s0;
            s0 = peg$FAILED;
          }
        } else {
          peg$currPos = s0;
          s0 = peg$FAILED;
        }
      } else {
        peg$currPos = s0;
        s0 = peg$FAILED;
      }
    } else {
      peg$currPos = s0;
      s0 = peg$FAILED;
    }
    peg$silentFails--;
    if (s0 === peg$FAILED) {
      if (peg$silentFails === 0) { peg$fail(peg$e0); }
    }

    return s0;
  }

  function peg$parsetagName() {
    var s0, s1, s2;

    peg$silentFails++;
    s0 = peg$currPos;
    s1 = [];
    s2 = input.charAt(peg$currPos);
    if (peg$r0.test(s2)) {
      peg$currPos++;
    } else {
      s2 = peg$FAILED;
      if (peg$silentFails === 0) { peg$fail(peg$e5); }
    }
    if (s2 !== peg$FAILED) {
      while (s2 !== peg$FAILED) {
        s1.push(s2);
        s2 = input.charAt(peg$currPos);
        if (peg$r0.test(s2)) {
          peg$currPos++;
        } else {
          s2 = peg$FAILED;
          if (peg$silentFails === 0) { peg$fail(peg$e5); }
        }
      }
    } else {
      s1 = peg$FAILED;
    }
    if (s1 !== peg$FAILED) {
      s0 = input.substring(s0, peg$currPos);
    } else {
      s0 = s1;
    }
    peg$silentFails--;
    if (s0 === peg$FAILED) {
      s1 = peg$FAILED;
      if (peg$silentFails === 0) { peg$fail(peg$e4); }
    }

    return s0;
  }

  function peg$parsetagValue() {
    var s0, s1, s2;

    peg$silentFails++;
    s0 = peg$currPos;
    s1 = [];
    s2 = input.charAt(peg$currPos);
    if (peg$r1.test(s2)) {
      peg$currPos++;
    } else {
      s2 = peg$FAILED;
      if (peg$silentFails === 0) { peg$fail(peg$e7); }
    }
    while (s2 !== peg$FAILED) {
      s1.push(s2);
      s2 = input.charAt(peg$currPos);
      if (peg$r1.test(s2)) {
        peg$currPos++;
      } else {
        s2 = peg$FAILED;
        if (peg$silentFails === 0) { peg$fail(peg$e7); }
      }
    }
    s0 = input.substring(s0, peg$currPos);
    peg$silentFails--;
    s1 = peg$FAILED;
    if (peg$silentFails === 0) { peg$fail(peg$e6); }

    return s0;
  }

  function peg$parsemoveTextSection() {
    var s0, s1, s3;

    s0 = peg$currPos;
    s1 = peg$parseline();
    peg$parse_();
    s3 = peg$parsegameTerminationMarker();
    if (s3 === peg$FAILED) {
      s3 = null;
    }
    peg$parse_();
    s0 = peg$f3(s1, s3);

    return s0;
  }

  function peg$parseline() {
    var s0, s1, s2, s3;

    s0 = peg$currPos;
    s1 = peg$parsecomment();
    if (s1 === peg$FAILED) {
      s1 = null;
    }
    s2 = [];
    s3 = peg$parsemove();
    while (s3 !== peg$FAILED) {
      s2.push(s3);
      s3 = peg$parsemove();
    }
    s0 = peg$f4(s1, s2);

    return s0;
  }

  function peg$parsemove() {
    var s0, s4, s5, s6, s7, s8, s9, s10;

    s0 = peg$currPos;
    peg$parse_();
    peg$parsemoveNumber();
    peg$parse_();
    s4 = peg$parsesan();
    if (s4 !== peg$FAILED) {
      s5 = peg$parsesuffixAnnotation();
      if (s5 === peg$FAILED) {
        s5 = null;
      }
      s6 = [];
      s7 = peg$parsenag();
      while (s7 !== peg$FAILED) {
        s6.push(s7);
        s7 = peg$parsenag();
      }
      s7 = peg$parse_();
      s8 = peg$parsecomment();
      if (s8 === peg$FAILED) {
        s8 = null;
      }
      s9 = [];
      s10 = peg$parsevariation();
      while (s10 !== peg$FAILED) {
        s9.push(s10);
        s10 = peg$parsevariation();
      }
      s0 = peg$f5(s4, s5, s6, s8, s9);
    } else {
      peg$currPos = s0;
      s0 = peg$FAILED;
    }

    return s0;
  }

  function peg$parsemoveNumber() {
    var s0, s1, s2, s3, s4, s5;

    peg$silentFails++;
    s0 = peg$currPos;
    s1 = [];
    s2 = input.charAt(peg$currPos);
    if (peg$r2.test(s2)) {
      peg$currPos++;
    } else {
      s2 = peg$FAILED;
      if (peg$silentFails === 0) { peg$fail(peg$e9); }
    }
    while (s2 !== peg$FAILED) {
      s1.push(s2);
      s2 = input.charAt(peg$currPos);
      if (peg$r2.test(s2)) {
        peg$currPos++;
      } else {
        s2 = peg$FAILED;
        if (peg$silentFails === 0) { peg$fail(peg$e9); }
      }
    }
    if (input.charCodeAt(peg$currPos) === 46) {
      s2 = peg$c3;
      peg$currPos++;
    } else {
      s2 = peg$FAILED;
      if (peg$silentFails === 0) { peg$fail(peg$e10); }
    }
    if (s2 !== peg$FAILED) {
      s3 = peg$parse_();
      s4 = [];
      s5 = input.charAt(peg$currPos);
      if (peg$r3.test(s5)) {
        peg$currPos++;
      } else {
        s5 = peg$FAILED;
        if (peg$silentFails === 0) { peg$fail(peg$e11); }
      }
      while (s5 !== peg$FAILED) {
        s4.push(s5);
        s5 = input.charAt(peg$currPos);
        if (peg$r3.test(s5)) {
          peg$currPos++;
        } else {
          s5 = peg$FAILED;
          if (peg$silentFails === 0) { peg$fail(peg$e11); }
        }
      }
      s1 = [s1, s2, s3, s4];
      s0 = s1;
    } else {
      peg$currPos = s0;
      s0 = peg$FAILED;
    }
    peg$silentFails--;
    if (s0 === peg$FAILED) {
      s1 = peg$FAILED;
      if (peg$silentFails === 0) { peg$fail(peg$e8); }
    }

    return s0;
  }

  function peg$parsesan() {
    var s0, s1, s2, s3, s4, s5;

    peg$silentFails++;
    s0 = peg$currPos;
    s1 = peg$currPos;
    if (input.substr(peg$currPos, 5) === peg$c4) {
      s2 = peg$c4;
      peg$currPos += 5;
    } else {
      s2 = peg$FAILED;
      if (peg$silentFails === 0) { peg$fail(peg$e13); }
    }
    if (s2 === peg$FAILED) {
      if (input.substr(peg$currPos, 3) === peg$c5) {
        s2 = peg$c5;
        peg$currPos += 3;
      } else {
        s2 = peg$FAILED;
        if (peg$silentFails === 0) { peg$fail(peg$e14); }
      }
      if (s2 === peg$FAILED) {
        if (input.substr(peg$currPos, 5) === peg$c6) {
          s2 = peg$c6;
          peg$currPos += 5;
        } else {
          s2 = peg$FAILED;
          if (peg$silentFails === 0) { peg$fail(peg$e15); }
        }
        if (s2 === peg$FAILED) {
          if (input.substr(peg$currPos, 3) === peg$c7) {
            s2 = peg$c7;
            peg$currPos += 3;
          } else {
            s2 = peg$FAILED;
            if (peg$silentFails === 0) { peg$fail(peg$e16); }
          }
          if (s2 === peg$FAILED) {
            s2 = peg$currPos;
            s3 = input.charAt(peg$currPos);
            if (peg$r0.test(s3)) {
              peg$currPos++;
            } else {
              s3 = peg$FAILED;
              if (peg$silentFails === 0) { peg$fail(peg$e5); }
            }
            if (s3 !== peg$FAILED) {
              s4 = [];
              s5 = input.charAt(peg$currPos);
              if (peg$r4.test(s5)) {
                peg$currPos++;
              } else {
                s5 = peg$FAILED;
                if (peg$silentFails === 0) { peg$fail(peg$e17); }
              }
              if (s5 !== peg$FAILED) {
                while (s5 !== peg$FAILED) {
                  s4.push(s5);
                  s5 = input.charAt(peg$currPos);
                  if (peg$r4.test(s5)) {
                    peg$currPos++;
                  } else {
                    s5 = peg$FAILED;
                    if (peg$silentFails === 0) { peg$fail(peg$e17); }
                  }
                }
              } else {
                s4 = peg$FAILED;
              }
              if (s4 !== peg$FAILED) {
                s3 = [s3, s4];
                s2 = s3;
              } else {
                peg$currPos = s2;
                s2 = peg$FAILED;
              }
            } else {
              peg$currPos = s2;
              s2 = peg$FAILED;
            }
          }
        }
      }
    }
    if (s2 !== peg$FAILED) {
      s3 = input.charAt(peg$currPos);
      if (peg$r5.test(s3)) {
        peg$currPos++;
      } else {
        s3 = peg$FAILED;
        if (peg$silentFails === 0) { peg$fail(peg$e18); }
      }
      if (s3 === peg$FAILED) {
        s3 = null;
      }
      s2 = [s2, s3];
      s1 = s2;
    } else {
      peg$currPos = s1;
      s1 = peg$FAILED;
    }
    if (s1 !== peg$FAILED) {
      s0 = input.substring(s0, peg$currPos);
    } else {
      s0 = s1;
    }
    peg$silentFails--;
    if (s0 === peg$FAILED) {
      s1 = peg$FAILED;
      if (peg$silentFails === 0) { peg$fail(peg$e12); }
    }

    return s0;
  }

  function peg$parsesuffixAnnotation() {
    var s0, s1, s2;

    peg$silentFails++;
    s0 = peg$currPos;
    s1 = [];
    s2 = input.charAt(peg$currPos);
    if (peg$r6.test(s2)) {
      peg$currPos++;
    } else {
      s2 = peg$FAILED;
      if (peg$silentFails === 0) { peg$fail(peg$e20); }
    }
    while (s2 !== peg$FAILED) {
      s1.push(s2);
      if (s1.length >= 2) {
        s2 = peg$FAILED;
      } else {
        s2 = input.charAt(peg$currPos);
        if (peg$r6.test(s2)) {
          peg$currPos++;
        } else {
          s2 = peg$FAILED;
          if (peg$silentFails === 0) { peg$fail(peg$e20); }
        }
      }
    }
    if (s1.length < 1) {
      peg$currPos = s0;
      s0 = peg$FAILED;
    } else {
      s0 = s1;
    }
    peg$silentFails--;
    if (s0 === peg$FAILED) {
      s1 = peg$FAILED;
      if (peg$silentFails === 0) { peg$fail(peg$e19); }
    }

    return s0;
  }

  function peg$parsenag() {
    var s0, s2, s3, s4, s5;

    peg$silentFails++;
    s0 = peg$currPos;
    peg$parse_();
    if (input.charCodeAt(peg$currPos) === 36) {
      s2 = peg$c8;
      peg$currPos++;
    } else {
      s2 = peg$FAILED;
      if (peg$silentFails === 0) { peg$fail(peg$e22); }
    }
    if (s2 !== peg$FAILED) {
      s3 = peg$currPos;
      s4 = [];
      s5 = input.charAt(peg$currPos);
      if (peg$r2.test(s5)) {
        peg$currPos++;
      } else {
        s5 = peg$FAILED;
        if (peg$silentFails === 0) { peg$fail(peg$e9); }
      }
      if (s5 !== peg$FAILED) {
        while (s5 !== peg$FAILED) {
          s4.push(s5);
          s5 = input.charAt(peg$currPos);
          if (peg$r2.test(s5)) {
            peg$currPos++;
          } else {
            s5 = peg$FAILED;
            if (peg$silentFails === 0) { peg$fail(peg$e9); }
          }
        }
      } else {
        s4 = peg$FAILED;
      }
      if (s4 !== peg$FAILED) {
        s3 = input.substring(s3, peg$currPos);
      } else {
        s3 = s4;
      }
      if (s3 !== peg$FAILED) {
        s0 = peg$f6(s3);
      } else {
        peg$currPos = s0;
        s0 = peg$FAILED;
      }
    } else {
      peg$currPos = s0;
      s0 = peg$FAILED;
    }
    peg$silentFails--;
    if (s0 === peg$FAILED) {
      if (peg$silentFails === 0) { peg$fail(peg$e21); }
    }

    return s0;
  }

  function peg$parsecomment() {
    var s0;

    s0 = peg$parsebraceComment();
    if (s0 === peg$FAILED) {
      s0 = peg$parserestOfLineComment();
    }

    return s0;
  }

  function peg$parsebraceComment() {
    var s0, s1, s2, s3, s4;

    peg$silentFails++;
    s0 = peg$currPos;
    if (input.charCodeAt(peg$currPos) === 123) {
      s1 = peg$c9;
      peg$currPos++;
    } else {
      s1 = peg$FAILED;
      if (peg$silentFails === 0) { peg$fail(peg$e24); }
    }
    if (s1 !== peg$FAILED) {
      s2 = peg$currPos;
      s3 = [];
      s4 = input.charAt(peg$currPos);
      if (peg$r7.test(s4)) {
        peg$currPos++;
      } else {
        s4 = peg$FAILED;
        if (peg$silentFails === 0) { peg$fail(peg$e25); }
      }
      while (s4 !== peg$FAILED) {
        s3.push(s4);
        s4 = input.charAt(peg$currPos);
        if (peg$r7.test(s4)) {
          peg$currPos++;
        } else {
          s4 = peg$FAILED;
          if (peg$silentFails === 0) { peg$fail(peg$e25); }
        }
      }
      s2 = input.substring(s2, peg$currPos);
      if (input.charCodeAt(peg$currPos) === 125) {
        s3 = peg$c10;
        peg$currPos++;
      } else {
        s3 = peg$FAILED;
        if (peg$silentFails === 0) { peg$fail(peg$e26); }
      }
      if (s3 !== peg$FAILED) {
        s0 = peg$f7(s2);
      } else {
        peg$currPos = s0;
        s0 = peg$FAILED;
      }
    } else {
      peg$currPos = s0;
      s0 = peg$FAILED;
    }
    peg$silentFails--;
    if (s0 === peg$FAILED) {
      s1 = peg$FAILED;
      if (peg$silentFails === 0) { peg$fail(peg$e23); }
    }

    return s0;
  }

  function peg$parserestOfLineComment() {
    var s0, s1, s2, s3, s4;

    peg$silentFails++;
    s0 = peg$currPos;
    if (input.charCodeAt(peg$currPos) === 59) {
      s1 = peg$c11;
      peg$currPos++;
    } else {
      s1 = peg$FAILED;
      if (peg$silentFails === 0) { peg$fail(peg$e28); }
    }
    if (s1 !== peg$FAILED) {
      s2 = peg$currPos;
      s3 = [];
      s4 = input.charAt(peg$currPos);
      if (peg$r8.test(s4)) {
        peg$currPos++;
      } else {
        s4 = peg$FAILED;
        if (peg$silentFails === 0) { peg$fail(peg$e29); }
      }
      while (s4 !== peg$FAILED) {
        s3.push(s4);
        s4 = input.charAt(peg$currPos);
        if (peg$r8.test(s4)) {
          peg$currPos++;
        } else {
          s4 = peg$FAILED;
          if (peg$silentFails === 0) { peg$fail(peg$e29); }
        }
      }
      s2 = input.substring(s2, peg$currPos);
      s0 = peg$f8(s2);
    } else {
      peg$currPos = s0;
      s0 = peg$FAILED;
    }
    peg$silentFails--;
    if (s0 === peg$FAILED) {
      s1 = peg$FAILED;
      if (peg$silentFails === 0) { peg$fail(peg$e27); }
    }

    return s0;
  }

  function peg$parsevariation() {
    var s0, s2, s3, s5;

    peg$silentFails++;
    s0 = peg$currPos;
    peg$parse_();
    if (input.charCodeAt(peg$currPos) === 40) {
      s2 = peg$c12;
      peg$currPos++;
    } else {
      s2 = peg$FAILED;
      if (peg$silentFails === 0) { peg$fail(peg$e31); }
    }
    if (s2 !== peg$FAILED) {
      s3 = peg$parseline();
      if (s3 !== peg$FAILED) {
        peg$parse_();
        if (input.charCodeAt(peg$currPos) === 41) {
          s5 = peg$c13;
          peg$currPos++;
        } else {
          s5 = peg$FAILED;
          if (peg$silentFails === 0) { peg$fail(peg$e32); }
        }
        if (s5 !== peg$FAILED) {
          s0 = peg$f9(s3);
        } else {
          peg$currPos = s0;
          s0 = peg$FAILED;
        }
      } else {
        peg$currPos = s0;
        s0 = peg$FAILED;
      }
    } else {
      peg$currPos = s0;
      s0 = peg$FAILED;
    }
    peg$silentFails--;
    if (s0 === peg$FAILED) {
      if (peg$silentFails === 0) { peg$fail(peg$e30); }
    }

    return s0;
  }

  function peg$parsegameTerminationMarker() {
    var s0, s1, s3;

    peg$silentFails++;
    s0 = peg$currPos;
    if (input.substr(peg$currPos, 3) === peg$c14) {
      s1 = peg$c14;
      peg$currPos += 3;
    } else {
      s1 = peg$FAILED;
      if (peg$silentFails === 0) { peg$fail(peg$e34); }
    }
    if (s1 === peg$FAILED) {
      if (input.substr(peg$currPos, 3) === peg$c15) {
        s1 = peg$c15;
        peg$currPos += 3;
      } else {
        s1 = peg$FAILED;
        if (peg$silentFails === 0) { peg$fail(peg$e35); }
      }
      if (s1 === peg$FAILED) {
        if (input.substr(peg$currPos, 7) === peg$c16) {
          s1 = peg$c16;
          peg$currPos += 7;
        } else {
          s1 = peg$FAILED;
          if (peg$silentFails === 0) { peg$fail(peg$e36); }
        }
        if (s1 === peg$FAILED) {
          if (input.charCodeAt(peg$currPos) === 42) {
            s1 = peg$c17;
            peg$currPos++;
          } else {
            s1 = peg$FAILED;
            if (peg$silentFails === 0) { peg$fail(peg$e37); }
          }
        }
      }
    }
    if (s1 !== peg$FAILED) {
      peg$parse_();
      s3 = peg$parsecomment();
      if (s3 === peg$FAILED) {
        s3 = null;
      }
      s0 = peg$f10(s1, s3);
    } else {
      peg$currPos = s0;
      s0 = peg$FAILED;
    }
    peg$silentFails--;
    if (s0 === peg$FAILED) {
      s1 = peg$FAILED;
      if (peg$silentFails === 0) { peg$fail(peg$e33); }
    }

    return s0;
  }

  function peg$parse_() {
    var s0, s1;

    peg$silentFails++;
    s0 = [];
    s1 = input.charAt(peg$currPos);
    if (peg$r9.test(s1)) {
      peg$currPos++;
    } else {
      s1 = peg$FAILED;
      if (peg$silentFails === 0) { peg$fail(peg$e39); }
    }
    while (s1 !== peg$FAILED) {
      s0.push(s1);
      s1 = input.charAt(peg$currPos);
      if (peg$r9.test(s1)) {
        peg$currPos++;
      } else {
        s1 = peg$FAILED;
        if (peg$silentFails === 0) { peg$fail(peg$e39); }
      }
    }
    peg$silentFails--;
    s1 = peg$FAILED;
    if (peg$silentFails === 0) { peg$fail(peg$e38); }

    return s0;
  }

  peg$result = peg$startRuleFunction();

  if (options.peg$library) {
    return /** @type {any} */ ({
      peg$result,
      peg$currPos,
      peg$FAILED,
      peg$maxFailExpected,
      peg$maxFailPos
    });
  }
  if (peg$result !== peg$FAILED && peg$currPos === input.length) {
    return peg$result;
  } else {
    if (peg$result !== peg$FAILED && peg$currPos < input.length) {
      peg$fail(peg$endExpectation());
    }

    throw peg$buildStructuredError(
      peg$maxFailExpected,
      peg$maxFailPos < input.length ? input.charAt(peg$maxFailPos) : null,
      peg$maxFailPos < input.length
        ? peg$computeLocation(peg$maxFailPos, peg$maxFailPos + 1)
        : peg$computeLocation(peg$maxFailPos, peg$maxFailPos)
    );
  }
}

/**
 * @license
 * Copyright (c) 2025, Jeff Hlywa (jhlywa@gmail.com)
 * All rights reserved.
 *
 * Redistribution and use in source and binary forms, with or without
 * modification, are permitted provided that the following conditions are met:
 *
 * 1. Redistributions of source code must retain the above copyright notice,
 *    this list of conditions and the following disclaimer.
 * 2. Redistributions in binary form must reproduce the above copyright notice,
 *    this list of conditions and the following disclaimer in the documentation
 *    and/or other materials provided with the distribution.
 *
 * THIS SOFTWARE IS PROVIDED BY THE COPYRIGHT HOLDERS AND CONTRIBUTORS "AS IS"
 * AND ANY EXPRESS OR IMPLIED WARRANTIES, INCLUDING, BUT NOT LIMITED TO, THE
 * IMPLIED WARRANTIES OF MERCHANTABILITY AND FITNESS FOR A PARTICULAR PURPOSE
 * ARE DISCLAIMED. IN NO EVENT SHALL THE COPYRIGHT OWNER OR CONTRIBUTORS BE
 * LIABLE FOR ANY DIRECT, INDIRECT, INCIDENTAL, SPECIAL, EXEMPLARY, OR
 * CONSEQUENTIAL DAMAGES (INCLUDING, BUT NOT LIMITED TO, PROCUREMENT OF
 * SUBSTITUTE GOODS OR SERVICES; LOSS OF USE, DATA, OR PROFITS; OR BUSINESS
 * INTERRUPTION) HOWEVER CAUSED AND ON ANY THEORY OF LIABILITY, WHETHER IN
 * CONTRACT, STRICT LIABILITY, OR TORT (INCLUDING NEGLIGENCE OR OTHERWISE)
 * ARISING IN ANY WAY OUT OF THE USE OF THIS SOFTWARE, EVEN IF ADVISED OF THE
 * POSSIBILITY OF SUCH DAMAGE.
 */
const MASK64 = 0xffffffffffffffffn;
function rotl(x, k) {
    return ((x << k) | (x >> (64n - k))) & 0xffffffffffffffffn;
}
function wrappingMul(x, y) {
    return (x * y) & MASK64;
}
// xoroshiro128**
function xoroshiro128(state) {
    return function () {
        let s0 = BigInt(state & MASK64);
        let s1 = BigInt((state >> 64n) & MASK64);
        const result = wrappingMul(rotl(wrappingMul(s0, 5n), 7n), 9n);
        s1 ^= s0;
        s0 = (rotl(s0, 24n) ^ s1 ^ (s1 << 16n)) & MASK64;
        s1 = rotl(s1, 37n);
        state = (s1 << 64n) | s0;
        return result;
    };
}
const rand = xoroshiro128(0xa187eb39cdcaed8f31c4b365b102e01en);
const PIECE_KEYS = Array.from({ length: 2 }, () => Array.from({ length: 6 }, () => Array.from({ length: 128 }, () => rand())));
const EP_KEYS = Array.from({ length: 8 }, () => rand());
const CASTLING_KEYS = Array.from({ length: 16 }, () => rand());
const SIDE_KEY = rand();
const WHITE = 'w';
const BLACK = 'b';
const PAWN = 'p';
const KNIGHT = 'n';
const BISHOP = 'b';
const ROOK = 'r';
const QUEEN = 'q';
const KING = 'k';
const DEFAULT_POSITION = 'rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR w KQkq - 0 1';
class Move {
    color;
    from;
    to;
    piece;
    captured;
    promotion;
    /**
     * @deprecated This field is deprecated and will be removed in version 2.0.0.
     * Please use move descriptor functions instead: `isCapture`, `isPromotion`,
     * `isEnPassant`, `isKingsideCastle`, `isQueensideCastle`, `isCastle`, and
     * `isBigPawn`
     */
    flags;
    san;
    lan;
    before;
    after;
    constructor(chess, internal) {
        const { color, piece, from, to, flags, captured, promotion } = internal;
        const fromAlgebraic = algebraic(from);
        const toAlgebraic = algebraic(to);
        this.color = color;
        this.piece = piece;
        this.from = fromAlgebraic;
        this.to = toAlgebraic;
        /*
         * HACK: The chess['_method']() calls below invoke private methods in the
         * Chess class to generate SAN and FEN. It's a bit of a hack, but makes the
         * code cleaner elsewhere.
         */
        this.san = chess['_moveToSan'](internal, chess['_moves']({ legal: true }));
        this.lan = fromAlgebraic + toAlgebraic;
        this.before = chess.fen();
        // Generate the FEN for the 'after' key
        chess['_makeMove'](internal);
        this.after = chess.fen();
        chess['_undoMove']();
        // Build the text representation of the move flags
        this.flags = '';
        for (const flag in BITS) {
            if (BITS[flag] & flags) {
                this.flags += FLAGS[flag];
            }
        }
        if (captured) {
            this.captured = captured;
        }
        if (promotion) {
            this.promotion = promotion;
            this.lan += promotion;
        }
    }
    isCapture() {
        return this.flags.indexOf(FLAGS['CAPTURE']) > -1;
    }
    isPromotion() {
        return this.flags.indexOf(FLAGS['PROMOTION']) > -1;
    }
    isEnPassant() {
        return this.flags.indexOf(FLAGS['EP_CAPTURE']) > -1;
    }
    isKingsideCastle() {
        return this.flags.indexOf(FLAGS['KSIDE_CASTLE']) > -1;
    }
    isQueensideCastle() {
        return this.flags.indexOf(FLAGS['QSIDE_CASTLE']) > -1;
    }
    isBigPawn() {
        return this.flags.indexOf(FLAGS['BIG_PAWN']) > -1;
    }
}
const EMPTY = -1;
const FLAGS = {
    NORMAL: 'n',
    CAPTURE: 'c',
    BIG_PAWN: 'b',
    EP_CAPTURE: 'e',
    PROMOTION: 'p',
    KSIDE_CASTLE: 'k',
    QSIDE_CASTLE: 'q',
    NULL_MOVE: '-',
};
const BITS = {
    NORMAL: 1,
    CAPTURE: 2,
    BIG_PAWN: 4,
    EP_CAPTURE: 8,
    PROMOTION: 16,
    KSIDE_CASTLE: 32,
    QSIDE_CASTLE: 64,
    NULL_MOVE: 128,
};
/* eslint-disable @typescript-eslint/naming-convention */
// these are required, according to spec
const SEVEN_TAG_ROSTER = {
    Event: '?',
    Site: '?',
    Date: '????.??.??',
    Round: '?',
    White: '?',
    Black: '?',
    Result: '*',
};
/**
 * These nulls are placeholders to fix the order of tags (as they appear in PGN spec); null values will be
 * eliminated in getHeaders()
 */
const SUPLEMENTAL_TAGS = {
    WhiteTitle: null,
    BlackTitle: null,
    WhiteElo: null,
    BlackElo: null,
    WhiteUSCF: null,
    BlackUSCF: null,
    WhiteNA: null,
    BlackNA: null,
    WhiteType: null,
    BlackType: null,
    EventDate: null,
    EventSponsor: null,
    Section: null,
    Stage: null,
    Board: null,
    Opening: null,
    Variation: null,
    SubVariation: null,
    ECO: null,
    NIC: null,
    Time: null,
    UTCTime: null,
    UTCDate: null,
    TimeControl: null,
    SetUp: null,
    FEN: null,
    Termination: null,
    Annotator: null,
    Mode: null,
    PlyCount: null,
};
const HEADER_TEMPLATE = {
    ...SEVEN_TAG_ROSTER,
    ...SUPLEMENTAL_TAGS,
};
/* eslint-enable @typescript-eslint/naming-convention */
/*
 * NOTES ABOUT 0x88 MOVE GENERATION ALGORITHM
 * ----------------------------------------------------------------------------
 * From https://github.com/jhlywa/chess.js/issues/230
 *
 * A lot of people are confused when they first see the internal representation
 * of chess.js. It uses the 0x88 Move Generation Algorithm which internally
 * stores the board as an 8x16 array. This is purely for efficiency but has a
 * couple of interesting benefits:
 *
 * 1. 0x88 offers a very inexpensive "off the board" check. Bitwise AND (&) any
 *    square with 0x88, if the result is non-zero then the square is off the
 *    board. For example, assuming a knight square A8 (0 in 0x88 notation),
 *    there are 8 possible directions in which the knight can move. These
 *    directions are relative to the 8x16 board and are stored in the
 *    PIECE_OFFSETS map. One possible move is A8 - 18 (up one square, and two
 *    squares to the left - which is off the board). 0 - 18 = -18 & 0x88 = 0x88
 *    (because of two-complement representation of -18). The non-zero result
 *    means the square is off the board and the move is illegal. Take the
 *    opposite move (from A8 to C7), 0 + 18 = 18 & 0x88 = 0. A result of zero
 *    means the square is on the board.
 *
 * 2. The relative distance (or difference) between two squares on a 8x16 board
 *    is unique and can be used to inexpensively determine if a piece on a
 *    square can attack any other arbitrary square. For example, let's see if a
 *    pawn on E7 can attack E2. The difference between E7 (20) - E2 (100) is
 *    -80. We add 119 to make the ATTACKS array index non-negative (because the
 *    worst case difference is A8 - H1 = -119). The ATTACKS array contains a
 *    bitmask of pieces that can attack from that distance and direction.
 *    ATTACKS[-80 + 119=39] gives us 24 or 0b11000 in binary. Look at the
 *    PIECE_MASKS map to determine the mask for a given piece type. In our pawn
 *    example, we would check to see if 24 & 0x1 is non-zero, which it is
 *    not. So, naturally, a pawn on E7 can't attack a piece on E2. However, a
 *    rook can since 24 & 0x8 is non-zero. The only thing left to check is that
 *    there are no blocking pieces between E7 and E2. That's where the RAYS
 *    array comes in. It provides an offset (in this case 16) to add to E7 (20)
 *    to check for blocking pieces. E7 (20) + 16 = E6 (36) + 16 = E5 (52) etc.
 */
// prettier-ignore
// eslint-disable-next-line
const Ox88 = {
    a8: 0, b8: 1, c8: 2, d8: 3, e8: 4, f8: 5, g8: 6, h8: 7,
    a7: 16, b7: 17, c7: 18, d7: 19, e7: 20, f7: 21, g7: 22, h7: 23,
    a6: 32, b6: 33, c6: 34, d6: 35, e6: 36, f6: 37, g6: 38, h6: 39,
    a5: 48, b5: 49, c5: 50, d5: 51, e5: 52, f5: 53, g5: 54, h5: 55,
    a4: 64, b4: 65, c4: 66, d4: 67, e4: 68, f4: 69, g4: 70, h4: 71,
    a3: 80, b3: 81, c3: 82, d3: 83, e3: 84, f3: 85, g3: 86, h3: 87,
    a2: 96, b2: 97, c2: 98, d2: 99, e2: 100, f2: 101, g2: 102, h2: 103,
    a1: 112, b1: 113, c1: 114, d1: 115, e1: 116, f1: 117, g1: 118, h1: 119
};
const PAWN_OFFSETS = {
    b: [16, 32, 17, 15],
    w: [-16, -32, -17, -15],
};
const PIECE_OFFSETS = {
    n: [-18, -33, -31, -14, 18, 33, 31, 14],
    b: [-17, -15, 17, 15],
    r: [-16, 1, 16, -1],
    q: [-17, -16, -15, 1, 17, 16, 15, -1],
    k: [-17, -16, -15, 1, 17, 16, 15, -1],
};
// prettier-ignore
const ATTACKS = [
    20, 0, 0, 0, 0, 0, 0, 24, 0, 0, 0, 0, 0, 0, 20, 0,
    0, 20, 0, 0, 0, 0, 0, 24, 0, 0, 0, 0, 0, 20, 0, 0,
    0, 0, 20, 0, 0, 0, 0, 24, 0, 0, 0, 0, 20, 0, 0, 0,
    0, 0, 0, 20, 0, 0, 0, 24, 0, 0, 0, 20, 0, 0, 0, 0,
    0, 0, 0, 0, 20, 0, 0, 24, 0, 0, 20, 0, 0, 0, 0, 0,
    0, 0, 0, 0, 0, 20, 2, 24, 2, 20, 0, 0, 0, 0, 0, 0,
    0, 0, 0, 0, 0, 2, 53, 56, 53, 2, 0, 0, 0, 0, 0, 0,
    24, 24, 24, 24, 24, 24, 56, 0, 56, 24, 24, 24, 24, 24, 24, 0,
    0, 0, 0, 0, 0, 2, 53, 56, 53, 2, 0, 0, 0, 0, 0, 0,
    0, 0, 0, 0, 0, 20, 2, 24, 2, 20, 0, 0, 0, 0, 0, 0,
    0, 0, 0, 0, 20, 0, 0, 24, 0, 0, 20, 0, 0, 0, 0, 0,
    0, 0, 0, 20, 0, 0, 0, 24, 0, 0, 0, 20, 0, 0, 0, 0,
    0, 0, 20, 0, 0, 0, 0, 24, 0, 0, 0, 0, 20, 0, 0, 0,
    0, 20, 0, 0, 0, 0, 0, 24, 0, 0, 0, 0, 0, 20, 0, 0,
    20, 0, 0, 0, 0, 0, 0, 24, 0, 0, 0, 0, 0, 0, 20
];
// prettier-ignore
const RAYS = [
    17, 0, 0, 0, 0, 0, 0, 16, 0, 0, 0, 0, 0, 0, 15, 0,
    0, 17, 0, 0, 0, 0, 0, 16, 0, 0, 0, 0, 0, 15, 0, 0,
    0, 0, 17, 0, 0, 0, 0, 16, 0, 0, 0, 0, 15, 0, 0, 0,
    0, 0, 0, 17, 0, 0, 0, 16, 0, 0, 0, 15, 0, 0, 0, 0,
    0, 0, 0, 0, 17, 0, 0, 16, 0, 0, 15, 0, 0, 0, 0, 0,
    0, 0, 0, 0, 0, 17, 0, 16, 0, 15, 0, 0, 0, 0, 0, 0,
    0, 0, 0, 0, 0, 0, 17, 16, 15, 0, 0, 0, 0, 0, 0, 0,
    1, 1, 1, 1, 1, 1, 1, 0, -1, -1, -1, -1, -1, -1, -1, 0,
    0, 0, 0, 0, 0, 0, -15, -16, -17, 0, 0, 0, 0, 0, 0, 0,
    0, 0, 0, 0, 0, -15, 0, -16, 0, -17, 0, 0, 0, 0, 0, 0,
    0, 0, 0, 0, -15, 0, 0, -16, 0, 0, -17, 0, 0, 0, 0, 0,
    0, 0, 0, -15, 0, 0, 0, -16, 0, 0, 0, -17, 0, 0, 0, 0,
    0, 0, -15, 0, 0, 0, 0, -16, 0, 0, 0, 0, -17, 0, 0, 0,
    0, -15, 0, 0, 0, 0, 0, -16, 0, 0, 0, 0, 0, -17, 0, 0,
    -15, 0, 0, 0, 0, 0, 0, -16, 0, 0, 0, 0, 0, 0, -17
];
const PIECE_MASKS = { p: 0x1, n: 0x2, b: 0x4, r: 0x8, q: 0x10, k: 0x20 };
const SYMBOLS = 'pnbrqkPNBRQK';
const PROMOTIONS = [KNIGHT, BISHOP, ROOK, QUEEN];
const RANK_1 = 7;
const RANK_2 = 6;
/*
 * const RANK_3 = 5
 * const RANK_4 = 4
 * const RANK_5 = 3
 * const RANK_6 = 2
 */
const RANK_7 = 1;
const RANK_8 = 0;
const SIDES = {
    [KING]: BITS.KSIDE_CASTLE,
    [QUEEN]: BITS.QSIDE_CASTLE,
};
const ROOKS = {
    w: [
        { square: Ox88.a1, flag: BITS.QSIDE_CASTLE },
        { square: Ox88.h1, flag: BITS.KSIDE_CASTLE },
    ],
    b: [
        { square: Ox88.a8, flag: BITS.QSIDE_CASTLE },
        { square: Ox88.h8, flag: BITS.KSIDE_CASTLE },
    ],
};
const SECOND_RANK = { b: RANK_7, w: RANK_2 };
const SAN_NULLMOVE = '--';
// Extracts the zero-based rank of an 0x88 square.
function rank(square) {
    return square >> 4;
}
// Extracts the zero-based file of an 0x88 square.
function file(square) {
    return square & 0xf;
}
function isDigit(c) {
    return '0123456789'.indexOf(c) !== -1;
}
// Converts a 0x88 square to algebraic notation.
function algebraic(square) {
    const f = file(square);
    const r = rank(square);
    return ('abcdefgh'.substring(f, f + 1) +
        '87654321'.substring(r, r + 1));
}
function swapColor(color) {
    return color === WHITE ? BLACK : WHITE;
}
function validateFen(fen) {
    // 1st criterion: 6 space-seperated fields?
    const tokens = fen.split(/\s+/);
    if (tokens.length !== 6) {
        return {
            ok: false,
            error: 'Invalid FEN: must contain six space-delimited fields',
        };
    }
    // 2nd criterion: move number field is a integer value > 0?
    const moveNumber = parseInt(tokens[5], 10);
    if (isNaN(moveNumber) || moveNumber <= 0) {
        return {
            ok: false,
            error: 'Invalid FEN: move number must be a positive integer',
        };
    }
    // 3rd criterion: half move counter is an integer >= 0?
    const halfMoves = parseInt(tokens[4], 10);
    if (isNaN(halfMoves) || halfMoves < 0) {
        return {
            ok: false,
            error: 'Invalid FEN: half move counter number must be a non-negative integer',
        };
    }
    // 4th criterion: 4th field is a valid e.p.-string?
    if (!/^(-|[abcdefgh][36])$/.test(tokens[3])) {
        return { ok: false, error: 'Invalid FEN: en-passant square is invalid' };
    }
    // 5th criterion: 3th field is a valid castle-string?
    if (/[^kKqQ-]/.test(tokens[2])) {
        return { ok: false, error: 'Invalid FEN: castling availability is invalid' };
    }
    // 6th criterion: 2nd field is "w" (white) or "b" (black)?
    if (!/^(w|b)$/.test(tokens[1])) {
        return { ok: false, error: 'Invalid FEN: side-to-move is invalid' };
    }
    // 7th criterion: 1st field contains 8 rows?
    const rows = tokens[0].split('/');
    if (rows.length !== 8) {
        return {
            ok: false,
            error: "Invalid FEN: piece data does not contain 8 '/'-delimited rows",
        };
    }
    // 8th criterion: every row is valid?
    for (let i = 0; i < rows.length; i++) {
        // check for right sum of fields AND not two numbers in succession
        let sumFields = 0;
        let previousWasNumber = false;
        for (let k = 0; k < rows[i].length; k++) {
            if (isDigit(rows[i][k])) {
                if (previousWasNumber) {
                    return {
                        ok: false,
                        error: 'Invalid FEN: piece data is invalid (consecutive number)',
                    };
                }
                sumFields += parseInt(rows[i][k], 10);
                previousWasNumber = true;
            }
            else {
                if (!/^[prnbqkPRNBQK]$/.test(rows[i][k])) {
                    return {
                        ok: false,
                        error: 'Invalid FEN: piece data is invalid (invalid piece)',
                    };
                }
                sumFields += 1;
                previousWasNumber = false;
            }
        }
        if (sumFields !== 8) {
            return {
                ok: false,
                error: 'Invalid FEN: piece data is invalid (too many squares in rank)',
            };
        }
    }
    // 9th criterion: is en-passant square legal?
    if ((tokens[3][1] == '3' && tokens[1] == 'w') ||
        (tokens[3][1] == '6' && tokens[1] == 'b')) {
        return { ok: false, error: 'Invalid FEN: illegal en-passant square' };
    }
    // 10th criterion: does chess position contain exact two kings?
    const kings = [
        { color: 'white', regex: /K/g },
        { color: 'black', regex: /k/g },
    ];
    for (const { color, regex } of kings) {
        if (!regex.test(tokens[0])) {
            return { ok: false, error: `Invalid FEN: missing ${color} king` };
        }
        if ((tokens[0].match(regex) || []).length > 1) {
            return { ok: false, error: `Invalid FEN: too many ${color} kings` };
        }
    }
    // 11th criterion: are any pawns on the first or eighth rows?
    if (Array.from(rows[0] + rows[7]).some((char) => char.toUpperCase() === 'P')) {
        return {
            ok: false,
            error: 'Invalid FEN: some pawns are on the edge rows',
        };
    }
    return { ok: true };
}
// this function is used to uniquely identify ambiguous moves
function getDisambiguator(move, moves) {
    const from = move.from;
    const to = move.to;
    const piece = move.piece;
    let ambiguities = 0;
    let sameRank = 0;
    let sameFile = 0;
    for (let i = 0, len = moves.length; i < len; i++) {
        const ambigFrom = moves[i].from;
        const ambigTo = moves[i].to;
        const ambigPiece = moves[i].piece;
        /*
         * if a move of the same piece type ends on the same to square, we'll need
         * to add a disambiguator to the algebraic notation
         */
        if (piece === ambigPiece && from !== ambigFrom && to === ambigTo) {
            ambiguities++;
            if (rank(from) === rank(ambigFrom)) {
                sameRank++;
            }
            if (file(from) === file(ambigFrom)) {
                sameFile++;
            }
        }
    }
    if (ambiguities > 0) {
        if (sameRank > 0 && sameFile > 0) {
            /*
             * if there exists a similar moving piece on the same rank and file as
             * the move in question, use the square as the disambiguator
             */
            return algebraic(from);
        }
        else if (sameFile > 0) {
            /*
             * if the moving piece rests on the same file, use the rank symbol as the
             * disambiguator
             */
            return algebraic(from).charAt(1);
        }
        else {
            // else use the file symbol
            return algebraic(from).charAt(0);
        }
    }
    return '';
}
function addMove(moves, color, from, to, piece, captured = undefined, flags = BITS.NORMAL) {
    const r = rank(to);
    if (piece === PAWN && (r === RANK_1 || r === RANK_8)) {
        for (let i = 0; i < PROMOTIONS.length; i++) {
            const promotion = PROMOTIONS[i];
            moves.push({
                color,
                from,
                to,
                piece,
                captured,
                promotion,
                flags: flags | BITS.PROMOTION,
            });
        }
    }
    else {
        moves.push({
            color,
            from,
            to,
            piece,
            captured,
            flags,
        });
    }
}
function inferPieceType(san) {
    let pieceType = san.charAt(0);
    if (pieceType >= 'a' && pieceType <= 'h') {
        const matches = san.match(/[a-h]\d.*[a-h]\d/);
        if (matches) {
            return undefined;
        }
        return PAWN;
    }
    pieceType = pieceType.toLowerCase();
    if (pieceType === 'o') {
        return KING;
    }
    return pieceType;
}
// parses all of the decorators out of a SAN string
function strippedSan(move) {
    return move.replace(/=/, '').replace(/[+#]?[?!]*$/, '');
}
class Chess {
    _board = new Array(128);
    _turn = WHITE;
    _header = {};
    _kings = { w: EMPTY, b: EMPTY };
    _epSquare = -1;
    _halfMoves = 0;
    _moveNumber = 0;
    _history = [];
    _comments = {};
    _castling = { w: 0, b: 0 };
    _hash = 0n;
    // tracks number of times a position has been seen for repetition checking
    _positionCount = new Map();
    constructor(fen = DEFAULT_POSITION, { skipValidation = false } = {}) {
        this.load(fen, { skipValidation });
    }
    clear({ preserveHeaders = false } = {}) {
        this._board = new Array(128);
        this._kings = { w: EMPTY, b: EMPTY };
        this._turn = WHITE;
        this._castling = { w: 0, b: 0 };
        this._epSquare = EMPTY;
        this._halfMoves = 0;
        this._moveNumber = 1;
        this._history = [];
        this._comments = {};
        this._header = preserveHeaders ? this._header : { ...HEADER_TEMPLATE };
        this._hash = this._computeHash();
        this._positionCount = new Map();
        /*
         * Delete the SetUp and FEN headers (if preserved), the board is empty and
         * these headers don't make sense in this state. They'll get added later
         * via .load() or .put()
         */
        this._header['SetUp'] = null;
        this._header['FEN'] = null;
    }
    load(fen, { skipValidation = false, preserveHeaders = false } = {}) {
        let tokens = fen.split(/\s+/);
        // append commonly omitted fen tokens
        if (tokens.length >= 2 && tokens.length < 6) {
            const adjustments = ['-', '-', '0', '1'];
            fen = tokens.concat(adjustments.slice(-(6 - tokens.length))).join(' ');
        }
        tokens = fen.split(/\s+/);
        if (!skipValidation) {
            const { ok, error } = validateFen(fen);
            if (!ok) {
                throw new Error(error);
            }
        }
        const position = tokens[0];
        let square = 0;
        this.clear({ preserveHeaders });
        for (let i = 0; i < position.length; i++) {
            const piece = position.charAt(i);
            if (piece === '/') {
                square += 8;
            }
            else if (isDigit(piece)) {
                square += parseInt(piece, 10);
            }
            else {
                const color = piece < 'a' ? WHITE : BLACK;
                this._put({ type: piece.toLowerCase(), color }, algebraic(square));
                square++;
            }
        }
        this._turn = tokens[1];
        if (tokens[2].indexOf('K') > -1) {
            this._castling.w |= BITS.KSIDE_CASTLE;
        }
        if (tokens[2].indexOf('Q') > -1) {
            this._castling.w |= BITS.QSIDE_CASTLE;
        }
        if (tokens[2].indexOf('k') > -1) {
            this._castling.b |= BITS.KSIDE_CASTLE;
        }
        if (tokens[2].indexOf('q') > -1) {
            this._castling.b |= BITS.QSIDE_CASTLE;
        }
        this._epSquare = tokens[3] === '-' ? EMPTY : Ox88[tokens[3]];
        this._halfMoves = parseInt(tokens[4], 10);
        this._moveNumber = parseInt(tokens[5], 10);
        this._hash = this._computeHash();
        this._updateSetup(fen);
        this._incPositionCount();
    }
    fen({ forceEnpassantSquare = false, } = {}) {
        let empty = 0;
        let fen = '';
        for (let i = Ox88.a8; i <= Ox88.h1; i++) {
            if (this._board[i]) {
                if (empty > 0) {
                    fen += empty;
                    empty = 0;
                }
                const { color, type: piece } = this._board[i];
                fen += color === WHITE ? piece.toUpperCase() : piece.toLowerCase();
            }
            else {
                empty++;
            }
            if ((i + 1) & 0x88) {
                if (empty > 0) {
                    fen += empty;
                }
                if (i !== Ox88.h1) {
                    fen += '/';
                }
                empty = 0;
                i += 8;
            }
        }
        let castling = '';
        if (this._castling[WHITE] & BITS.KSIDE_CASTLE) {
            castling += 'K';
        }
        if (this._castling[WHITE] & BITS.QSIDE_CASTLE) {
            castling += 'Q';
        }
        if (this._castling[BLACK] & BITS.KSIDE_CASTLE) {
            castling += 'k';
        }
        if (this._castling[BLACK] & BITS.QSIDE_CASTLE) {
            castling += 'q';
        }
        // do we have an empty castling flag?
        castling = castling || '-';
        let epSquare = '-';
        /*
         * only print the ep square if en passant is a valid move (pawn is present
         * and ep capture is not pinned)
         */
        if (this._epSquare !== EMPTY) {
            if (forceEnpassantSquare) {
                epSquare = algebraic(this._epSquare);
            }
            else {
                const bigPawnSquare = this._epSquare + (this._turn === WHITE ? 16 : -16);
                const squares = [bigPawnSquare + 1, bigPawnSquare - 1];
                for (const square of squares) {
                    // is the square off the board?
                    if (square & 0x88) {
                        continue;
                    }
                    const color = this._turn;
                    // is there a pawn that can capture the epSquare?
                    if (this._board[square]?.color === color &&
                        this._board[square]?.type === PAWN) {
                        // if the pawn makes an ep capture, does it leave its king in check?
                        this._makeMove({
                            color,
                            from: square,
                            to: this._epSquare,
                            piece: PAWN,
                            captured: PAWN,
                            flags: BITS.EP_CAPTURE,
                        });
                        const isLegal = !this._isKingAttacked(color);
                        this._undoMove();
                        // if ep is legal, break and set the ep square in the FEN output
                        if (isLegal) {
                            epSquare = algebraic(this._epSquare);
                            break;
                        }
                    }
                }
            }
        }
        return [
            fen,
            this._turn,
            castling,
            epSquare,
            this._halfMoves,
            this._moveNumber,
        ].join(' ');
    }
    _pieceKey(i) {
        if (!this._board[i]) {
            return 0n;
        }
        const { color, type } = this._board[i];
        const colorIndex = {
            w: 0,
            b: 1,
        }[color];
        const typeIndex = {
            p: 0,
            n: 1,
            b: 2,
            r: 3,
            q: 4,
            k: 5,
        }[type];
        return PIECE_KEYS[colorIndex][typeIndex][i];
    }
    _epKey() {
        return this._epSquare === EMPTY ? 0n : EP_KEYS[this._epSquare & 7];
    }
    _castlingKey() {
        const index = (this._castling.w >> 5) | (this._castling.b >> 3);
        return CASTLING_KEYS[index];
    }
    _computeHash() {
        let hash = 0n;
        for (let i = Ox88.a8; i <= Ox88.h1; i++) {
            // did we run off the end of the board
            if (i & 0x88) {
                i += 7;
                continue;
            }
            if (this._board[i]) {
                hash ^= this._pieceKey(i);
            }
        }
        hash ^= this._epKey();
        hash ^= this._castlingKey();
        if (this._turn === 'b') {
            hash ^= SIDE_KEY;
        }
        return hash;
    }
    /*
     * Called when the initial board setup is changed with put() or remove().
     * modifies the SetUp and FEN properties of the header object. If the FEN
     * is equal to the default position, the SetUp and FEN are deleted the setup
     * is only updated if history.length is zero, ie moves haven't been made.
     */
    _updateSetup(fen) {
        if (this._history.length > 0)
            return;
        if (fen !== DEFAULT_POSITION) {
            this._header['SetUp'] = '1';
            this._header['FEN'] = fen;
        }
        else {
            this._header['SetUp'] = null;
            this._header['FEN'] = null;
        }
    }
    reset() {
        this.load(DEFAULT_POSITION);
    }
    get(square) {
        return this._board[Ox88[square]];
    }
    findPiece(piece) {
        const squares = [];
        for (let i = Ox88.a8; i <= Ox88.h1; i++) {
            // did we run off the end of the board
            if (i & 0x88) {
                i += 7;
                continue;
            }
            // if empty square or wrong color
            if (!this._board[i] || this._board[i]?.color !== piece.color) {
                continue;
            }
            // check if square contains the requested piece
            if (this._board[i].color === piece.color &&
                this._board[i].type === piece.type) {
                squares.push(algebraic(i));
            }
        }
        return squares;
    }
    put({ type, color }, square) {
        if (this._put({ type, color }, square)) {
            this._updateCastlingRights();
            this._updateEnPassantSquare();
            this._updateSetup(this.fen());
            return true;
        }
        return false;
    }
    _set(sq, piece) {
        this._hash ^= this._pieceKey(sq);
        this._board[sq] = piece;
        this._hash ^= this._pieceKey(sq);
    }
    _put({ type, color }, square) {
        // check for piece
        if (SYMBOLS.indexOf(type.toLowerCase()) === -1) {
            return false;
        }
        // check for valid square
        if (!(square in Ox88)) {
            return false;
        }
        const sq = Ox88[square];
        // don't let the user place more than one king
        if (type == KING &&
            !(this._kings[color] == EMPTY || this._kings[color] == sq)) {
            return false;
        }
        const currentPieceOnSquare = this._board[sq];
        // if one of the kings will be replaced by the piece from args, set the `_kings` respective entry to `EMPTY`
        if (currentPieceOnSquare && currentPieceOnSquare.type === KING) {
            this._kings[currentPieceOnSquare.color] = EMPTY;
        }
        this._set(sq, { type: type, color: color });
        if (type === KING) {
            this._kings[color] = sq;
        }
        return true;
    }
    _clear(sq) {
        this._hash ^= this._pieceKey(sq);
        delete this._board[sq];
    }
    remove(square) {
        const piece = this.get(square);
        this._clear(Ox88[square]);
        if (piece && piece.type === KING) {
            this._kings[piece.color] = EMPTY;
        }
        this._updateCastlingRights();
        this._updateEnPassantSquare();
        this._updateSetup(this.fen());
        return piece;
    }
    _updateCastlingRights() {
        this._hash ^= this._castlingKey();
        const whiteKingInPlace = this._board[Ox88.e1]?.type === KING &&
            this._board[Ox88.e1]?.color === WHITE;
        const blackKingInPlace = this._board[Ox88.e8]?.type === KING &&
            this._board[Ox88.e8]?.color === BLACK;
        if (!whiteKingInPlace ||
            this._board[Ox88.a1]?.type !== ROOK ||
            this._board[Ox88.a1]?.color !== WHITE) {
            this._castling.w &= -65;
        }
        if (!whiteKingInPlace ||
            this._board[Ox88.h1]?.type !== ROOK ||
            this._board[Ox88.h1]?.color !== WHITE) {
            this._castling.w &= -33;
        }
        if (!blackKingInPlace ||
            this._board[Ox88.a8]?.type !== ROOK ||
            this._board[Ox88.a8]?.color !== BLACK) {
            this._castling.b &= -65;
        }
        if (!blackKingInPlace ||
            this._board[Ox88.h8]?.type !== ROOK ||
            this._board[Ox88.h8]?.color !== BLACK) {
            this._castling.b &= -33;
        }
        this._hash ^= this._castlingKey();
    }
    _updateEnPassantSquare() {
        if (this._epSquare === EMPTY) {
            return;
        }
        const startSquare = this._epSquare + (this._turn === WHITE ? -16 : 16);
        const currentSquare = this._epSquare + (this._turn === WHITE ? 16 : -16);
        const attackers = [currentSquare + 1, currentSquare - 1];
        if (this._board[startSquare] !== null ||
            this._board[this._epSquare] !== null ||
            this._board[currentSquare]?.color !== swapColor(this._turn) ||
            this._board[currentSquare]?.type !== PAWN) {
            this._hash ^= this._epKey();
            this._epSquare = EMPTY;
            return;
        }
        const canCapture = (square) => !(square & 0x88) &&
            this._board[square]?.color === this._turn &&
            this._board[square]?.type === PAWN;
        if (!attackers.some(canCapture)) {
            this._hash ^= this._epKey();
            this._epSquare = EMPTY;
        }
    }
    _attacked(color, square, verbose) {
        const attackers = [];
        for (let i = Ox88.a8; i <= Ox88.h1; i++) {
            // did we run off the end of the board
            if (i & 0x88) {
                i += 7;
                continue;
            }
            // if empty square or wrong color
            if (this._board[i] === undefined || this._board[i].color !== color) {
                continue;
            }
            const piece = this._board[i];
            const difference = i - square;
            // skip - to/from square are the same
            if (difference === 0) {
                continue;
            }
            const index = difference + 119;
            if (ATTACKS[index] & PIECE_MASKS[piece.type]) {
                if (piece.type === PAWN) {
                    if ((difference > 0 && piece.color === WHITE) ||
                        (difference <= 0 && piece.color === BLACK)) {
                        if (!verbose) {
                            return true;
                        }
                        else {
                            attackers.push(algebraic(i));
                        }
                    }
                    continue;
                }
                // if the piece is a knight or a king
                if (piece.type === 'n' || piece.type === 'k') {
                    if (!verbose) {
                        return true;
                    }
                    else {
                        attackers.push(algebraic(i));
                        continue;
                    }
                }
                const offset = RAYS[index];
                let j = i + offset;
                let blocked = false;
                while (j !== square) {
                    if (this._board[j] != null) {
                        blocked = true;
                        break;
                    }
                    j += offset;
                }
                if (!blocked) {
                    if (!verbose) {
                        return true;
                    }
                    else {
                        attackers.push(algebraic(i));
                        continue;
                    }
                }
            }
        }
        if (verbose) {
            return attackers;
        }
        else {
            return false;
        }
    }
    attackers(square, attackedBy) {
        if (!attackedBy) {
            return this._attacked(this._turn, Ox88[square], true);
        }
        else {
            return this._attacked(attackedBy, Ox88[square], true);
        }
    }
    _isKingAttacked(color) {
        const square = this._kings[color];
        return square === -1 ? false : this._attacked(swapColor(color), square);
    }
    hash() {
        return this._hash.toString(16);
    }
    isAttacked(square, attackedBy) {
        return this._attacked(attackedBy, Ox88[square]);
    }
    isCheck() {
        return this._isKingAttacked(this._turn);
    }
    inCheck() {
        return this.isCheck();
    }
    isCheckmate() {
        return this.isCheck() && this._moves().length === 0;
    }
    isStalemate() {
        return !this.isCheck() && this._moves().length === 0;
    }
    isInsufficientMaterial() {
        /*
         * k.b. vs k.b. (of opposite colors) with mate in 1:
         * 8/8/8/8/1b6/8/B1k5/K7 b - - 0 1
         *
         * k.b. vs k.n. with mate in 1:
         * 8/8/8/8/1n6/8/B7/K1k5 b - - 2 1
         */
        const pieces = {
            b: 0,
            n: 0,
            r: 0,
            q: 0,
            k: 0,
            p: 0,
        };
        const bishops = [];
        let numPieces = 0;
        let squareColor = 0;
        for (let i = Ox88.a8; i <= Ox88.h1; i++) {
            squareColor = (squareColor + 1) % 2;
            if (i & 0x88) {
                i += 7;
                continue;
            }
            const piece = this._board[i];
            if (piece) {
                pieces[piece.type] = piece.type in pieces ? pieces[piece.type] + 1 : 1;
                if (piece.type === BISHOP) {
                    bishops.push(squareColor);
                }
                numPieces++;
            }
        }
        // k vs. k
        if (numPieces === 2) {
            return true;
        }
        else if (
        // k vs. kn .... or .... k vs. kb
        numPieces === 3 &&
            (pieces[BISHOP] === 1 || pieces[KNIGHT] === 1)) {
            return true;
        }
        else if (numPieces === pieces[BISHOP] + 2) {
            // kb vs. kb where any number of bishops are all on the same color
            let sum = 0;
            const len = bishops.length;
            for (let i = 0; i < len; i++) {
                sum += bishops[i];
            }
            if (sum === 0 || sum === len) {
                return true;
            }
        }
        return false;
    }
    isThreefoldRepetition() {
        return this._getPositionCount(this._hash) >= 3;
    }
    isDrawByFiftyMoves() {
        return this._halfMoves >= 100; // 50 moves per side = 100 half moves
    }
    isDraw() {
        return (this.isDrawByFiftyMoves() ||
            this.isStalemate() ||
            this.isInsufficientMaterial() ||
            this.isThreefoldRepetition());
    }
    isGameOver() {
        return this.isCheckmate() || this.isDraw();
    }
    moves({ verbose = false, square = undefined, piece = undefined, } = {}) {
        const moves = this._moves({ square, piece });
        if (verbose) {
            return moves.map((move) => new Move(this, move));
        }
        else {
            return moves.map((move) => this._moveToSan(move, moves));
        }
    }
    _moves({ legal = true, piece = undefined, square = undefined, } = {}) {
        const forSquare = square ? square.toLowerCase() : undefined;
        const forPiece = piece?.toLowerCase();
        const moves = [];
        const us = this._turn;
        const them = swapColor(us);
        let firstSquare = Ox88.a8;
        let lastSquare = Ox88.h1;
        let singleSquare = false;
        // are we generating moves for a single square?
        if (forSquare) {
            // illegal square, return empty moves
            if (!(forSquare in Ox88)) {
                return [];
            }
            else {
                firstSquare = lastSquare = Ox88[forSquare];
                singleSquare = true;
            }
        }
        for (let from = firstSquare; from <= lastSquare; from++) {
            // did we run off the end of the board
            if (from & 0x88) {
                from += 7;
                continue;
            }
            // empty square or opponent, skip
            if (!this._board[from] || this._board[from].color === them) {
                continue;
            }
            const { type } = this._board[from];
            let to;
            if (type === PAWN) {
                if (forPiece && forPiece !== type)
                    continue;
                // single square, non-capturing
                to = from + PAWN_OFFSETS[us][0];
                if (!this._board[to]) {
                    addMove(moves, us, from, to, PAWN);
                    // double square
                    to = from + PAWN_OFFSETS[us][1];
                    if (SECOND_RANK[us] === rank(from) && !this._board[to]) {
                        addMove(moves, us, from, to, PAWN, undefined, BITS.BIG_PAWN);
                    }
                }
                // pawn captures
                for (let j = 2; j < 4; j++) {
                    to = from + PAWN_OFFSETS[us][j];
                    if (to & 0x88)
                        continue;
                    if (this._board[to]?.color === them) {
                        addMove(moves, us, from, to, PAWN, this._board[to].type, BITS.CAPTURE);
                    }
                    else if (to === this._epSquare) {
                        addMove(moves, us, from, to, PAWN, PAWN, BITS.EP_CAPTURE);
                    }
                }
            }
            else {
                if (forPiece && forPiece !== type)
                    continue;
                for (let j = 0, len = PIECE_OFFSETS[type].length; j < len; j++) {
                    const offset = PIECE_OFFSETS[type][j];
                    to = from;
                    while (true) {
                        to += offset;
                        if (to & 0x88)
                            break;
                        if (!this._board[to]) {
                            addMove(moves, us, from, to, type);
                        }
                        else {
                            // own color, stop loop
                            if (this._board[to].color === us)
                                break;
                            addMove(moves, us, from, to, type, this._board[to].type, BITS.CAPTURE);
                            break;
                        }
                        /* break, if knight or king */
                        if (type === KNIGHT || type === KING)
                            break;
                    }
                }
            }
        }
        /*
         * check for castling if we're:
         *   a) generating all moves, or
         *   b) doing single square move generation on the king's square
         */
        if (forPiece === undefined || forPiece === KING) {
            if (!singleSquare || lastSquare === this._kings[us]) {
                // king-side castling
                if (this._castling[us] & BITS.KSIDE_CASTLE) {
                    const castlingFrom = this._kings[us];
                    const castlingTo = castlingFrom + 2;
                    if (!this._board[castlingFrom + 1] &&
                        !this._board[castlingTo] &&
                        !this._attacked(them, this._kings[us]) &&
                        !this._attacked(them, castlingFrom + 1) &&
                        !this._attacked(them, castlingTo)) {
                        addMove(moves, us, this._kings[us], castlingTo, KING, undefined, BITS.KSIDE_CASTLE);
                    }
                }
                // queen-side castling
                if (this._castling[us] & BITS.QSIDE_CASTLE) {
                    const castlingFrom = this._kings[us];
                    const castlingTo = castlingFrom - 2;
                    if (!this._board[castlingFrom - 1] &&
                        !this._board[castlingFrom - 2] &&
                        !this._board[castlingFrom - 3] &&
                        !this._attacked(them, this._kings[us]) &&
                        !this._attacked(them, castlingFrom - 1) &&
                        !this._attacked(them, castlingTo)) {
                        addMove(moves, us, this._kings[us], castlingTo, KING, undefined, BITS.QSIDE_CASTLE);
                    }
                }
            }
        }
        /*
         * return all pseudo-legal moves (this includes moves that allow the king
         * to be captured)
         */
        if (!legal || this._kings[us] === -1) {
            return moves;
        }
        // filter out illegal moves
        const legalMoves = [];
        for (let i = 0, len = moves.length; i < len; i++) {
            this._makeMove(moves[i]);
            if (!this._isKingAttacked(us)) {
                legalMoves.push(moves[i]);
            }
            this._undoMove();
        }
        return legalMoves;
    }
    move(move, { strict = false } = {}) {
        /*
         * The move function can be called with in the following parameters:
         *
         * .move('Nxb7')       <- argument is a case-sensitive SAN string
         *
         * .move({ from: 'h7', <- argument is a move object
         *         to :'h8',
         *         promotion: 'q' })
         *
         *
         * An optional strict argument may be supplied to tell chess.js to
         * strictly follow the SAN specification.
         */
        let moveObj = null;
        if (typeof move === 'string') {
            moveObj = this._moveFromSan(move, strict);
        }
        else if (move === null) {
            moveObj = this._moveFromSan(SAN_NULLMOVE, strict);
        }
        else if (typeof move === 'object') {
            const moves = this._moves();
            // convert the pretty move object to an ugly move object
            for (let i = 0, len = moves.length; i < len; i++) {
                if (move.from === algebraic(moves[i].from) &&
                    move.to === algebraic(moves[i].to) &&
                    (!('promotion' in moves[i]) || move.promotion === moves[i].promotion)) {
                    moveObj = moves[i];
                    break;
                }
            }
        }
        // failed to find move
        if (!moveObj) {
            if (typeof move === 'string') {
                throw new Error(`Invalid move: ${move}`);
            }
            else {
                throw new Error(`Invalid move: ${JSON.stringify(move)}`);
            }
        }
        //disallow null moves when in check
        if (this.isCheck() && moveObj.flags & BITS.NULL_MOVE) {
            throw new Error('Null move not allowed when in check');
        }
        /*
         * need to make a copy of move because we can't generate SAN after the move
         * is made
         */
        const prettyMove = new Move(this, moveObj);
        this._makeMove(moveObj);
        this._incPositionCount();
        return prettyMove;
    }
    _push(move) {
        this._history.push({
            move,
            kings: { b: this._kings.b, w: this._kings.w },
            turn: this._turn,
            castling: { b: this._castling.b, w: this._castling.w },
            epSquare: this._epSquare,
            halfMoves: this._halfMoves,
            moveNumber: this._moveNumber,
        });
    }
    _movePiece(from, to) {
        this._hash ^= this._pieceKey(from);
        this._board[to] = this._board[from];
        delete this._board[from];
        this._hash ^= this._pieceKey(to);
    }
    _makeMove(move) {
        const us = this._turn;
        const them = swapColor(us);
        this._push(move);
        if (move.flags & BITS.NULL_MOVE) {
            if (us === BLACK) {
                this._moveNumber++;
            }
            this._halfMoves++;
            this._turn = them;
            this._epSquare = EMPTY;
            return;
        }
        this._hash ^= this._epKey();
        this._hash ^= this._castlingKey();
        if (move.captured) {
            this._hash ^= this._pieceKey(move.to);
        }
        this._movePiece(move.from, move.to);
        // if ep capture, remove the captured pawn
        if (move.flags & BITS.EP_CAPTURE) {
            if (this._turn === BLACK) {
                this._clear(move.to - 16);
            }
            else {
                this._clear(move.to + 16);
            }
        }
        // if pawn promotion, replace with new piece
        if (move.promotion) {
            this._clear(move.to);
            this._set(move.to, { type: move.promotion, color: us });
        }
        // if we moved the king
        if (this._board[move.to].type === KING) {
            this._kings[us] = move.to;
            // if we castled, move the rook next to the king
            if (move.flags & BITS.KSIDE_CASTLE) {
                const castlingTo = move.to - 1;
                const castlingFrom = move.to + 1;
                this._movePiece(castlingFrom, castlingTo);
            }
            else if (move.flags & BITS.QSIDE_CASTLE) {
                const castlingTo = move.to + 1;
                const castlingFrom = move.to - 2;
                this._movePiece(castlingFrom, castlingTo);
            }
            // turn off castling
            this._castling[us] = 0;
        }
        // turn off castling if we move a rook
        if (this._castling[us]) {
            for (let i = 0, len = ROOKS[us].length; i < len; i++) {
                if (move.from === ROOKS[us][i].square &&
                    this._castling[us] & ROOKS[us][i].flag) {
                    this._castling[us] ^= ROOKS[us][i].flag;
                    break;
                }
            }
        }
        // turn off castling if we capture a rook
        if (this._castling[them]) {
            for (let i = 0, len = ROOKS[them].length; i < len; i++) {
                if (move.to === ROOKS[them][i].square &&
                    this._castling[them] & ROOKS[them][i].flag) {
                    this._castling[them] ^= ROOKS[them][i].flag;
                    break;
                }
            }
        }
        this._hash ^= this._castlingKey();
        // if big pawn move, update the en passant square
        if (move.flags & BITS.BIG_PAWN) {
            let epSquare;
            if (us === BLACK) {
                epSquare = move.to - 16;
            }
            else {
                epSquare = move.to + 16;
            }
            if ((!((move.to - 1) & 0x88) &&
                this._board[move.to - 1]?.type === PAWN &&
                this._board[move.to - 1]?.color === them) ||
                (!((move.to + 1) & 0x88) &&
                    this._board[move.to + 1]?.type === PAWN &&
                    this._board[move.to + 1]?.color === them)) {
                this._epSquare = epSquare;
                this._hash ^= this._epKey();
            }
            else {
                this._epSquare = EMPTY;
            }
        }
        else {
            this._epSquare = EMPTY;
        }
        // reset the 50 move counter if a pawn is moved or a piece is captured
        if (move.piece === PAWN) {
            this._halfMoves = 0;
        }
        else if (move.flags & (BITS.CAPTURE | BITS.EP_CAPTURE)) {
            this._halfMoves = 0;
        }
        else {
            this._halfMoves++;
        }
        if (us === BLACK) {
            this._moveNumber++;
        }
        this._turn = them;
        this._hash ^= SIDE_KEY;
    }
    undo() {
        const hash = this._hash;
        const move = this._undoMove();
        if (move) {
            const prettyMove = new Move(this, move);
            this._decPositionCount(hash);
            return prettyMove;
        }
        return null;
    }
    _undoMove() {
        const old = this._history.pop();
        if (old === undefined) {
            return null;
        }
        this._hash ^= this._epKey();
        this._hash ^= this._castlingKey();
        const move = old.move;
        this._kings = old.kings;
        this._turn = old.turn;
        this._castling = old.castling;
        this._epSquare = old.epSquare;
        this._halfMoves = old.halfMoves;
        this._moveNumber = old.moveNumber;
        this._hash ^= this._epKey();
        this._hash ^= this._castlingKey();
        this._hash ^= SIDE_KEY;
        const us = this._turn;
        const them = swapColor(us);
        if (move.flags & BITS.NULL_MOVE) {
            return move;
        }
        this._movePiece(move.to, move.from);
        // to undo any promotions
        if (move.piece) {
            this._clear(move.from);
            this._set(move.from, { type: move.piece, color: us });
        }
        if (move.captured) {
            if (move.flags & BITS.EP_CAPTURE) {
                // en passant capture
                let index;
                if (us === BLACK) {
                    index = move.to - 16;
                }
                else {
                    index = move.to + 16;
                }
                this._set(index, { type: PAWN, color: them });
            }
            else {
                // regular capture
                this._set(move.to, { type: move.captured, color: them });
            }
        }
        if (move.flags & (BITS.KSIDE_CASTLE | BITS.QSIDE_CASTLE)) {
            let castlingTo, castlingFrom;
            if (move.flags & BITS.KSIDE_CASTLE) {
                castlingTo = move.to + 1;
                castlingFrom = move.to - 1;
            }
            else {
                castlingTo = move.to - 2;
                castlingFrom = move.to + 1;
            }
            this._movePiece(castlingFrom, castlingTo);
        }
        return move;
    }
    pgn({ newline = '\n', maxWidth = 0, } = {}) {
        /*
         * using the specification from http://www.chessclub.com/help/PGN-spec
         * example for html usage: .pgn({ max_width: 72, newline_char: "<br />" })
         */
        const result = [];
        let headerExists = false;
        /* add the PGN header information */
        for (const i in this._header) {
            /*
             * TODO: order of enumerated properties in header object is not
             * guaranteed, see ECMA-262 spec (section 12.6.4)
             *
             * By using HEADER_TEMPLATE, the order of tags should be preserved; we
             * do have to check for null placeholders, though, and omit them
             */
            const headerTag = this._header[i];
            if (headerTag)
                result.push(`[${i} "${this._header[i]}"]` + newline);
            headerExists = true;
        }
        if (headerExists && this._history.length) {
            result.push(newline);
        }
        const appendComment = (moveString) => {
            const comment = this._comments[this.fen()];
            if (typeof comment !== 'undefined') {
                const delimiter = moveString.length > 0 ? ' ' : '';
                moveString = `${moveString}${delimiter}{${comment}}`;
            }
            return moveString;
        };
        // pop all of history onto reversed_history
        const reversedHistory = [];
        while (this._history.length > 0) {
            reversedHistory.push(this._undoMove());
        }
        const moves = [];
        let moveString = '';
        // special case of a commented starting position with no moves
        if (reversedHistory.length === 0) {
            moves.push(appendComment(''));
        }
        // build the list of moves.  a move_string looks like: "3. e3 e6"
        while (reversedHistory.length > 0) {
            moveString = appendComment(moveString);
            const move = reversedHistory.pop();
            // make TypeScript stop complaining about move being undefined
            if (!move) {
                break;
            }
            // if the position started with black to move, start PGN with #. ...
            if (!this._history.length && move.color === 'b') {
                const prefix = `${this._moveNumber}. ...`;
                // is there a comment preceding the first move?
                moveString = moveString ? `${moveString} ${prefix}` : prefix;
            }
            else if (move.color === 'w') {
                // store the previous generated move_string if we have one
                if (moveString.length) {
                    moves.push(moveString);
                }
                moveString = this._moveNumber + '.';
            }
            moveString =
                moveString + ' ' + this._moveToSan(move, this._moves({ legal: true }));
            this._makeMove(move);
        }
        // are there any other leftover moves?
        if (moveString.length) {
            moves.push(appendComment(moveString));
        }
        // is there a result? (there ALWAYS has to be a result according to spec; see Seven Tag Roster)
        moves.push(this._header.Result || '*');
        /*
         * history should be back to what it was before we started generating PGN,
         * so join together moves
         */
        if (maxWidth === 0) {
            return result.join('') + moves.join(' ');
        }
        // TODO (jah): huh?
        const strip = function () {
            if (result.length > 0 && result[result.length - 1] === ' ') {
                result.pop();
                return true;
            }
            return false;
        };
        // NB: this does not preserve comment whitespace.
        const wrapComment = function (width, move) {
            for (const token of move.split(' ')) {
                if (!token) {
                    continue;
                }
                if (width + token.length > maxWidth) {
                    while (strip()) {
                        width--;
                    }
                    result.push(newline);
                    width = 0;
                }
                result.push(token);
                width += token.length;
                result.push(' ');
                width++;
            }
            if (strip()) {
                width--;
            }
            return width;
        };
        // wrap the PGN output at max_width
        let currentWidth = 0;
        for (let i = 0; i < moves.length; i++) {
            if (currentWidth + moves[i].length > maxWidth) {
                if (moves[i].includes('{')) {
                    currentWidth = wrapComment(currentWidth, moves[i]);
                    continue;
                }
            }
            // if the current move will push past max_width
            if (currentWidth + moves[i].length > maxWidth && i !== 0) {
                // don't end the line with whitespace
                if (result[result.length - 1] === ' ') {
                    result.pop();
                }
                result.push(newline);
                currentWidth = 0;
            }
            else if (i !== 0) {
                result.push(' ');
                currentWidth++;
            }
            result.push(moves[i]);
            currentWidth += moves[i].length;
        }
        return result.join('');
    }
    /**
     * @deprecated Use `setHeader` and `getHeaders` instead. This method will return null header tags (which is not what you want)
     */
    header(...args) {
        for (let i = 0; i < args.length; i += 2) {
            if (typeof args[i] === 'string' && typeof args[i + 1] === 'string') {
                this._header[args[i]] = args[i + 1];
            }
        }
        return this._header;
    }
    // TODO: value validation per spec
    setHeader(key, value) {
        this._header[key] = value ?? SEVEN_TAG_ROSTER[key] ?? null;
        return this.getHeaders();
    }
    removeHeader(key) {
        if (key in this._header) {
            this._header[key] = SEVEN_TAG_ROSTER[key] || null;
            return true;
        }
        return false;
    }
    // return only non-null headers (omit placemarker nulls)
    getHeaders() {
        const nonNullHeaders = {};
        for (const [key, value] of Object.entries(this._header)) {
            if (value !== null) {
                nonNullHeaders[key] = value;
            }
        }
        return nonNullHeaders;
    }
    loadPgn(pgn, { strict = false, newlineChar = '\r?\n', } = {}) {
        // If newlineChar is not the default, replace all instances with \n
        if (newlineChar !== '\r?\n') {
            pgn = pgn.replace(new RegExp(newlineChar, 'g'), '\n');
        }
        const parsedPgn = peg$parse(pgn);
        // Put the board in the starting position
        this.reset();
        // parse PGN header
        const headers = parsedPgn.headers;
        let fen = '';
        for (const key in headers) {
            // check to see user is including fen (possibly with wrong tag case)
            if (key.toLowerCase() === 'fen') {
                fen = headers[key];
            }
            this.header(key, headers[key]);
        }
        /*
         * the permissive parser should attempt to load a fen tag, even if it's the
         * wrong case and doesn't include a corresponding [SetUp "1"] tag
         */
        if (!strict) {
            if (fen) {
                this.load(fen, { preserveHeaders: true });
            }
        }
        else {
            /*
             * strict parser - load the starting position indicated by [Setup '1']
             * and [FEN position]
             */
            if (headers['SetUp'] === '1') {
                if (!('FEN' in headers)) {
                    throw new Error('Invalid PGN: FEN tag must be supplied with SetUp tag');
                }
                // don't clear the headers when loading
                this.load(headers['FEN'], { preserveHeaders: true });
            }
        }
        let node = parsedPgn.root;
        while (node) {
            if (node.move) {
                const move = this._moveFromSan(node.move, strict);
                if (move == null) {
                    throw new Error(`Invalid move in PGN: ${node.move}`);
                }
                else {
                    this._makeMove(move);
                    this._incPositionCount();
                }
            }
            if (node.comment !== undefined) {
                this._comments[this.fen()] = node.comment;
            }
            node = node.variations[0];
        }
        /*
         * Per section 8.2.6 of the PGN spec, the Result tag pair must match match
         * the termination marker. Only do this when headers are present, but the
         * result tag is missing
         */
        const result = parsedPgn.result;
        if (result &&
            Object.keys(this._header).length &&
            this._header['Result'] !== result) {
            this.setHeader('Result', result);
        }
    }
    /*
     * Convert a move from 0x88 coordinates to Standard Algebraic Notation
     * (SAN)
     *
     * @param {boolean} strict Use the strict SAN parser. It will throw errors
     * on overly disambiguated moves (see below):
     *
     * r1bqkbnr/ppp2ppp/2n5/1B1pP3/4P3/8/PPPP2PP/RNBQK1NR b KQkq - 2 4
     * 4. ... Nge7 is overly disambiguated because the knight on c6 is pinned
     * 4. ... Ne7 is technically the valid SAN
     */
    _moveToSan(move, moves) {
        let output = '';
        if (move.flags & BITS.KSIDE_CASTLE) {
            output = 'O-O';
        }
        else if (move.flags & BITS.QSIDE_CASTLE) {
            output = 'O-O-O';
        }
        else if (move.flags & BITS.NULL_MOVE) {
            return SAN_NULLMOVE;
        }
        else {
            if (move.piece !== PAWN) {
                const disambiguator = getDisambiguator(move, moves);
                output += move.piece.toUpperCase() + disambiguator;
            }
            if (move.flags & (BITS.CAPTURE | BITS.EP_CAPTURE)) {
                if (move.piece === PAWN) {
                    output += algebraic(move.from)[0];
                }
                output += 'x';
            }
            output += algebraic(move.to);
            if (move.promotion) {
                output += '=' + move.promotion.toUpperCase();
            }
        }
        this._makeMove(move);
        if (this.isCheck()) {
            if (this.isCheckmate()) {
                output += '#';
            }
            else {
                output += '+';
            }
        }
        this._undoMove();
        return output;
    }
    // convert a move from Standard Algebraic Notation (SAN) to 0x88 coordinates
    _moveFromSan(move, strict = false) {
        // strip off any move decorations: e.g Nf3+?! becomes Nf3
        let cleanMove = strippedSan(move);
        if (!strict) {
            if (cleanMove === '0-0') {
                cleanMove = 'O-O';
            }
            else if (cleanMove === '0-0-0') {
                cleanMove = 'O-O-O';
            }
        }
        //first implementation of null with a dummy move (black king moves from a8 to a8), maybe this can be implemented better
        if (cleanMove == SAN_NULLMOVE) {
            const res = {
                color: this._turn,
                from: 0,
                to: 0,
                piece: 'k',
                flags: BITS.NULL_MOVE,
            };
            return res;
        }
        let pieceType = inferPieceType(cleanMove);
        let moves = this._moves({ legal: true, piece: pieceType });
        // strict parser
        for (let i = 0, len = moves.length; i < len; i++) {
            if (cleanMove === strippedSan(this._moveToSan(moves[i], moves))) {
                return moves[i];
            }
        }
        // the strict parser failed
        if (strict) {
            return null;
        }
        let piece = undefined;
        let matches = undefined;
        let from = undefined;
        let to = undefined;
        let promotion = undefined;
        /*
         * The default permissive (non-strict) parser allows the user to parse
         * non-standard chess notations. This parser is only run after the strict
         * Standard Algebraic Notation (SAN) parser has failed.
         *
         * When running the permissive parser, we'll run a regex to grab the piece, the
         * to/from square, and an optional promotion piece. This regex will
         * parse common non-standard notation like: Pe2-e4, Rc1c4, Qf3xf7,
         * f7f8q, b1c3
         *
         * NOTE: Some positions and moves may be ambiguous when using the permissive
         * parser. For example, in this position: 6k1/8/8/B7/8/8/8/BN4K1 w - - 0 1,
         * the move b1c3 may be interpreted as Nc3 or B1c3 (a disambiguated bishop
         * move). In these cases, the permissive parser will default to the most
         * basic interpretation (which is b1c3 parsing to Nc3).
         */
        let overlyDisambiguated = false;
        matches = cleanMove.match(/([pnbrqkPNBRQK])?([a-h][1-8])x?-?([a-h][1-8])([qrbnQRBN])?/);
        if (matches) {
            piece = matches[1];
            from = matches[2];
            to = matches[3];
            promotion = matches[4];
            if (from.length == 1) {
                overlyDisambiguated = true;
            }
        }
        else {
            /*
             * The [a-h]?[1-8]? portion of the regex below handles moves that may be
             * overly disambiguated (e.g. Nge7 is unnecessary and non-standard when
             * there is one legal knight move to e7). In this case, the value of
             * 'from' variable will be a rank or file, not a square.
             */
            matches = cleanMove.match(/([pnbrqkPNBRQK])?([a-h]?[1-8]?)x?-?([a-h][1-8])([qrbnQRBN])?/);
            if (matches) {
                piece = matches[1];
                from = matches[2];
                to = matches[3];
                promotion = matches[4];
                if (from.length == 1) {
                    overlyDisambiguated = true;
                }
            }
        }
        pieceType = inferPieceType(cleanMove);
        moves = this._moves({
            legal: true,
            piece: piece ? piece : pieceType,
        });
        if (!to) {
            return null;
        }
        for (let i = 0, len = moves.length; i < len; i++) {
            if (!from) {
                // if there is no from square, it could be just 'x' missing from a capture
                if (cleanMove ===
                    strippedSan(this._moveToSan(moves[i], moves)).replace('x', '')) {
                    return moves[i];
                }
                // hand-compare move properties with the results from our permissive regex
            }
            else if ((!piece || piece.toLowerCase() == moves[i].piece) &&
                Ox88[from] == moves[i].from &&
                Ox88[to] == moves[i].to &&
                (!promotion || promotion.toLowerCase() == moves[i].promotion)) {
                return moves[i];
            }
            else if (overlyDisambiguated) {
                /*
                 * SPECIAL CASE: we parsed a move string that may have an unneeded
                 * rank/file disambiguator (e.g. Nge7).  The 'from' variable will
                 */
                const square = algebraic(moves[i].from);
                if ((!piece || piece.toLowerCase() == moves[i].piece) &&
                    Ox88[to] == moves[i].to &&
                    (from == square[0] || from == square[1]) &&
                    (!promotion || promotion.toLowerCase() == moves[i].promotion)) {
                    return moves[i];
                }
            }
        }
        return null;
    }
    ascii() {
        let s = '   +------------------------+\n';
        for (let i = Ox88.a8; i <= Ox88.h1; i++) {
            // display the rank
            if (file(i) === 0) {
                s += ' ' + '87654321'[rank(i)] + ' |';
            }
            if (this._board[i]) {
                const piece = this._board[i].type;
                const color = this._board[i].color;
                const symbol = color === WHITE ? piece.toUpperCase() : piece.toLowerCase();
                s += ' ' + symbol + ' ';
            }
            else {
                s += ' . ';
            }
            if ((i + 1) & 0x88) {
                s += '|\n';
                i += 8;
            }
        }
        s += '   +------------------------+\n';
        s += '     a  b  c  d  e  f  g  h';
        return s;
    }
    perft(depth) {
        const moves = this._moves({ legal: false });
        let nodes = 0;
        const color = this._turn;
        for (let i = 0, len = moves.length; i < len; i++) {
            this._makeMove(moves[i]);
            if (!this._isKingAttacked(color)) {
                if (depth - 1 > 0) {
                    nodes += this.perft(depth - 1);
                }
                else {
                    nodes++;
                }
            }
            this._undoMove();
        }
        return nodes;
    }
    setTurn(color) {
        if (this._turn == color) {
            return false;
        }
        this.move('--');
        return true;
    }
    turn() {
        return this._turn;
    }
    board() {
        const output = [];
        let row = [];
        for (let i = Ox88.a8; i <= Ox88.h1; i++) {
            if (this._board[i] == null) {
                row.push(null);
            }
            else {
                row.push({
                    square: algebraic(i),
                    type: this._board[i].type,
                    color: this._board[i].color,
                });
            }
            if ((i + 1) & 0x88) {
                output.push(row);
                row = [];
                i += 8;
            }
        }
        return output;
    }
    squareColor(square) {
        if (square in Ox88) {
            const sq = Ox88[square];
            return (rank(sq) + file(sq)) % 2 === 0 ? 'light' : 'dark';
        }
        return null;
    }
    history({ verbose = false } = {}) {
        const reversedHistory = [];
        const moveHistory = [];
        while (this._history.length > 0) {
            reversedHistory.push(this._undoMove());
        }
        while (true) {
            const move = reversedHistory.pop();
            if (!move) {
                break;
            }
            if (verbose) {
                moveHistory.push(new Move(this, move));
            }
            else {
                moveHistory.push(this._moveToSan(move, this._moves()));
            }
            this._makeMove(move);
        }
        return moveHistory;
    }
    /*
     * Keeps track of position occurrence counts for the purpose of repetition
     * checking. Old positions are removed from the map if their counts are reduced to 0.
     */
    _getPositionCount(hash) {
        return this._positionCount.get(hash) ?? 0;
    }
    _incPositionCount() {
        this._positionCount.set(this._hash, (this._positionCount.get(this._hash) ?? 0) + 1);
    }
    _decPositionCount(hash) {
        const currentCount = this._positionCount.get(hash) ?? 0;
        if (currentCount === 1) {
            this._positionCount.delete(hash);
        }
        else {
            this._positionCount.set(hash, currentCount - 1);
        }
    }
    _pruneComments() {
        const reversedHistory = [];
        const currentComments = {};
        const copyComment = (fen) => {
            if (fen in this._comments) {
                currentComments[fen] = this._comments[fen];
            }
        };
        while (this._history.length > 0) {
            reversedHistory.push(this._undoMove());
        }
        copyComment(this.fen());
        while (true) {
            const move = reversedHistory.pop();
            if (!move) {
                break;
            }
            this._makeMove(move);
            copyComment(this.fen());
        }
        this._comments = currentComments;
    }
    getComment() {
        return this._comments[this.fen()];
    }
    setComment(comment) {
        this._comments[this.fen()] = comment.replace('{', '[').replace('}', ']');
    }
    /**
     * @deprecated Renamed to `removeComment` for consistency
     */
    deleteComment() {
        return this.removeComment();
    }
    removeComment() {
        const comment = this._comments[this.fen()];
        delete this._comments[this.fen()];
        return comment;
    }
    getComments() {
        this._pruneComments();
        return Object.keys(this._comments).map((fen) => {
            return { fen: fen, comment: this._comments[fen] };
        });
    }
    /**
     * @deprecated Renamed to `removeComments` for consistency
     */
    deleteComments() {
        return this.removeComments();
    }
    removeComments() {
        this._pruneComments();
        return Object.keys(this._comments).map((fen) => {
            const comment = this._comments[fen];
            delete this._comments[fen];
            return { fen: fen, comment: comment };
        });
    }
    setCastlingRights(color, rights) {
        for (const side of [KING, QUEEN]) {
            if (rights[side] !== undefined) {
                if (rights[side]) {
                    this._castling[color] |= SIDES[side];
                }
                else {
                    this._castling[color] &= ~SIDES[side];
                }
            }
        }
        this._updateCastlingRights();
        const result = this.getCastlingRights(color);
        return ((rights[KING] === undefined || rights[KING] === result[KING]) &&
            (rights[QUEEN] === undefined || rights[QUEEN] === result[QUEEN]));
    }
    getCastlingRights(color) {
        return {
            [KING]: (this._castling[color] & SIDES[KING]) !== 0,
            [QUEEN]: (this._castling[color] & SIDES[QUEEN]) !== 0,
        };
    }
    moveNumber() {
        return this._moveNumber;
    }
}

export { BORDER_TYPE, COLOR, Chess, Chessboard, INPUT_EVENT_TYPE, MARKER_TYPE, Markers, PromotionDialog };
