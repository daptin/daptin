/**
 * Copyright (C) 2012-2013 KO GmbH <copyright@kogmbh.com>
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

/*global define, require, runtime*/

define("webodf/editor/widgets/undoRedoMenu",
    ["webodf/editor/EditorSession", "dijit/form/Button"],

    function (EditorSession, Button) {
        "use strict";

        return function UndoRedoMenu(callback) {
            var self = this,
                editorSession,
                undoButton,
                redoButton,
                widget = {};

            undoButton = new Button({
                label: runtime.tr('Undo'),
                showLabel: false,
                disabled: true, // TODO: get current session state
                iconClass: "dijitEditorIcon dijitEditorIconUndo",
                onClick: function () {
                    if (editorSession) {
                        editorSession.undo();
                        self.onToolDone();
                    }
                }
            });

            redoButton = new Button({
                label: runtime.tr('Redo'),
                showLabel: false,
                disabled: true, // TODO: get current session state
                iconClass: "dijitEditorIcon dijitEditorIconRedo",
                onClick: function () {
                    if (editorSession) {
                        editorSession.redo();
                        self.onToolDone();
                    }
                }
            });

            widget.children = [undoButton, redoButton];
            widget.startup = function () {
                widget.children.forEach(function (element) {
                    element.startup();
                });
            };

            widget.placeAt = function (container) {
                widget.children.forEach(function (element) {
                    element.placeAt(container);
                });
                return widget;
            };

            function checkUndoButtons(e) {
                if (undoButton) {
                    undoButton.set('disabled', e.undoAvailable === false);
                }
                if (redoButton) {
                    redoButton.set('disabled', e.redoAvailable === false);
                }
            }

            this.setEditorSession = function (session) {
                if (editorSession) {
                    editorSession.unsubscribe(EditorSession.signalUndoStackChanged, checkUndoButtons);
                }

                editorSession = session;
                if (editorSession) {
                    editorSession.subscribe(EditorSession.signalUndoStackChanged, checkUndoButtons);
                    // TODO: checkUndoButtons(editorSession.getundoredoavailablalalo());
                } else {
                    widget.children.forEach(function (element) {
                        element.setAttribute('disabled', true);
                    });
                }
            };

            /*jslint emptyblock: true*/
            this.onToolDone = function () {};
            /*jslint emptyblock: false*/

            // init
            callback(widget);
        };
    });
