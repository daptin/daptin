/**
 * Copyright (C) 2012 KO GmbH <copyright@kogmbh.com>
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
/*global define,require,document */
define("webodf/editor/widgets/fontPicker", [
    "dijit/form/Select",
    "dojox/html/entities"],

    function (Select, htmlEntities) {
        "use strict";

        /**
         * @constructor
         */
        var FontPicker = function (callback) {
            var self = this,
                editorSession,
                select,
                documentFonts = [];

            select = new Select({
                name: 'FontPicker',
                disabled: true,
                maxHeight: 200,
                style: {
                    width: '150px'
                }
            });
            // prevent browser translation service messing up ids
            select.domNode.setAttribute("translate", "no");
            select.domNode.classList.add("notranslate");
            select.dropDown.domNode.setAttribute("translate", "no");
            select.dropDown.domNode.classList.add("notranslate");

            this.widget = function () {
                return select;
            };

            this.value = function () {
                return select.get('value');
            };

            this.setValue = function (value) {
                select.set('value', value);
            };

            /**
             * Returns the font family for a given font name. If unavailable,
             * return the name itself (e.g. editor fonts won't have a name-family separation
             * @param {!string} name
             * @return {!string}
             */
            this.getFamily = function (name) {
                var i;
                for (i = 0; i < documentFonts.length; i += 1) {
                    if ((documentFonts[i].name === name) && documentFonts[i].family) {
                        return documentFonts[i].family;
                    }
                }
                return name;
            };
            // events
            this.onAdd = null;
            this.onRemove = null;

            function populateFonts() {
                var i,
                    name,
                    family,
                    editorFonts = editorSession ? editorSession.availableFonts : [],
                    selectionList = [];

                documentFonts = editorSession ? editorSession.getDeclaredFonts() : [];

                // First populate the fonts used in the document
                for (i = 0; i < documentFonts.length; i += 1) {
                    name = documentFonts[i].name;
                    family = documentFonts[i].family || name;
                    selectionList.push({
                        label: '<span style="font-family: ' + htmlEntities.encode(family) + ';">' + htmlEntities.encode(name)+ '</span>',
                        value: name
                    });
                }
                if (editorFonts.length) {
                    // Then add a separator
                    selectionList.push({
                        type: 'separator'
                    });
                }
                // Lastly populate the fonts provided by the editor
                for (i = 0; i < editorFonts.length; i += 1) {
                    selectionList.push({
                        label: '<span style="font-family: ' + htmlEntities.encode(editorFonts[i]) + ';">' + htmlEntities.encode(editorFonts[i]) + '</span>',
                        value: editorFonts[i]
                    });
                }

                select.removeOption(select.getOptions());
                select.addOption(selectionList);
            }

            this.setEditorSession = function(session) {
                editorSession = session;
                populateFonts();
                select.setAttribute('disabled', !editorSession);
            };
            populateFonts();

            callback(self);
        };

        return FontPicker;
});
