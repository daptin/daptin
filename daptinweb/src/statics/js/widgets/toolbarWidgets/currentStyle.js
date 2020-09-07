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

/*global define, require */

define("webodf/editor/widgets/toolbarWidgets/currentStyle",
       ["webodf/editor/EditorSession"],

    function (EditorSession) {
        "use strict";

        return function CurrentStyle(callback) {
            var self = this,
                editorSession,
                paragraphStyles;

            function selectParagraphStyle(info) {
                if (paragraphStyles) {
                    if (info.type === 'style') {
                        paragraphStyles.setValue(info.styleName);
                    }
                }
            }

            function setParagraphStyle() {
                if (editorSession) {
                    editorSession.setCurrentParagraphStyle(paragraphStyles.value());
                }
                self.onToolDone();
            }

            function makeWidget(callback) {
                require(["webodf/editor/widgets/paragraphStyles"], function (ParagraphStyles) {
                    var p = new ParagraphStyles(function (pStyles) {
                        paragraphStyles = pStyles;

                        paragraphStyles.widget().onChange = setParagraphStyle;

                        paragraphStyles.setEditorSession(editorSession);
                        return callback(paragraphStyles.widget());
                    });
                    return p; // make sure p is not unused
                });
            }

            this.setEditorSession = function (session) {
                if (editorSession) {
                    editorSession.unsubscribe(EditorSession.signalParagraphChanged, selectParagraphStyle);
                }
                editorSession = session;
                if (paragraphStyles) {
                    paragraphStyles.setEditorSession(editorSession);
                }
                if (editorSession) {
                    editorSession.subscribe(EditorSession.signalParagraphChanged, selectParagraphStyle);
                    // TODO: selectParagraphStyle(editorSession.getCurrentParagraphStyle());
                }
            };

            /*jslint emptyblock: true*/
            this.onToolDone = function () {};
            /*jslint emptyblock: false*/

            makeWidget(function (widget) {
                return callback(widget);
            });
        };
    });
