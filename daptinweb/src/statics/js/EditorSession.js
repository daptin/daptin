/**
 * Copyright (C) 2013 KO GmbH <copyright@kogmbh.com>
 *
 * @licstart
 * This file is part of WebODF.
 *
 * WebODF is free software: you can redistribute it and/or modify it
 * under the terms of the GNU Affero General Public License (GNU AGPL)
 * as published by the Free Software Foundation, either version 3 of
 * the License, or (at your option) any later version.
 *
 * WebODF is distributed in the hope that it will be useful, but
 * WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with WebODF.  If not, see <http://www.gnu.org/licenses/>.
 * @licend
 *
 * @source: http://www.webodf.org/
 * @source: https://github.com/kogmbh/WebODF/
 */

/*global runtime, define, document, core, odf, gui, ops*/

define("webodf/editor/EditorSession", [
    "dojo/text!resources/fonts/fonts.css"
], function (fontsCSS) { // fontsCSS is retrieved as a string, using dojo's text retrieval AMD plugin
    "use strict";

    runtime.loadClass("core.Async");
    runtime.loadClass("core.DomUtils");
    runtime.loadClass("odf.OdfUtils");
    runtime.loadClass("ops.OdtDocument");
    runtime.loadClass("ops.OdtStepsTranslator");
    runtime.loadClass("ops.Session");
    runtime.loadClass("odf.Namespaces");
    runtime.loadClass("odf.OdfCanvas");
    runtime.loadClass("odf.OdfUtils");
    runtime.loadClass("gui.CaretManager");
    runtime.loadClass("gui.Caret");
    runtime.loadClass("gui.OdfFieldView");
    runtime.loadClass("gui.SessionController");
    runtime.loadClass("gui.SessionView");
    runtime.loadClass("gui.HyperlinkTooltipView");
    runtime.loadClass("gui.TrivialUndoManager");
    runtime.loadClass("gui.SvgSelectionView");
    runtime.loadClass("gui.SelectionViewManager");
    runtime.loadClass("core.EventNotifier");
    runtime.loadClass("gui.ShadowCursor");
    runtime.loadClass("gui.CommonConstraints");

    /**
     * Instantiate a new editor session attached to an existing operation session
     * @constructor
     * @implements {core.EventSource}
     * @param {!ops.Session} session
     * @param {!string} localMemberId
     * @param {{viewOptions:gui.SessionViewOptions,directParagraphStylingEnabled:boolean,annotationsEnabled:boolean}} config
     */
    var EditorSession = function EditorSession(session, localMemberId, config) {
        var self = this,
            currentParagraphNode = null,
            currentCommonStyleName = null,
            currentStyleName = null,
            caretManager,
            selectionViewManager,
            hyperlinkTooltipView,
            odtDocument = session.getOdtDocument(),
            textns = odf.Namespaces.textns,
            fontStyles = document.createElement('style'),
            formatting = odtDocument.getFormatting(),
            domUtils = webodfcore.DomUtils,
            odfUtils = odf.OdfUtils,
            odfFieldView,
            eventNotifier = new webodfcore.EventNotifier([
                EditorSession.signalMemberAdded,
                EditorSession.signalMemberUpdated,
                EditorSession.signalMemberRemoved,
                EditorSession.signalCursorAdded,
                EditorSession.signalCursorMoved,
                EditorSession.signalCursorRemoved,
                EditorSession.signalParagraphChanged,
                EditorSession.signalCommonStyleCreated,
                EditorSession.signalCommonStyleDeleted,
                EditorSession.signalParagraphStyleModified,
                EditorSession.signalUndoStackChanged]),
            shadowCursor = new gui.ShadowCursor(odtDocument),
            sessionConstraints,
            /**@const*/
            NEXT = webodfcore.StepDirection.NEXT;

        /**
         * @return {Array.<!string>}
         */
        function getAvailableFonts() {
            var availableFonts, regex, matches;

            availableFonts = {};

            /*jslint regexp: true*/
            regex =  /font-family *: *(?:\'([^']*)\'|\"([^"]*)\")/gm;
            /*jslint regexp: false*/
            matches = regex.exec(fontsCSS);

            while (matches) {
                availableFonts[matches[1] || matches[2]] = 1;
                matches = regex.exec(fontsCSS);
            }
            availableFonts = Object.keys(availableFonts);

            return availableFonts;
        }

        function checkParagraphStyleName() {
            var newStyleName,
                newCommonStyleName;

            newStyleName = currentParagraphNode.getAttributeNS(textns, 'style-name');

            if (newStyleName !== currentStyleName) {
                currentStyleName = newStyleName;
                // check if common style is still the same
                newCommonStyleName = formatting.getFirstCommonParentStyleNameOrSelf(newStyleName);
                if (!newCommonStyleName) {
                    // Default style, empty-string name
                    currentCommonStyleName = newStyleName = currentStyleName = "";
                    self.emit(EditorSession.signalParagraphChanged, {
                        type: 'style',
                        node: currentParagraphNode,
                        styleName: currentCommonStyleName
                    });
                    return;
                }
                // a common style
                if (newCommonStyleName !== currentCommonStyleName) {
                    currentCommonStyleName = newCommonStyleName;
                    self.emit(EditorSession.signalParagraphChanged, {
                        type: 'style',
                        node: currentParagraphNode,
                        styleName: currentCommonStyleName
                    });
                }
            }
        }
        /**
         * Creates a NCName from the passed string
         * @param {!string} name
         * @return {!string}
         */
        function createNCName(name) {
            var letter,
                result = "",
                i;

            // encode
            for (i = 0; i < name.length; i += 1) {
                letter = name[i];
                // simple approach, can be improved to not skip other allowed chars
                if (letter.match(/[a-zA-Z0-9.-_]/) !== null) {
                    result += letter;
                } else {
                    result += "_" + letter.charCodeAt(0).toString(16) + "_";
                }
            }
            // ensure leading char is from proper range
            if (result.match(/^[a-zA-Z_]/) === null) {
                result = "_" + result;
            }

            return result;
        }

        function uniqueParagraphStyleNCName(name) {
            var result,
                i = 0,
                ncMemberId = createNCName(localMemberId),
                ncName = createNCName(name);

            // create default paragraph style
            // localMemberId is used to avoid id conflicts with ids created by other members
            result = ncName + "_" + ncMemberId;
            // then loop until result is really unique
            while (formatting.hasParagraphStyle(result)) {
                result = ncName + "_" + i + "_" + ncMemberId;
                i += 1;
            }

            return result;
        }

        function trackCursor(cursor) {
            var node;

            node = odfUtils.getParagraphElement(cursor.getNode());
            if (!node) {
                return;
            }
            currentParagraphNode = node;
            checkParagraphStyleName();
        }

        function trackCurrentParagraph(info) {
            var cursor = odtDocument.getCursor(localMemberId),
                range = cursor && cursor.getSelectedRange(),
                paragraphRange = odtDocument.getDOMDocument().createRange();
            paragraphRange.selectNode(info.paragraphElement);
            if ((range && domUtils.rangesIntersect(range, paragraphRange)) || info.paragraphElement === currentParagraphNode) {
                self.emit(EditorSession.signalParagraphChanged, info);
                checkParagraphStyleName();
            }
            paragraphRange.detach();
        }

        function onMemberAdded(member) {
            self.emit(EditorSession.signalMemberAdded, member.getMemberId());
        }

        function onMemberUpdated(member) {
            self.emit(EditorSession.signalMemberUpdated, member.getMemberId());
        }

        function onMemberRemoved(memberId) {
            self.emit(EditorSession.signalMemberRemoved, memberId);
        }

        function onCursorAdded(cursor) {
            self.emit(EditorSession.signalCursorAdded, cursor.getMemberId());
            trackCursor(cursor);
        }

        function onCursorRemoved(memberId) {
            self.emit(EditorSession.signalCursorRemoved, memberId);
        }

        function onCursorMoved(cursor) {
            // Emit 'cursorMoved' only when *I* am moving the cursor, not the other users
            if (cursor.getMemberId() === localMemberId) {
                self.emit(EditorSession.signalCursorMoved, cursor);
                trackCursor(cursor);
            }
        }

        function onStyleCreated(newStyleName) {
            self.emit(EditorSession.signalCommonStyleCreated, newStyleName);
        }

        function onStyleDeleted(styleName) {
            self.emit(EditorSession.signalCommonStyleDeleted, styleName);
        }

        function onParagraphStyleModified(styleName) {
            self.emit(EditorSession.signalParagraphStyleModified, styleName);
        }

        /**
         * Call all subscribers for the given event with the specified argument
         * @param {!string} eventid
         * @param {Object} args
         */
        this.emit = function (eventid, args) {
            eventNotifier.emit(eventid, args);
        };

        /**
         * Subscribe to a given event with a callback
         * @param {!string} eventid
         * @param {!Function} cb
         */
        this.subscribe = function (eventid, cb) {
            eventNotifier.subscribe(eventid, cb);
        };

        /**
         * @param {!string} eventid
         * @param {!Function} cb
         * @return {undefined}
         */
        this.unsubscribe = function (eventid, cb) {
            eventNotifier.unsubscribe(eventid, cb);
        };

        this.getCursorPosition = function () {
            return odtDocument.getCursorPosition(localMemberId);
        };

        this.getCursorSelection = function () {
            return odtDocument.getCursorSelection(localMemberId);
        };

        this.getOdfCanvas = function () {
            return odtDocument.getOdfCanvas();
        };

        this.getCurrentParagraph = function () {
            return currentParagraphNode;
        };

        this.getAvailableParagraphStyles = function () {
            return formatting.getAvailableParagraphStyles();
        };

        this.getCurrentParagraphStyle = function () {
            return currentCommonStyleName;
        };

        /**
         * Applies the paragraph style with the given
         * style name to all the paragraphs within
         * the cursor selection.
         * @param {!string} styleName
         * @return {undefined}
         */
        this.setCurrentParagraphStyle = function (styleName) {
            var range = odtDocument.getCursor(localMemberId).getSelectedRange(),
                paragraphs = odfUtils.getParagraphElements(range),
                opQueue = [];

            paragraphs.forEach(function (paragraph) {
                var paragraphStartPoint = odtDocument.convertDomPointToCursorStep(paragraph, 0, NEXT),
                    paragraphStyleName = paragraph.getAttributeNS(odf.Namespaces.textns, "style-name"),
                    opSetParagraphStyle;

                if (paragraphStyleName !== styleName) {
                    opSetParagraphStyle = new ops.OpSetParagraphStyle();
                    opSetParagraphStyle.init({
                        memberid: localMemberId,
                        styleName: styleName,
                        position: paragraphStartPoint
                    });
                    opQueue.push(opSetParagraphStyle);
                }
            });

            if (opQueue.length > 0) {
                session.enqueue(opQueue);
            }
        };

        this.insertTable = function (initialRows, initialColumns, tableStyleName, tableColumnStyleName, tableCellStyleMatrix) {
            var op = new ops.OpInsertTable();
            op.init({
                memberid: localMemberId,
                position: self.getCursorPosition(),
                initialRows: initialRows,
                initialColumns: initialColumns,
                tableStyleName: tableStyleName,
                tableColumnStyleName: tableColumnStyleName,
                tableCellStyleMatrix: tableCellStyleMatrix
            });
            session.enqueue([op]);
        };

        /**
         * Takes a style name and returns the corresponding paragraph style
         * element. If the style name is an empty string, the default style
         * is returned.
         * @param {!string} styleName
         * @return {?Element}
         */
        function getParagraphStyleElement(styleName) {
            return (styleName === "")
                ? formatting.getDefaultStyleElement('paragraph')
                : formatting.getStyleElement(styleName, 'paragraph');
        }

        this.getParagraphStyleElement = getParagraphStyleElement;

        /**
         * Returns if the style is used anywhere in the document
         * @param {!Element} styleElement
         * @return {boolean}
         */
        this.isStyleUsed = function (styleElement) {
            return formatting.isStyleUsed(styleElement);
        };

        /**
         * Returns the attributes of a given paragraph style name
         * (with inheritance). If the name is an empty string,
         * the attributes of the default style are returned.
         * @param {!string} styleName
         * @return {?odf.Formatting.StyleData}
         */
        this.getParagraphStyleAttributes = function (styleName) {
            var styleNode = getParagraphStyleElement(styleName),
                includeSystemDefault = styleName === "";

            if (styleNode) {
                return formatting.getInheritedStyleAttributes(styleNode, includeSystemDefault);
            }

            return null;
        };

        /**
         * Creates and enqueues a paragraph-style cloning operation.
         * Returns the created id for the new style.
         * @param {!string} styleName  id of the style to update
         * @param {!{paragraphProperties,textProperties}} setProperties  properties which are set
         * @param {!{paragraphPropertyNames,textPropertyNames}=} removedProperties  properties which are removed
         * @return {undefined}
         */
        this.updateParagraphStyle = function (styleName, setProperties, removedProperties) {
            var op;
            op = new ops.OpUpdateParagraphStyle();
            op.init({
                memberid: localMemberId,
                styleName: styleName,
                setProperties: setProperties,
                removedProperties: (!removedProperties) ? {} : removedProperties
            });
            session.enqueue([op]);
        };

        /**
         * Creates and enqueues a paragraph-style cloning operation.
         * Returns the created id for the new style.
         * @param {!string} styleName id of the style to clone
         * @param {!string} newStyleDisplayName display name of the new style
         * @return {!string}
         */
        this.cloneParagraphStyle = function (styleName, newStyleDisplayName) {
            var newStyleName = uniqueParagraphStyleNCName(newStyleDisplayName),
                styleNode = getParagraphStyleElement(styleName),
                op, setProperties, attributes, i;

            setProperties = formatting.getStyleAttributes(styleNode);
            // copy any attributes directly on the style
            attributes = styleNode.attributes;
            for (i = 0; i < attributes.length; i += 1) {
                // skip...
                // * style:display-name -> not copied, set to new string below
                // * style:name         -> not copied, set from op by styleName property
                // * style:family       -> "paragraph" always, set by op
                if (!/^(style:display-name|style:name|style:family)/.test(attributes[i].name)) {
                    setProperties[attributes[i].name] = attributes[i].value;
                }
            }

            setProperties['style:display-name'] = newStyleDisplayName;

            op = new ops.OpAddStyle();
            op.init({
                memberid: localMemberId,
                styleName: newStyleName,
                styleFamily: 'paragraph',
                setProperties: setProperties
            });
            session.enqueue([op]);

            return newStyleName;
        };

        this.deleteStyle = function (styleName) {
            var op;
            op = new ops.OpRemoveStyle();
            op.init({
                memberid: localMemberId,
                styleName: styleName,
                styleFamily: 'paragraph'
            });
            session.enqueue([op]);
        };

        /**
         * Returns an array of the declared fonts in the ODF document,
         * with 'duplicates' like Arial1, Arial2, etc removed. The alphabetically
         * first font name for any given family is kept.
         * The elements of the array are objects containing the font's name and
         * the family.
         * @return {Array.<!Object>}
         */
        this.getDeclaredFonts = function () {
            var fontMap = formatting.getFontMap(),
                usedFamilies = [],
                array = [],
                sortedNames,
                key,
                value,
                i;

            // Sort all the keys in the font map alphabetically
            sortedNames = Object.keys(fontMap);
            sortedNames.sort();

            for (i = 0; i < sortedNames.length; i += 1) {
                key = sortedNames[i];
                value = fontMap[key];

                // Use the font declaration only if the family is not already used.
                // Therefore we are able to discard the alphabetic successors of the first
                // font name.
                if (usedFamilies.indexOf(value) === -1) {
                    array.push({
                        name: key,
                        family: value
                    });
                    if (value) {
                        usedFamilies.push(value);
                    }
                }
            }

            return array;
        };

        this.getSelectedHyperlinks = function () {
            var cursor = odtDocument.getCursor(localMemberId);
            // no own cursor yet/currently added?
            if (!cursor) {
                return [];
            }
            return odfUtils.getHyperlinkElements(cursor.getSelectedRange());
        };

        this.getSelectedRange = function () {
            var cursor = odtDocument.getCursor(localMemberId);
            return cursor && cursor.getSelectedRange();
        };

        function undoStackModified(e) {
            self.emit(EditorSession.signalUndoStackChanged, e);
        }

        this.undo = function () {
            self.sessionController.undo();
        };

        this.redo = function () {
            self.sessionController.redo();
        };

        /**
         * @param {!string} memberId
         * @return {?ops.Member}
         */
        this.getMember = function (memberId) {
            return odtDocument.getMember(memberId);
        };

        /**
         * @param {!function(!Object=)} callback passing an error object in case of error
         * @return {undefined}
         */
        function destroy(callback) {
            var head = document.getElementsByTagName('head')[0],
                eventManager = self.sessionController.getEventManager();

            head.removeChild(fontStyles);

            odtDocument.unsubscribe(ops.Document.signalMemberAdded, onMemberAdded);
            odtDocument.unsubscribe(ops.Document.signalMemberUpdated, onMemberUpdated);
            odtDocument.unsubscribe(ops.Document.signalMemberRemoved, onMemberRemoved);
            odtDocument.unsubscribe(ops.Document.signalCursorAdded, onCursorAdded);
            odtDocument.unsubscribe(ops.Document.signalCursorRemoved, onCursorRemoved);
            odtDocument.unsubscribe(ops.Document.signalCursorMoved, onCursorMoved);
            odtDocument.unsubscribe(ops.OdtDocument.signalCommonStyleCreated, onStyleCreated);
            odtDocument.unsubscribe(ops.OdtDocument.signalCommonStyleDeleted, onStyleDeleted);
            odtDocument.unsubscribe(ops.OdtDocument.signalParagraphStyleModified, onParagraphStyleModified);
            odtDocument.unsubscribe(ops.OdtDocument.signalParagraphChanged, trackCurrentParagraph);
            odtDocument.unsubscribe(ops.OdtDocument.signalUndoStackChanged, undoStackModified);

            eventManager.unsubscribe("mousemove", hyperlinkTooltipView.showTooltip);
            eventManager.unsubscribe("mouseout", hyperlinkTooltipView.hideTooltip);
            delete self.sessionView;
            delete self.sessionController;
            callback();
        }

        /**
         * @param {!function(!Error=)} callback passing an error object in case of error
         * @return {undefined}
         */
        this.destroy = function(callback) {
                var cleanup = [
                    self.sessionView.destroy,
                    caretManager.destroy,
                    selectionViewManager.destroy,
                    self.sessionController.destroy,
                    hyperlinkTooltipView.destroy,
                    odfFieldView.destroy,
                    destroy
                ];

            webodfcore.Async.destroyAll(cleanup, callback);
        };

        function init() {
            var head = document.getElementsByTagName('head')[0],
                odfCanvas = session.getOdtDocument().getOdfCanvas(),
                eventManager;

            // TODO: fonts.css should be rather done by odfCanvas, or?
            fontStyles.type = 'text/css';
            fontStyles.media = 'screen, print, handheld, projection';
            fontStyles.appendChild(document.createTextNode(fontsCSS));
            head.appendChild(fontStyles);

            odfFieldView = new gui.OdfFieldView(odfCanvas);
            odfFieldView.showFieldHighlight();
            self.sessionController = new gui.SessionController(session, localMemberId, shadowCursor, {
                annotationsEnabled: config.annotationsEnabled,
                directTextStylingEnabled: config.directTextStylingEnabled,
                directParagraphStylingEnabled: config.directParagraphStylingEnabled
            });
            sessionConstraints = self.sessionController.getSessionConstraints();

            eventManager = self.sessionController.getEventManager();
            hyperlinkTooltipView = new gui.HyperlinkTooltipView(odfCanvas,
                                                    self.sessionController.getHyperlinkClickHandler().getModifier);
            eventManager.subscribe("mousemove", hyperlinkTooltipView.showTooltip);
            eventManager.subscribe("mouseout", hyperlinkTooltipView.hideTooltip);

            caretManager = new gui.CaretManager(self.sessionController, odfCanvas.getViewport());
            selectionViewManager = new gui.SelectionViewManager(gui.SvgSelectionView);
            self.sessionView = new gui.SessionView(config.viewOptions, localMemberId, session, sessionConstraints, caretManager, selectionViewManager);
            self.availableFonts = getAvailableFonts();
            selectionViewManager.registerCursor(shadowCursor, true);

            // Session Constraints can be applied once the controllers are instantiated.
            if (config.reviewModeEnabled) {
                // Disallow deleting other authors' annotations.
                sessionConstraints.setState(gui.CommonConstraints.EDIT.ANNOTATIONS.ONLY_DELETE_OWN, true);
                sessionConstraints.setState(gui.CommonConstraints.EDIT.REVIEW_MODE, true);
            }

            // Custom signals, that make sense in the Editor context. We do not want to expose webodf's ops signals to random bits of the editor UI.
            odtDocument.subscribe(ops.Document.signalMemberAdded, onMemberAdded);
            odtDocument.subscribe(ops.Document.signalMemberUpdated, onMemberUpdated);
            odtDocument.subscribe(ops.Document.signalMemberRemoved, onMemberRemoved);
            odtDocument.subscribe(ops.Document.signalCursorAdded, onCursorAdded);
            odtDocument.subscribe(ops.Document.signalCursorRemoved, onCursorRemoved);
            odtDocument.subscribe(ops.Document.signalCursorMoved, onCursorMoved);
            odtDocument.subscribe(ops.OdtDocument.signalCommonStyleCreated, onStyleCreated);
            odtDocument.subscribe(ops.OdtDocument.signalCommonStyleDeleted, onStyleDeleted);
            odtDocument.subscribe(ops.OdtDocument.signalParagraphStyleModified, onParagraphStyleModified);
            odtDocument.subscribe(ops.OdtDocument.signalParagraphChanged, trackCurrentParagraph);
            odtDocument.subscribe(ops.OdtDocument.signalUndoStackChanged, undoStackModified);
        }

        init();
    };

    /**@const*/EditorSession.signalMemberAdded =            "memberAdded";
    /**@const*/EditorSession.signalMemberUpdated =          "memberUpdated";
    /**@const*/EditorSession.signalMemberRemoved =          "memberRemoved";
    /**@const*/EditorSession.signalCursorAdded =            "cursorAdded";
    /**@const*/EditorSession.signalCursorRemoved =          "cursorRemoved";
    /**@const*/EditorSession.signalCursorMoved =            "cursorMoved";
    /**@const*/EditorSession.signalParagraphChanged =       "paragraphChanged";
    /**@const*/EditorSession.signalCommonStyleCreated =     "styleCreated";
    /**@const*/EditorSession.signalCommonStyleDeleted =     "styleDeleted";
    /**@const*/EditorSession.signalParagraphStyleModified = "paragraphStyleModified";
    /**@const*/EditorSession.signalUndoStackChanged =       "signalUndoStackChanged";

    return EditorSession;
});
