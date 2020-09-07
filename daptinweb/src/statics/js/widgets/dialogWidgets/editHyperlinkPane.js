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

/*global runtime,core,define,require,document,dijit */

define("webodf/editor/widgets/dialogWidgets/editHyperlinkPane", [
    "dojo",
    "dijit/layout/ContentPane",
    "webodf/editor/widgets/dialogWidgets/idMangler"],

    function (dojo, ContentPane, IdMangler) {
        "use strict";

        runtime.loadClass("core.CSSUnits");

        var EditHyperlinkPane = function () {
            var self = this,
                editorBase = dojo.config && dojo.config.paths && dojo.config.paths['webodf/editor'],
                idMangler = new IdMangler(),
                contentPane,
                form,
                displayTextField,
                linkField,
                initialValue;

            runtime.assert(editorBase, "webodf/editor path not defined in dojoConfig");

            function onSave() {
                if (self.onSave) {
                    self.onSave();
                }
                return false;
            }

            function onCancel() {
                form.set('value', initialValue);
                if (self.onCancel) {
                    self.onCancel();
                }
            }

            contentPane = new ContentPane({
                title: runtime.tr("editLink"),
                href: editorBase+"/widgets/dialogWidgets/editHyperlinkPane.html",
                preload: true,
                ioMethod: idMangler.ioMethod,
                onLoad : function () {
                    form = idMangler.byId('editHyperlinkPaneForm');
                    form.onSubmit = onSave;
                    idMangler.byId('cancelHyperlinkChangeButton').onClick = onCancel;
                    displayTextField = idMangler.byId('linkDisplayText');
                    linkField = idMangler.byId('linkUrl');
                    linkField.on("change", function(value) {
                        displayTextField.set('placeHolder', value);
                    });

                    runtime.translateContent(form.domNode);
                    if (initialValue) {
                        form.set('value', initialValue);
                        displayTextField.set('disabled', initialValue.isReadOnlyText);
                        initialValue = undefined;
                    }
                    displayTextField.set('placeHolder', linkField.value);
                }
            });

            this.widget = function () {
                return contentPane;
            };

            this.value = function () {
                return form && form.get('value');
            };

            this.set = function (value) {
                initialValue = value;
                if (form) {
                    form.set('value', value);
                    displayTextField.set('disabled', value.isReadOnlyText);
                }
            };
        };

        return EditHyperlinkPane;
});
