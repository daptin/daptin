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

/*global runtime,define,require,document,dijit */

define("webodf/editor/widgets/dialogWidgets/fontEffectsPane", [
    "webodf/editor/widgets/dialogWidgets/idMangler"],
function (IdMangler) {
    "use strict";

    var FontEffectsPane = function (callback) {
        var self = this,
            idMangler = new IdMangler(),
            editorSession,
            contentPane,
            form,
            preview,
            textColorPicker,
            backgroundColorPicker,
            fontPicker;

        this.widget = function () {
            return contentPane;
        };

        this.value = function () {
            var textProperties = form.get('value'),
                textStyle = textProperties.textStyle;

            textProperties.fontWeight = (textStyle.indexOf('bold') !== -1)
                                            ? 'bold'
                                            : 'normal';
            textProperties.fontStyle = (textStyle.indexOf('italic') !== -1)
                                            ? 'italic'
                                            : 'normal';
            textProperties.underline = (textStyle.indexOf('underline') !== -1)
                                            ? 'solid'
                                            : 'none';

            delete textProperties.textStyle;
            return textProperties;
        };

        this.setStyle = function (styleName) {
            var style = editorSession.getParagraphStyleAttributes(styleName)['style:text-properties'],
                s_bold,
                s_italic,
                s_underline,
                s_fontSize,
                s_fontName,
                s_color,
                s_backgroundColor;

            if (style !== undefined) {
                s_bold = style['fo:font-weight'];
                s_italic = style['fo:font-style'];
                s_underline = style['style:text-underline-style'];
                s_fontSize = parseFloat(style['fo:font-size']);
                s_fontName = style['style:font-name'];
                s_color = style['fo:color'];
                s_backgroundColor = style['fo:background-color'];

                form.attr('value', {
                    fontName: s_fontName && s_fontName.length ? s_fontName : 'Arial',
                    fontSize: isNaN(s_fontSize) ? 12 : s_fontSize,
                    textStyle: [
                        s_bold,
                        s_italic,
                        s_underline === 'solid' ? 'underline' : undefined
                    ]
                });
                textColorPicker.set('value', s_color && s_color.length ? s_color : '#000000');
                backgroundColorPicker.set('value', s_backgroundColor && s_backgroundColor.length ? s_backgroundColor : '#ffffff');

            } else {
                // TODO: Use default style here
                form.attr('value', {
                    fontFamily: 'sans-serif',
                    fontSize: 12,
                    textStyle: []
                });
                textColorPicker.set('value', '#000000');
                backgroundColorPicker.set('value', '#ffffff');
            }

        };

        /*jslint unparam: true*/
        function init(cb) {
            require([
                "dojo",
                "dojo/ready",
                "dijit/layout/ContentPane",
                "dojox/widget/ColorPicker", // referenced in fontEffectsPane.html
                "webodf/editor/widgets/fontPicker"
            ], function (dojo, ready, ContentPane, ColorPicker, FontPicker) {
                var editorBase = dojo.config && dojo.config.paths &&
                            dojo.config.paths['webodf/editor'];
                runtime.assert(editorBase, "webodf/editor path not defined in dojoConfig");
                ready(function () {
                    contentPane = new ContentPane({
                        title: runtime.tr("Font Effects"),
                        href: editorBase + "/widgets/dialogWidgets/fontEffectsPane.html",
                        preload: true,
                        ioMethod: idMangler.ioMethod
                    });

                    contentPane.onLoad = function () {
                        var textColorTB = idMangler.byId('textColorTB'),
                            backgroundColorTB = idMangler.byId('backgroundColorTB');

                        form = idMangler.byId('fontEffectsPaneForm');
                        runtime.translateContent(form.domNode);

                        preview = idMangler.getElementById('previewText');
                        textColorPicker = idMangler.byId('textColorPicker');
                        backgroundColorPicker = idMangler.byId('backgroundColorPicker');

                        // Bind dojox widgets' values to invisible form elements, for easy parsing
                        textColorPicker.onChange = function (value) {
                            textColorTB.set('value', value);
                        };
                        backgroundColorPicker.onChange = function (value) {
                            backgroundColorTB.set('value', value);
                        };

                        fontPicker = new FontPicker(function (picker) {
                            picker.widget().startup();
                            idMangler.getElementById('fontPicker').appendChild(picker.widget().domNode);
                            picker.widget().name = 'fontName';
                            picker.setEditorSession(editorSession);
                        });

                        // Automatically update preview when selections change
                        form.watch('value', function () {
                            if (form.value.textStyle.indexOf('bold') !== -1) {
                                preview.style.fontWeight = 'bold';
                            } else {
                                preview.style.fontWeight = 'normal';
                            }
                            if (form.value.textStyle.indexOf('italic') !== -1) {
                                preview.style.fontStyle = 'italic';
                            } else {
                                preview.style.fontStyle = 'normal';
                            }
                            if (form.value.textStyle.indexOf('underline') !== -1) {
                                preview.style.textDecoration = 'underline';
                            } else {
                                preview.style.textDecoration = 'none';
                            }

                            preview.style.fontSize = form.value.fontSize + 'pt';
                            preview.style.fontFamily = fontPicker.getFamily(form.value.fontName);
                            preview.style.color = form.value.color;
                            preview.style.backgroundColor = form.value.backgroundColor;
                        });
                    };

                    return cb();
                });
            });
        }
        /*jslint unparam: false*/

        this.setEditorSession = function(session) {
            editorSession = session;
            if (fontPicker) {
                fontPicker.setEditorSession(editorSession);
            }
        };

        init(function () {
            return callback(self);
        });
    };

    return FontEffectsPane;
});
