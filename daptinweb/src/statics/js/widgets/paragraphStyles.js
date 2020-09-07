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

/*global define, require, runtime, ops */

define("webodf/editor/widgets/paragraphStyles", [
    "dijit/form/Select",
    "dojox/html/entities",
    "webodf/editor/EditorSession"],

    function (Select, htmlEntities, EditorSession) {
    "use strict";

    /**
     * @constructor
     */
    var ParagraphStyles = function (callback) {
        var self = this,
            editorSession,
            select,
            defaultStyleUIId = ":default";

        this.widget = function () {
            return select;
        };

        /*
         * In this widget, we name the default style
         * (which is referred to as "" in webodf) as
         * ":default". The ":" is disallowed in an NCName, so this
         * avoids clashes with other styles.
         */

        this.value = function () {
            var value = select.get('value');
            if (value === defaultStyleUIId) {
                value = "";
            }
            return value;
        };

        this.setValue = function (value) {
            if (value === "") {
                value = defaultStyleUIId;
            }
            select.set('value', value, false);
        };

        // events
        this.onAdd = null;
        this.onRemove = null;
        /*jslint emptyblock: true*/
        this.onChange = function () {};
        /*jslint emptyblock: false*/

        function populateStyles() {
            var i, selectionList, availableStyles;

            // Populate the Default Style always 
            selectionList = [{
                label: runtime.tr("Default Style"),
                value: defaultStyleUIId
            }];
            availableStyles = editorSession ? editorSession.getAvailableParagraphStyles() : [];

            for (i = 0; i < availableStyles.length; i += 1) {
                selectionList.push({
                    label: htmlEntities.encode(availableStyles[i].displayName) || htmlEntities.encode(availableStyles[i].name),
                    value: availableStyles[i].name
                });
            }

            select.removeOption(select.getOptions());
            select.addOption(selectionList);
        }

        function addStyle(styleInfo) {
            var stylens = "urn:oasis:names:tc:opendocument:xmlns:style:1.0",
                newStyleElement;

            if (styleInfo.family !== 'paragraph') {
                return;
            }

            newStyleElement = editorSession.getParagraphStyleElement(styleInfo.name);
            select.addOption({
                label: htmlEntities.encode(newStyleElement.getAttributeNS(stylens, 'display-name')),
                value: styleInfo.name
            });

            if (self.onAdd) {
                self.onAdd(styleInfo.name);
            }
        }

        function removeStyle(styleInfo) {
            if (styleInfo.family !== 'paragraph') {
                return;
            }

            select.removeOption(styleInfo.name);

            if (self.onRemove) {
                self.onRemove(styleInfo.name);
            }
        }

        function handleCursorMoved(cursor) {
            var disabled = cursor.getSelectionType() === ops.OdtCursor.RegionSelection;
            select.setAttribute('disabled', disabled);
        }

        this.setEditorSession = function(session) {
            if (editorSession) {
                editorSession.unsubscribe(EditorSession.signalCommonStyleCreated, addStyle);
                editorSession.unsubscribe(EditorSession.signalCommonStyleDeleted, removeStyle);
                editorSession.unsubscribe(EditorSession.signalCursorMoved, handleCursorMoved);
            }

            editorSession = session;
            if (editorSession) {
                editorSession.subscribe(EditorSession.signalCommonStyleCreated, addStyle);
                editorSession.subscribe(EditorSession.signalCommonStyleDeleted, removeStyle);
                editorSession.subscribe(EditorSession.signalCursorMoved, handleCursorMoved);
            }
            select.setAttribute('disabled', !editorSession);

            populateStyles();
        };

        // init
        function init() {
            select = new Select({
                name: 'ParagraphStyles',
                maxHeight: 200,
                style: {
                    width: '100px'
                }
            });
            // prevent browser translation service messing up ids
            select.domNode.setAttribute("translate", "no");
            select.domNode.classList.add("notranslate");
            select.dropDown.domNode.setAttribute("translate", "no");
            select.dropDown.domNode.classList.add("notranslate");

            populateStyles();

            // Call ParagraphStyles's onChange handler every time
            // the select's onchange is called, and pass the value
            // as reported by ParagraphStyles.value(), because we do not
            // want to expose the internal naming like ":default" outside this
            // class.
            select.onChange = function () {
                self.onChange(self.value());
            };

            return callback(self);
        }

        init();
    };

    return ParagraphStyles;
});
