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

/*global define, require, dojo, dijit, runtime */

define("webodf/editor/widgets/paragraphStylesDialog", [
    "webodf/editor/widgets/dialogWidgets/idMangler"],
function (IdMangler) {
    "use strict";
    return function ParagraphStylesDialog(callback) {
        var self = this,
            idMangler = new IdMangler(),
            editorSession,
            dialog,
            stylePicker, alignmentPane, fontEffectsPane;

        function makeWidget(callback) {
            require([
                "dijit/Dialog",
                "dijit/TooltipDialog",
                "dijit/popup",
                "dijit/layout/LayoutContainer",
                "dijit/layout/TabContainer",
                "dijit/layout/ContentPane",
                "dijit/form/Button",
                "dijit/form/DropDownButton"], function (Dialog, TooltipDialog, popup, LayoutContainer, TabContainer, ContentPane, Button, DropDownButton) {
                var tr = runtime.tr,
                    mainLayoutContainer,
                    tabContainer,
                    topBar,
                    actionBar,
                    cloneButton,
                    deleteButton,
                    cloneTooltip,
                    cloneDropDown,
                    /**
                    * Mapping of the properties from edit pane properties to the attributes of style:text-properties
                    * @const@type{Array.<!{propertyName:string,attributeName:string,unit:string}>}
                    */
                    textPropertyMapping = [{
                        propertyName:  'fontSize',
                        attributeName: 'fo:font-size',
                        unit:          'pt'
                    }, {
                        propertyName:  'fontName',
                        attributeName: 'style:font-name'
                    }, {
                        propertyName:  'color',
                        attributeName: 'fo:color'
                    }, {
                        propertyName:  'backgroundColor',
                        attributeName: 'fo:background-color'
                    }, {
                        propertyName:  'fontWeight',
                        attributeName: 'fo:font-weight'
                    }, {
                        propertyName:  'fontStyle',
                        attributeName: 'fo:font-style'
                    }, {
                        propertyName:  'underline',
                        attributeName: 'style:text-underline-style'
                    }, {
                        propertyName:  'strikethrough',
                        attributeName: 'style:text-line-through-style'
                    }],
                    /**
                    * Mapping of the properties from edit pane properties to the attributes of style:paragraph-properties
                    * @const@type{Array.<!{propertyName:string,attributeName:string,unit:string}>}
                    */
                    paragraphPropertyMapping = [{
                        propertyName:  'topMargin',
                        attributeName: 'fo:margin-top',
                        unit:          'mm'
                    }, {
                        propertyName:  'bottomMargin',
                        attributeName: 'fo:margin-bottom',
                        unit:          'mm'
                    }, {
                        propertyName:  'leftMargin',
                        attributeName: 'fo:margin-left',
                        unit:          'mm'
                    }, {
                        propertyName:  'rightMargin',
                        attributeName: 'fo:margin-right',
                        unit:          'mm'
                    }, {
                        propertyName:  'textAlign',
                        attributeName: 'fo:text-align'
                    }],
                    originalFontEffectsPaneValue,
                    originalAlignmentPaneValue;

                /**
                * Sets attributes of a node by the properties of the object properties,
                * based on the mapping defined in propertyMapping.
                * @param {!Object} properties
                * @param {!Array.<!{propertyName:string,attributeName:string,unit:string}>} propertyMapping
                * @return {!Object}
                */
                function mappedProperties(properties, propertyMapping) {
                    var i, m, value,
                        result = {};
                    for (i = 0; i < propertyMapping.length; i += 1) {
                        m = propertyMapping[i];
                        value = properties[m.propertyName];
                        // Set a value as the attribute of a node, if that value is defined.
                        // If there is a unit specified, it is suffixed to the value.
                        if (value !== undefined) {
                            result[m.attributeName] = (m.unit !== undefined) ? value + m.unit : value;
                        }
                    }
                    return result;
                }

                /**
                 * Returns an flat object containing only the key-value mappings
                 * from the 'new' flat object which are different from the 'old' object's.
                 * @param {!Object} oldProperties
                 * @param {!Object} newProperties
                 * @return {!Object}
                 */
                function updatedProperties(oldProperties, newProperties) {
                    var properties = {};
                    Object.keys(newProperties).forEach(function (key) {
                        if (newProperties[key] !== oldProperties[key]) {
                            properties[key] = newProperties[key];
                        }
                    });
                    return properties;
                }

                function accept() {
                    editorSession.updateParagraphStyle(stylePicker.value(), {
                        "style:paragraph-properties": mappedProperties(
                                                        updatedProperties(originalAlignmentPaneValue, alignmentPane.value()),
                                                        paragraphPropertyMapping
                                                     ),
                        "style:text-properties": mappedProperties(
                                                    updatedProperties(originalFontEffectsPaneValue, fontEffectsPane.value()),
                                                    textPropertyMapping
                                                 )
                    });

                    dialog.hide();
                }

                function cancel() {
                    dialog.hide();
                }

                function setStyle(value) {
                    if (value !== stylePicker.value()) {
                        stylePicker.setValue(value);
                    }

                    alignmentPane.setStyle(value);
                    fontEffectsPane.setStyle(value);
                    originalAlignmentPaneValue = alignmentPane.value();
                    originalFontEffectsPaneValue = fontEffectsPane.value();

                    // If it is a default (nameless) style or is used, make it undeletable.
                    if (value === "" || editorSession.isStyleUsed(editorSession.getParagraphStyleElement(value))) {
                        deleteButton.domNode.style.display = 'none';
                    } else {
                        deleteButton.domNode.style.display = 'block';
                    }
                }

                /**
                * Creates and enqueues a paragraph-style cloning operation.
                * Remembers the id of the created style in newStyleName, so the
                * style picker can be set to it, once the operation has been applied.
                * @param {!string} styleName id of the style to clone
                * @param {!string} newStyleDisplayName display name of the new style
                */
                function cloneStyle(styleName, newStyleDisplayName) {
                    var newStyleName = editorSession.cloneParagraphStyle(styleName, newStyleDisplayName);
                    setStyle(newStyleName);
                }

                function deleteStyle(styleName) {
                    editorSession.deleteStyle(styleName);
                }
                // Dialog
                dialog = new Dialog({
                    title: tr("Paragraph Styles")
                });

                mainLayoutContainer = new LayoutContainer({
                    style: "height: 520px; width: 450px;"
                });

                topBar = new ContentPane({
                    region: "top",
                    style: "margin: 0; padding: 0"
                });
                mainLayoutContainer.addChild(topBar);

                cloneTooltip = new TooltipDialog({
                    content: idMangler.mangleIds(
                        '<h2 style="margin: 0;">' + tr("Clone this Style") + '</h2><br/>' +
                        '<label for="name">' + tr("New Name:") + '</label> <input data-dojo-type="dijit/form/TextBox" id="name" name="name"><br/><br/>'),
                    style: "width: 300px;"
                });
                cloneButton = new Button({
                    label: tr("Create"),
                    onClick: function () {
                        cloneStyle(stylePicker.value(), cloneTooltip.get('value').name);
                        cloneTooltip.reset();
                        popup.close(cloneTooltip);
                    }
                });
                cloneTooltip.addChild(cloneButton);
                cloneDropDown = new DropDownButton({
                    label: tr("Clone"),
                    showLabel: false,
                    iconClass: 'dijitEditorIcon dijitEditorIconCopy',
                    dropDown: cloneTooltip,
                    style: "float: right; margin-bottom: 5px;"
                });
                topBar.addChild(cloneDropDown, 1);

                deleteButton = new Button({
                    label: tr("Delete"),
                    showLabel: false,
                    iconClass: 'dijitEditorIcon dijitEditorIconDelete',
                    style: "float: right; margin-bottom: 5px;",
                    onClick: function () {
                        deleteStyle(stylePicker.value());
                    }
                });
                topBar.addChild(deleteButton, 2);

                // Tab Container
                tabContainer = new TabContainer({
                    region: "center"
                });
                mainLayoutContainer.addChild(tabContainer);

                actionBar = dojo.create("div", {
                    "class": "dijitDialogPaneActionBar"
                });
                new dijit.form.Button({
                    label: tr("OK"),
                    onClick: accept
                }).placeAt(actionBar);
                new dijit.form.Button({
                    label: tr("Cancel"),
                    onClick: cancel
                }).placeAt(actionBar);
                dialog.domNode.appendChild(actionBar);


                require([
                    "webodf/editor/widgets/paragraphStyles",
                    "webodf/editor/widgets/dialogWidgets/alignmentPane",
                    "webodf/editor/widgets/dialogWidgets/fontEffectsPane"
                ], function (ParagraphStyles, AlignmentPane, FontEffectsPane) {
                    var p, a, f;

                    p = new ParagraphStyles(function (paragraphStyles) {
                        stylePicker = paragraphStyles;
                        stylePicker.widget().startup();
                        stylePicker.widget().domNode.style.float = "left";
                        stylePicker.widget().domNode.style.width = "350px";
                        stylePicker.widget().domNode.style.marginTop = "5px";
                        topBar.addChild(stylePicker.widget(), 0);

                        stylePicker.onRemove = function () {
                            // The style picker automatically falls back
                            // to the first entry if the currently selected
                            // entry is deleted. So it is safe to simply
                            // open the new auto-selected entry after removal.
                            setStyle(stylePicker.value());
                        };

                        stylePicker.onChange = setStyle;
                        stylePicker.setEditorSession(editorSession);
                    });
                    a = new AlignmentPane(function (pane) {
                        alignmentPane = pane;
                        alignmentPane.widget().startup();
                        tabContainer.addChild(alignmentPane.widget());
                        alignmentPane.setEditorSession(editorSession);
                    });
                    f = new FontEffectsPane(function (pane) {
                        fontEffectsPane = pane;
                        fontEffectsPane.widget().startup();
                        tabContainer.addChild(fontEffectsPane.widget());
                        fontEffectsPane.setEditorSession(editorSession);
                    });

                    dialog.onShow = function () {
                        var currentStyle = editorSession.getCurrentParagraphStyle();
                        setStyle(currentStyle);
                    };

                    dialog.onHide = self.onToolDone;

                    // only done to make jslint see the var used
                    return p || a || f;
                });

                dialog.addChild(mainLayoutContainer);
                mainLayoutContainer.startup();

                return callback(dialog);
            });
        }

        this.setEditorSession = function (session) {
            editorSession = session;
            if (stylePicker) {
                stylePicker.setEditorSession(session);
            }
            if (alignmentPane) {
                alignmentPane.setEditorSession(session);
            }
            if (fontEffectsPane) {
                fontEffectsPane.setEditorSession(session);
            }
            if (!editorSession && dialog) { // TODO: check show state
                dialog.hide();
            }
        };

        /*jslint emptyblock: true*/
        this.onToolDone = function () {};
        /*jslint emptyblock: false*/

        // init
        makeWidget(function (dialog) {
            return callback(dialog);
        });
    };

});
