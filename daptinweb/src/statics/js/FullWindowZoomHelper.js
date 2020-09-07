/**
 * Copyright (C) 2014 KO GmbH <copyright@kogmbh.com>
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

/*global define, document, window */

define("webodf/editor/FullWindowZoomHelper", [], function () {
    "use strict";

    // fullscreen pinch-zoom adaption
    var FullWindowZoomHelper = function FullWindowZoomHelper(toolbarContainerElement, canvasContainerElement) {

        function translateToolbar() {
            var y = document.body.scrollTop;

            toolbarContainerElement.style.WebkitTransformOrigin = "center top";
            toolbarContainerElement.style.WebkitTransform = 'translateY(' + y + 'px)';
        }

        function repositionContainer() {
            canvasContainerElement.style.top = toolbarContainerElement.getBoundingClientRect().height + 'px';
        }

        this.destroy = function (callback) {
            window.removeEventListener('scroll', translateToolbar);
            window.removeEventListener('focusout', translateToolbar);
            window.removeEventListener('touchmove', translateToolbar);
            window.removeEventListener('resize', repositionContainer);

            callback();
        };

        function init() {
            var metaElement, toolbarStyle;

            // prevent any zooming on the window TODO: do not overwrite any other existing content of viewport metadata
            metaElement = document.createElement("meta");
            metaElement.setAttribute("name", "viewport");
            metaElement.setAttribute("content", "width=device-width, initial-scale=1.0, user-scalable=no, minimum-scale=1.0, maximum-scale=1.0");
            document.head.appendChild(metaElement);

            // set the toolbar absolute and fixed to top
            toolbarStyle = toolbarContainerElement.style;
            toolbarStyle.top = 0;
            toolbarStyle.left = 0;
            toolbarStyle.right = 0;
            toolbarStyle.position = "absolute";
            toolbarStyle.zIndex = 5;
            toolbarStyle.boxShadow = "0 1px 5px rgba(0, 0, 0, 0.25)";

            repositionContainer();

            window.addEventListener('scroll', translateToolbar);
            window.addEventListener('focusout', translateToolbar);
            window.addEventListener('touchmove', translateToolbar);
            window.addEventListener('resize', repositionContainer);
        }

        init();
    };

    return FullWindowZoomHelper;
});
