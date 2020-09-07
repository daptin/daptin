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

/*global define, runtime, XMLHttpRequest */

define("webodf/editor/Translator", [], function () {
    "use strict";

    return function Translator(translationsPath, locale, callback) {
        var self = this,
            dictionary = {};

        function translate(key) {
            return dictionary[key];
        }
        function setLocale(newLocale, cb) {
            // TODO: Add smarter locale resolution at some point
            if (newLocale.split('-')[0] === "de" || newLocale.split('_')[0] === "de") {
                newLocale = "de-DE";
            } else if (newLocale.split('-')[0] === "nl" || newLocale.split('_')[0] === "nl") {
                newLocale = "nl-NL";
            } else if (newLocale.split('-')[0] === "fr" || newLocale.split('_')[0] === "fr") {
                newLocale = "fr-FR";
            } else if (newLocale.split('-')[0] === "it" || newLocale.split('_')[0] === "it") {
                newLocale = "it-IT";
            } else if (newLocale.split('-')[0] === "eu" || newLocale.split('_')[0] === "eu") {
                newLocale = "eu";
            } else if (newLocale.split('-')[0] === "en" || newLocale.split('_')[0] === "en") {
                newLocale = "en-US";
            } else {
                newLocale = "en-US";
            }

            var xhr = new XMLHttpRequest(),
                path = translationsPath + '/' + newLocale + ".json";
            xhr.open("GET", path);
            xhr.onload = function () {
                if (xhr.status === 200) {// HTTP OK
                    dictionary = JSON.parse(xhr.response);
                    locale = newLocale;
                }
                cb();
            };
            xhr.send(null);
        }
        function getLocale() {
            return locale;
        }

        this.translate = translate;
        this.getLocale = getLocale;

        function init() {
            setLocale(locale, function () {
                callback(self);
            });
        }
        init();
    };
});
