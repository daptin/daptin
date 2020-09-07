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

/*global define, require, runtime, gui, ops */

define("webodf/editor/widgets/simpleStyles", [
    "webodf/editor/widgets/fontPicker",
    "dijit/form/ToggleButton",
    "dijit/form/NumberSpinner"],

    function (FontPicker, ToggleButton, NumberSpinner) {
        "use strict";

        var SimpleStyles = function (callback) {
            var self = this,
                editorSession,
                widget = {},
                directFormattingController,
                boldButton,
                italicButton,
                underlineButton,
                strikethroughButton,
                fontSizeSpinner,
                fontPicker,
                fontPickerWidget;

            boldButton = new ToggleButton({
                label: runtime.tr('Bold'),
                disabled: true,
                showLabel: false,
                checked: false,
                iconClass: "dijitEditorIcon dijitEditorIconBold",
                onChange: function (checked) {
                    directFormattingController.setBold(checked);
                    self.onToolDone();
                }
            });

            italicButton = new ToggleButton({
                label: runtime.tr('Italic'),
                disabled: true,
                showLabel: false,
                checked: false,
                iconClass: "dijitEditorIcon dijitEditorIconItalic",
                onChange: function (checked) {
                    directFormattingController.setItalic(checked);
                    self.onToolDone();
                }
            });

            underlineButton = new ToggleButton({
                label: runtime.tr('Underline'),
                disabled: true,
                showLabel: false,
                checked: false,
                iconClass: "dijitEditorIcon dijitEditorIconUnderline",
                onChange: function (checked) {
                    directFormattingController.setHasUnderline(checked);
                    self.onToolDone();
                }
            });

            strikethroughButton = new ToggleButton({
                label: runtime.tr('Strikethrough'),
                disabled: true,
                showLabel: false,
                checked: false,
                iconClass: "dijitEditorIcon dijitEditorIconStrikethrough",
                onChange: function (checked) {
                    directFormattingController.setHasStrikethrough(checked);
                    self.onToolDone();
                }
            });

            fontSizeSpinner = new NumberSpinner({
                label: runtime.tr('Size'),
                disabled: true,
                showLabel: false,
                value: 12,
                smallDelta: 1,
                constraints: {min: 6, max: 96},
                intermediateChanges: true,
                onChange: function (value) {
                    directFormattingController.setFontSize(value);
                },
                onClick: function () {
                    self.onToolDone();
                },
                onInput: function () {
                    // Do not process any input in the text box;
                    // even paste events will not be processed
                    // so that no corrupt values can exist
                    return false;
                }
            });

            /*jslint emptyblock: true*/
            fontPicker = new FontPicker(function () {});
            /*jslint emptyblock: false*/
            fontPickerWidget = fontPicker.widget();
            fontPickerWidget.setAttribute('disabled', true);
            fontPickerWidget.onChange = function (value) {
                directFormattingController.setFontName(value);
                self.onToolDone();
            };

            widget.children = [boldButton, italicButton, underlineButton, strikethroughButton, fontPickerWidget, fontSizeSpinner];
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

            function updateStyleButtons(changes) {
                // The 3rd parameter to set(...) is false to avoid firing onChange when setting the value programmatically.
                var updateCalls = {
                    isBold: function (value) { boldButton.set('checked', value, false); },
                    isItalic: function (value) { italicButton.set('checked', value, false); },
                    hasUnderline: function (value) { underlineButton.set('checked', value, false); },
                    hasStrikeThrough: function (value) { strikethroughButton.set('checked', value, false); },
                    fontSize: function (value) { 
                        fontSizeSpinner.set('intermediateChanges', false); // Necessary due to https://bugs.dojotoolkit.org/ticket/11588
                        fontSizeSpinner.set('value', value, false);
                        fontSizeSpinner.set('intermediateChanges', true);
                    },
                    fontName: function (value) { fontPickerWidget.set('value', value, false); }
                };

                Object.keys(changes).forEach(function (key) {
                    var updateCall = updateCalls[key];
                    if (updateCall) {
                        updateCall(changes[key]);
                    }
                });
            }

            function enableStyleButtons(enabledFeatures) {
                widget.children.forEach(function (element) {
                    element.setAttribute('disabled', !enabledFeatures.directTextStyling);
                });
            }

            this.setEditorSession = function (session) {
                if (editorSession) {
                    directFormattingController.unsubscribe(gui.DirectFormattingController.textStylingChanged, updateStyleButtons);
                    directFormattingController.unsubscribe(gui.DirectFormattingController.enabledChanged, enableStyleButtons);
                }

                editorSession = session;
                fontPicker.setEditorSession(editorSession);
                if (editorSession) {
                    directFormattingController = editorSession.sessionController.getDirectFormattingController();

                    directFormattingController.subscribe(gui.DirectFormattingController.textStylingChanged, updateStyleButtons);
                    directFormattingController.subscribe(gui.DirectFormattingController.enabledChanged, enableStyleButtons);

                    enableStyleButtons(directFormattingController.enabledFeatures());
                } else {
                    enableStyleButtons({ directTextStyling: false});
                }

                updateStyleButtons({
                    isBold: editorSession ? directFormattingController.isBold() : false,
                    isItalic: editorSession ? directFormattingController.isItalic() : false,
                    hasUnderline: editorSession ? directFormattingController.hasUnderline() : false,
                    hasStrikeThrough: editorSession ? directFormattingController.hasStrikeThrough() : false,
                    fontSize: editorSession ? directFormattingController.fontSize() : undefined,
                    fontName: editorSession ? directFormattingController.fontName() : undefined
                });
            };

            /*jslint emptyblock: true*/
            this.onToolDone = function () {};
            /*jslint emptyblock: false*/

            callback(widget);
        };

        return SimpleStyles;
});
