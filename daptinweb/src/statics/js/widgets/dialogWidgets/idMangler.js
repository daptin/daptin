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

/*global define, document, dojo, dijit */

define("webodf/editor/widgets/dialogWidgets/idMangler", ["dojo", "dijit"], function (dojo, dijit) {
    "use strict";
    var instanceCount = 0;

    /**
     * Mangles html id attributes and associated references so that the same HTML code can be copied into
     * a page multiple times without identifier clashes. This also provides helper functions for retrieving
     * these now-mangled identifiers.
     * @constructor
     */
    function IdMangler() {
        var suffix;

        /**
         * Returns the supplied text with identifiers mangled
         * @param {!string} text
         * @return {!string}
         */
        function mangleIds(text) {
            /*jslint regexp: true*/
            var newText = text.replace(/((id|for|data-dojo-id)\s*=\s*["'][^"']+)/g, "$1" + suffix);
            /*jslint regexp: false*/
            return newText;
        }
        this.mangleIds = mangleIds;

        /**
         * Replacement method for ContentPane's ioMethod
         * @return {*} See http://dojotoolkit.org/api/?qs=1.9/dojo/_base/xhr#1_9dojo__base_xhr_get for return details
         */
        this.ioMethod = function() {
            var args = Array.prototype.slice.call(arguments, 0);
            return dojo.xhr.get.apply(dojo, args).then(function(html) {
                return mangleIds(html);
            });
        };

        /**
         * Replacement for dijit.byId
         * @param {!string} id
         * @return {*} See http://dojotoolkit.org/api/?qs=1.9/dojo/_base/xhr#1_9dijit_registry_byId for return details
         */
        this.byId = function(id) {
            return dijit.byId(id + suffix);
        };

        /**
         * Replacement for document.getElementById
         * @param {!string} id
         * @return {HTMLElement|*}
         */
        this.getElementById = function(id) {
            return document.getElementById(id + suffix);
        };

        function init() {
            suffix = "_" + instanceCount;
            instanceCount += 1;
        }
        init();
    }

    return IdMangler;
});
