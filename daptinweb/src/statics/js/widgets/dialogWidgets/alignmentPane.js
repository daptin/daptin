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

/*global runtime,core,define,require,dijit */

define("webodf/editor/widgets/dialogWidgets/alignmentPane", [
    "webodf/editor/widgets/dialogWidgets/idMangler"],
function (IdMangler) {
    "use strict";

    runtime.loadClass("core.CSSUnits");

    var AlignmentPane = function (callback) {
        var self = this,
            idMangler = new IdMangler(),
            editorSession,
            contentPane,
            form;

        this.widget = function () {
            return contentPane;
        };

        this.value = function () {
            return form.get('value');
        };

        this.setStyle = function (styleName) {
            var style = editorSession.getParagraphStyleAttributes(styleName)['style:paragraph-properties'],
                cssUnits = new webodfcore.CSSUnits(),
                s_topMargin,
                s_bottomMargin,
                s_leftMargin,
                s_rightMargin,
                s_textAlign;

            if (style !== undefined) {
                s_topMargin = cssUnits.convertMeasure(style['fo:margin-top'], 'mm');
                s_leftMargin = cssUnits.convertMeasure(style['fo:margin-left'], 'mm');
                s_rightMargin = cssUnits.convertMeasure(style['fo:margin-right'], 'mm');
                s_bottomMargin = cssUnits.convertMeasure(style['fo:margin-bottom'], 'mm');
                s_textAlign = style['fo:text-align'];

                form.attr('value', {
                    topMargin: isNaN(s_topMargin) ? 0 : s_topMargin,
                    bottomMargin: isNaN(s_bottomMargin) ? 0 : s_bottomMargin,
                    leftMargin: isNaN(s_leftMargin) ? 0 : s_leftMargin,
                    rightMargin: isNaN(s_rightMargin) ? 0 : s_rightMargin,
                    textAlign: s_textAlign && s_textAlign.length ? s_textAlign : 'left'
                });
            } else {
                form.attr('value', {
                    topMargin: 0,
                    bottomMargin: 0,
                    leftMargin: 0,
                    rightMargin: 0,
                    textAlign: 'left'
                });
            }
        };

        this.setEditorSession = function (session) {
            editorSession = session;
        };

        function init(cb) {
            require([
                "dojo",
                "dojo/ready",
                "dijit/layout/ContentPane"],
                function (dojo, ready, ContentPane) {
                    var editorBase = dojo.config && dojo.config.paths &&
                            dojo.config.paths['webodf/editor'];
                    runtime.assert(editorBase, "webodf/editor path not defined in dojoConfig");
                    ready(function () {
                        contentPane = new ContentPane({
                            title: runtime.tr("Alignment"),
                            href: editorBase+"/widgets/dialogWidgets/alignmentPane.html",
                            preload: true,
                            ioMethod: idMangler.ioMethod
                        });
                        contentPane.onLoad = function () {
                            form = idMangler.byId('alignmentPaneForm');
                            runtime.translateContent(form.domNode);
                        };
                        return cb();
                    });
            });
        }

        init(function () {
            return callback(self);
        });
    };

    return AlignmentPane;
});
